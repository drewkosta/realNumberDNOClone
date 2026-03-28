package service

import (
	"context"
	"database/sql"
	"fmt"
	"regexp"
	"strings"
	"time"

	"realNumberDNOClone/internal/cache"
	"realNumberDNOClone/internal/metrics"
	"realNumberDNOClone/internal/models"
	"realNumberDNOClone/internal/querylog"
)

type DNOService struct {
	db             *sql.DB
	qlWriter       *querylog.AsyncWriter
	dnoCache       *cache.TTLCache[*models.DNOQueryResponse]
	analyticsCache *cache.TTLCache[*models.AnalyticsSummary]
}

func NewDNOService(
	db *sql.DB,
	qlWriter *querylog.AsyncWriter,
	dnoCache *cache.TTLCache[*models.DNOQueryResponse],
	analyticsCache *cache.TTLCache[*models.AnalyticsSummary],
) *DNOService {
	return &DNOService{
		db:             db,
		qlWriter:       qlWriter,
		dnoCache:       dnoCache,
		analyticsCache: analyticsCache,
	}
}

var phoneRegex = regexp.MustCompile(`^\d{10}$`)

func normalizePhone(phone string) string {
	phone = strings.ReplaceAll(phone, "-", "")
	phone = strings.ReplaceAll(phone, "(", "")
	phone = strings.ReplaceAll(phone, ")", "")
	phone = strings.ReplaceAll(phone, " ", "")
	phone = strings.TrimPrefix(phone, "+1")
	phone = strings.TrimPrefix(phone, "1")
	return phone
}

func dnoCacheKey(phone, channel string) string {
	return phone + ":" + channel
}

func (s *DNOService) QueryNumber(ctx context.Context, phoneNumber, channel string, orgID *int64) (*models.DNOQueryResponse, error) {
	phone := normalizePhone(phoneNumber)
	if !phoneRegex.MatchString(phone) {
		return nil, fmt.Errorf("invalid phone number format: must be 10 digits")
	}
	if channel == "" {
		channel = "voice"
	}
	if err := models.ValidateChannel(channel); err != nil {
		return nil, err
	}

	start := time.Now()
	defer func() {
		metrics.DNOQueryDuration.Observe(time.Since(start).Seconds())
	}()

	// Check cache first
	cacheKey := dnoCacheKey(phone, channel)
	if s.dnoCache != nil {
		if cached, ok := s.dnoCache.Get(cacheKey); ok {
			metrics.CacheHits.WithLabelValues("dno").Inc()
			if orgID != nil {
				result := "miss"
				if cached.IsDNO {
					result = "hit"
				}
				metrics.DNOQueryTotal.WithLabelValues(channel, result).Inc()
				s.qlWriter.Log(querylog.Entry{OrgID: *orgID, PhoneNumber: phone, Result: result, Channel: channel})
			}
			return cached, nil
		}
		metrics.CacheMisses.WithLabelValues("dno").Inc()
	}

	var dno models.DNONumber
	err := s.db.QueryRowContext(ctx,
		`SELECT id, phone_number, dataset, channel, status, updated_at FROM dno_numbers
		 WHERE phone_number = ? AND (channel = ? OR channel = 'both') AND status = 'active'
		 LIMIT 1`,
		phone, channel,
	).Scan(&dno.ID, &dno.PhoneNumber, &dno.Dataset, &dno.Channel, &dno.Status, &dno.UpdatedAt)

	// Log query asynchronously + metrics
	result := "miss"
	if err == nil {
		result = "hit"
	}
	metrics.DNOQueryTotal.WithLabelValues(channel, result).Inc()
	if orgID != nil {
		s.qlWriter.Log(querylog.Entry{OrgID: *orgID, PhoneNumber: phone, Result: result, Channel: channel})
	}

	if err != nil {
		if err == sql.ErrNoRows {
			resp := &models.DNOQueryResponse{
				PhoneNumber: phone,
				IsDNO:       false,
				Channel:     channel,
			}
			if s.dnoCache != nil {
				s.dnoCache.Set(cacheKey, resp)
			}
			return resp, nil
		}
		return nil, err
	}

	resp := &models.DNOQueryResponse{
		PhoneNumber: phone,
		IsDNO:       true,
		Dataset:     dno.Dataset,
		Channel:     dno.Channel,
		Status:      dno.Status,
		LastUpdated: &dno.UpdatedAt,
	}
	if s.dnoCache != nil {
		s.dnoCache.Set(cacheKey, resp)
	}
	return resp, nil
}

func (s *DNOService) BulkQuery(ctx context.Context, phoneNumbers []string, channel string, orgID *int64) (*models.BulkQueryResponse, error) {
	if len(phoneNumbers) > 1000 {
		return nil, fmt.Errorf("bulk query limited to 1000 numbers per request")
	}
	metrics.BulkQuerySize.Observe(float64(len(phoneNumbers)))
	if channel == "" {
		channel = "voice"
	}
	if err := models.ValidateChannel(channel); err != nil {
		return nil, err
	}

	// Normalize and split into cached vs uncached
	type phoneEntry struct {
		original   string
		normalized string
	}
	var toQuery []phoneEntry
	results := make([]models.DNOQueryResponse, 0, len(phoneNumbers))
	hits := 0

	// Check cache for each number
	uncachedNormalized := make([]string, 0)
	uncachedMap := make(map[string]bool)

	for _, p := range phoneNumbers {
		n := normalizePhone(p)
		if !phoneRegex.MatchString(n) {
			results = append(results, models.DNOQueryResponse{
				PhoneNumber: p,
				IsDNO:       false,
				Channel:     channel,
				Status:      "error",
			})
			continue
		}

		cacheKey := dnoCacheKey(n, channel)
		if s.dnoCache != nil {
			if cached, ok := s.dnoCache.Get(cacheKey); ok {
				results = append(results, *cached)
				if cached.IsDNO {
					hits++
				}
				if orgID != nil {
					result := "miss"
					if cached.IsDNO {
						result = "hit"
					}
					s.qlWriter.Log(querylog.Entry{OrgID: *orgID, PhoneNumber: n, Result: result, Channel: channel})
				}
				continue
			}
		}

		toQuery = append(toQuery, phoneEntry{original: p, normalized: n})
		if !uncachedMap[n] {
			uncachedNormalized = append(uncachedNormalized, n)
			uncachedMap[n] = true
		}
	}

	// Batch lookup uncached numbers
	dnoHits := make(map[string]models.DNONumber)
	if len(uncachedNormalized) > 0 {
		placeholders := make([]string, len(uncachedNormalized))
		args := make([]interface{}, 0, len(uncachedNormalized)+1)
		for i, n := range uncachedNormalized {
			placeholders[i] = "?"
			args = append(args, n)
		}
		args = append(args, channel)

		query := fmt.Sprintf(
			`SELECT phone_number, dataset, channel, status, updated_at FROM dno_numbers
			 WHERE phone_number IN (%s) AND (channel = ? OR channel = 'both') AND status = 'active'`,
			strings.Join(placeholders, ","),
		)
		rows, err := s.db.QueryContext(ctx, query, args...)
		if err != nil {
			return nil, fmt.Errorf("bulk query: %w", err)
		}
		defer rows.Close()

		for rows.Next() {
			var n models.DNONumber
			if err := rows.Scan(&n.PhoneNumber, &n.Dataset, &n.Channel, &n.Status, &n.UpdatedAt); err != nil {
				return nil, fmt.Errorf("scanning bulk result: %w", err)
			}
			dnoHits[n.PhoneNumber] = n
		}
		if err := rows.Err(); err != nil {
			return nil, fmt.Errorf("iterating bulk results: %w", err)
		}
	}

	// Build results for uncached numbers
	for _, pe := range toQuery {
		if hit, ok := dnoHits[pe.normalized]; ok {
			resp := models.DNOQueryResponse{
				PhoneNumber: pe.normalized,
				IsDNO:       true,
				Dataset:     hit.Dataset,
				Channel:     hit.Channel,
				Status:      hit.Status,
				LastUpdated: &hit.UpdatedAt,
			}
			results = append(results, resp)
			hits++
			if s.dnoCache != nil {
				s.dnoCache.Set(dnoCacheKey(pe.normalized, channel), &resp)
			}
			if orgID != nil {
				s.qlWriter.Log(querylog.Entry{OrgID: *orgID, PhoneNumber: pe.normalized, Result: "hit", Channel: channel})
			}
		} else {
			resp := models.DNOQueryResponse{
				PhoneNumber: pe.normalized,
				IsDNO:       false,
				Channel:     channel,
			}
			results = append(results, resp)
			if s.dnoCache != nil {
				s.dnoCache.Set(dnoCacheKey(pe.normalized, channel), &resp)
			}
			if orgID != nil {
				s.qlWriter.Log(querylog.Entry{OrgID: *orgID, PhoneNumber: pe.normalized, Result: "miss", Channel: channel})
			}
		}
	}

	return &models.BulkQueryResponse{
		Results: results,
		Total:   len(phoneNumbers),
		Hits:    hits,
		Misses:  len(phoneNumbers) - hits,
	}, nil
}

func (s *DNOService) AddNumber(ctx context.Context, req models.AddDNORequest, orgID, userID int64) (*models.DNONumber, error) {
	phone := normalizePhone(req.PhoneNumber)
	if !phoneRegex.MatchString(phone) {
		return nil, fmt.Errorf("invalid phone number format: must be 10 digits")
	}

	if req.Channel == "" {
		req.Channel = "voice"
	}
	if req.NumberType == "" {
		req.NumberType = "local"
	}
	if err := models.ValidateChannel(req.Channel); err != nil {
		return nil, err
	}
	if err := models.ValidateNumberType(req.NumberType); err != nil {
		return nil, err
	}

	_, err := s.db.ExecContext(ctx,
		`INSERT INTO dno_numbers (phone_number, dataset, number_type, channel, status, reason, added_by_org_id, added_by_user_id)
		 VALUES (?, 'subscriber', ?, ?, 'active', ?, ?, ?)
		 ON CONFLICT(phone_number, channel) DO UPDATE SET status='active', reason=?, updated_at=CURRENT_TIMESTAMP`,
		phone, req.NumberType, req.Channel, req.Reason, orgID, userID, req.Reason,
	)
	if err != nil {
		return nil, fmt.Errorf("adding DNO number: %w", err)
	}

	// Query back the actual ID
	var id int64
	err = s.db.QueryRowContext(ctx,
		`SELECT id FROM dno_numbers WHERE phone_number = ? AND channel = ?`, phone, req.Channel,
	).Scan(&id)
	if err != nil {
		return nil, fmt.Errorf("retrieving DNO number id: %w", err)
	}

	_, _ = s.db.ExecContext(ctx, `INSERT INTO audit_log (user_id, org_id, action, entity_type, entity_id, details)
		VALUES (?, ?, 'add', 'dno_number', ?, ?)`, userID, orgID, id, fmt.Sprintf("Added %s to subscriber DNO list", phone))

	// Invalidate cache for this number
	if s.dnoCache != nil {
		s.dnoCache.DeletePrefix(phone + ":")
	}
	// Invalidate analytics cache
	if s.analyticsCache != nil {
		s.analyticsCache.Delete("all")
		s.analyticsCache.DeletePrefix("org:")
	}

	return &models.DNONumber{
		ID:          id,
		PhoneNumber: phone,
		Dataset:     "subscriber",
		NumberType:  req.NumberType,
		Channel:     req.Channel,
		Status:      "active",
		Reason:      &req.Reason,
	}, nil
}

func (s *DNOService) RemoveNumber(ctx context.Context, phoneNumber, channel string, orgID, userID int64) error {
	phone := normalizePhone(phoneNumber)
	if channel == "" {
		channel = "voice"
	}
	if err := models.ValidateChannel(channel); err != nil {
		return err
	}

	result, err := s.db.ExecContext(ctx,
		`UPDATE dno_numbers SET status='inactive', updated_at=CURRENT_TIMESTAMP
		 WHERE phone_number = ? AND (channel = ? OR channel = 'both') AND dataset = 'subscriber' AND added_by_org_id = ?`,
		phone, channel, orgID,
	)
	if err != nil {
		return err
	}

	rows, err := result.RowsAffected()
	if err != nil {
		return fmt.Errorf("checking rows affected: %w", err)
	}
	if rows == 0 {
		return fmt.Errorf("number not found or not authorized to remove")
	}

	_, _ = s.db.ExecContext(ctx, `INSERT INTO audit_log (user_id, org_id, action, entity_type, details)
		VALUES (?, ?, 'remove', 'dno_number', ?)`, userID, orgID, fmt.Sprintf("Removed %s from subscriber DNO list", phone))

	// Invalidate caches
	if s.dnoCache != nil {
		s.dnoCache.DeletePrefix(phone + ":")
	}
	if s.analyticsCache != nil {
		s.analyticsCache.Delete("all")
		s.analyticsCache.DeletePrefix("org:")
	}

	return nil
}

func (s *DNOService) ListNumbers(ctx context.Context, orgID *int64, dataset, status, channel, search string, page, pageSize int) (*models.PaginatedResponse[models.DNONumber], error) {
	if page < 1 {
		page = 1
	}
	if pageSize < 1 || pageSize > 100 {
		pageSize = 25
	}

	if dataset != "" {
		if err := models.ValidateDataset(dataset); err != nil {
			return nil, err
		}
	}
	if status != "" {
		if err := models.ValidateStatus(status); err != nil {
			return nil, err
		}
	}
	if channel != "" {
		if err := models.ValidateChannel(channel); err != nil {
			return nil, err
		}
	}

	where := "WHERE 1=1"
	args := []interface{}{}

	if orgID != nil {
		where += " AND added_by_org_id = ?"
		args = append(args, *orgID)
	}
	if dataset != "" {
		where += " AND dataset = ?"
		args = append(args, dataset)
	}
	if status != "" {
		where += " AND status = ?"
		args = append(args, status)
	}
	if channel != "" {
		where += " AND (channel = ? OR channel = 'both')"
		args = append(args, channel)
	}
	if search != "" {
		where += " AND phone_number LIKE ?"
		args = append(args, "%"+normalizePhone(search)+"%")
	}

	var total int
	countArgs := make([]interface{}, len(args))
	copy(countArgs, args)
	if err := s.db.QueryRowContext(ctx, "SELECT COUNT(*) FROM dno_numbers "+where, countArgs...).Scan(&total); err != nil {
		return nil, fmt.Errorf("counting numbers: %w", err)
	}

	offset := (page - 1) * pageSize
	args = append(args, pageSize, offset)
	rows, err := s.db.QueryContext(ctx,
		"SELECT id, phone_number, dataset, number_type, channel, status, reason, created_at, updated_at FROM dno_numbers "+
			where+" ORDER BY updated_at DESC LIMIT ? OFFSET ?", args...)
	if err != nil {
		return nil, fmt.Errorf("querying numbers: %w", err)
	}
	defer rows.Close()

	numbers := []models.DNONumber{}
	for rows.Next() {
		var n models.DNONumber
		var reason sql.NullString
		if err := rows.Scan(&n.ID, &n.PhoneNumber, &n.Dataset, &n.NumberType, &n.Channel, &n.Status, &reason, &n.CreatedAt, &n.UpdatedAt); err != nil {
			return nil, fmt.Errorf("scanning number row: %w", err)
		}
		if reason.Valid {
			n.Reason = &reason.String
		}
		numbers = append(numbers, n)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("iterating number rows: %w", err)
	}

	totalPages := total / pageSize
	if total%pageSize > 0 {
		totalPages++
	}

	return &models.PaginatedResponse[models.DNONumber]{
		Data:       numbers,
		Total:      total,
		Page:       page,
		PageSize:   pageSize,
		TotalPages: totalPages,
	}, nil
}

func (s *DNOService) StreamNumbers(ctx context.Context, fn func(models.DNONumber) error) error {
	rows, err := s.db.QueryContext(ctx,
		`SELECT id, phone_number, dataset, number_type, channel, status, reason, created_at, updated_at
		 FROM dno_numbers WHERE status = 'active' ORDER BY phone_number`)
	if err != nil {
		return fmt.Errorf("querying numbers for export: %w", err)
	}
	defer rows.Close()

	for rows.Next() {
		var n models.DNONumber
		var reason sql.NullString
		if err := rows.Scan(&n.ID, &n.PhoneNumber, &n.Dataset, &n.NumberType, &n.Channel, &n.Status, &reason, &n.CreatedAt, &n.UpdatedAt); err != nil {
			return fmt.Errorf("scanning export row: %w", err)
		}
		if reason.Valid {
			n.Reason = &reason.String
		}
		if err := fn(n); err != nil {
			return err
		}
	}
	return rows.Err()
}

func (s *DNOService) GetAnalytics(ctx context.Context, orgID *int64) (*models.AnalyticsSummary, error) {
	// Check analytics cache
	cacheKey := "all"
	if orgID != nil {
		cacheKey = fmt.Sprintf("org:%d", *orgID)
	}
	if s.analyticsCache != nil {
		if cached, ok := s.analyticsCache.Get(cacheKey); ok {
			metrics.CacheHits.WithLabelValues("analytics").Inc()
			return cached, nil
		}
		metrics.CacheMisses.WithLabelValues("analytics").Inc()
	}

	summary := &models.AnalyticsSummary{
		ByDataset:    make(map[string]int),
		ByChannel:    make(map[string]int),
		ByNumberType: make(map[string]int),
	}

	dnoWhere := "WHERE status = 'active'"
	queryWhere := "WHERE queried_at >= ?"
	auditWhere := "WHERE created_at >= ?"
	var dnoArgs []interface{}
	var queryExtraArgs []interface{}
	var auditExtraArgs []interface{}
	if orgID != nil {
		dnoWhere += " AND added_by_org_id = ?"
		dnoArgs = append(dnoArgs, *orgID)
		queryWhere += " AND org_id = ?"
		queryExtraArgs = append(queryExtraArgs, *orgID)
		auditWhere += " AND org_id = ?"
		auditExtraArgs = append(auditExtraArgs, *orgID)
	}

	if err := s.db.QueryRowContext(ctx, "SELECT COUNT(*) FROM dno_numbers "+dnoWhere, dnoArgs...).Scan(&summary.TotalDNONumbers); err != nil {
		return nil, fmt.Errorf("counting active numbers: %w", err)
	}
	summary.ActiveNumbers = summary.TotalDNONumbers

	rows, err := s.db.QueryContext(ctx, "SELECT dataset, COUNT(*) FROM dno_numbers "+dnoWhere+" GROUP BY dataset", dnoArgs...)
	if err != nil {
		return nil, fmt.Errorf("querying by dataset: %w", err)
	}
	for rows.Next() {
		var ds string
		var cnt int
		if err := rows.Scan(&ds, &cnt); err != nil {
			rows.Close()
			return nil, fmt.Errorf("scanning dataset row: %w", err)
		}
		summary.ByDataset[ds] = cnt
	}
	rows.Close()
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("iterating dataset rows: %w", err)
	}

	rows, err = s.db.QueryContext(ctx, "SELECT channel, COUNT(*) FROM dno_numbers "+dnoWhere+" GROUP BY channel", dnoArgs...)
	if err != nil {
		return nil, fmt.Errorf("querying by channel: %w", err)
	}
	for rows.Next() {
		var ch string
		var cnt int
		if err := rows.Scan(&ch, &cnt); err != nil {
			rows.Close()
			return nil, fmt.Errorf("scanning channel row: %w", err)
		}
		summary.ByChannel[ch] = cnt
	}
	rows.Close()
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("iterating channel rows: %w", err)
	}

	rows, err = s.db.QueryContext(ctx, "SELECT number_type, COUNT(*) FROM dno_numbers "+dnoWhere+" GROUP BY number_type", dnoArgs...)
	if err != nil {
		return nil, fmt.Errorf("querying by number type: %w", err)
	}
	for rows.Next() {
		var nt string
		var cnt int
		if err := rows.Scan(&nt, &cnt); err != nil {
			rows.Close()
			return nil, fmt.Errorf("scanning number type row: %w", err)
		}
		summary.ByNumberType[nt] = cnt
	}
	rows.Close()
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("iterating number type rows: %w", err)
	}

	since := time.Now().Add(-24 * time.Hour).Format("2006-01-02 15:04:05")
	queryArgs := append([]interface{}{since}, queryExtraArgs...)
	if err := s.db.QueryRowContext(ctx, "SELECT COUNT(*) FROM query_log "+queryWhere, queryArgs...).Scan(&summary.TotalQueries24h); err != nil {
		return nil, fmt.Errorf("counting queries 24h: %w", err)
	}

	var hits int
	hitArgs := append([]interface{}{since}, queryExtraArgs...)
	if err := s.db.QueryRowContext(ctx, "SELECT COUNT(*) FROM query_log "+queryWhere+" AND result = 'hit'", hitArgs...).Scan(&hits); err != nil {
		return nil, fmt.Errorf("counting hits 24h: %w", err)
	}
	if summary.TotalQueries24h > 0 {
		summary.HitRate24h = float64(hits) / float64(summary.TotalQueries24h) * 100
	}

	hourArgs := append([]interface{}{since}, queryExtraArgs...)
	rows, err = s.db.QueryContext(ctx,
		`SELECT strftime('%Y-%m-%d %H:00', queried_at) as hour, COUNT(*)
		 FROM query_log `+queryWhere+` GROUP BY hour ORDER BY hour`, hourArgs...)
	if err != nil {
		return nil, fmt.Errorf("querying by hour: %w", err)
	}
	for rows.Next() {
		var hc models.HourlyCount
		if err := rows.Scan(&hc.Hour, &hc.Count); err != nil {
			rows.Close()
			return nil, fmt.Errorf("scanning hourly count: %w", err)
		}
		summary.QueriesByHour = append(summary.QueriesByHour, hc)
	}
	rows.Close()
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("iterating hourly rows: %w", err)
	}

	week := time.Now().Add(-7 * 24 * time.Hour).Format("2006-01-02 15:04:05")
	addArgs := append([]interface{}{week}, auditExtraArgs...)
	if err := s.db.QueryRowContext(ctx, "SELECT COUNT(*) FROM audit_log "+auditWhere+" AND action = 'add'", addArgs...).Scan(&summary.RecentAdditions); err != nil {
		return nil, fmt.Errorf("counting recent additions: %w", err)
	}
	removeArgs := append([]interface{}{week}, auditExtraArgs...)
	if err := s.db.QueryRowContext(ctx, "SELECT COUNT(*) FROM audit_log "+auditWhere+" AND action = 'remove'", removeArgs...).Scan(&summary.RecentRemovals); err != nil {
		return nil, fmt.Errorf("counting recent removals: %w", err)
	}

	// Cache the result
	if s.analyticsCache != nil {
		s.analyticsCache.Set(cacheKey, summary)
	}

	return summary, nil
}

func (s *DNOService) GetAuditLog(ctx context.Context, orgID *int64, page, pageSize int) (*models.PaginatedResponse[models.AuditLog], error) {
	if page < 1 {
		page = 1
	}
	if pageSize < 1 || pageSize > 100 {
		pageSize = 25
	}

	where := "WHERE 1=1"
	args := []interface{}{}
	if orgID != nil {
		where += " AND org_id = ?"
		args = append(args, *orgID)
	}

	var total int
	countArgs := make([]interface{}, len(args))
	copy(countArgs, args)
	if err := s.db.QueryRowContext(ctx, "SELECT COUNT(*) FROM audit_log "+where, countArgs...).Scan(&total); err != nil {
		return nil, fmt.Errorf("counting audit entries: %w", err)
	}

	offset := (page - 1) * pageSize
	args = append(args, pageSize, offset)
	rows, err := s.db.QueryContext(ctx,
		"SELECT id, user_id, org_id, action, entity_type, entity_id, details, created_at FROM audit_log "+
			where+" ORDER BY created_at DESC LIMIT ? OFFSET ?", args...)
	if err != nil {
		return nil, fmt.Errorf("querying audit log: %w", err)
	}
	defer rows.Close()

	logs := []models.AuditLog{}
	for rows.Next() {
		var l models.AuditLog
		var userID, orgIDVal, entityID sql.NullInt64
		var details sql.NullString
		if err := rows.Scan(&l.ID, &userID, &orgIDVal, &l.Action, &l.EntityType, &entityID, &details, &l.CreatedAt); err != nil {
			return nil, fmt.Errorf("scanning audit row: %w", err)
		}
		if userID.Valid {
			l.UserID = &userID.Int64
		}
		if orgIDVal.Valid {
			l.OrgID = &orgIDVal.Int64
		}
		if entityID.Valid {
			l.EntityID = &entityID.Int64
		}
		if details.Valid {
			l.Details = &details.String
		}
		logs = append(logs, l)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("iterating audit rows: %w", err)
	}

	totalPages := total / pageSize
	if total%pageSize > 0 {
		totalPages++
	}

	return &models.PaginatedResponse[models.AuditLog]{
		Data:       logs,
		Total:      total,
		Page:       page,
		PageSize:   pageSize,
		TotalPages: totalPages,
	}, nil
}
