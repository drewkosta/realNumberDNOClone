package service

import (
	"context"
	"database/sql"
	"fmt"
	"regexp"
	"strings"
	"time"

	"realNumberDNOClone/internal/cache"
	appdb "realNumberDNOClone/internal/db"
	"realNumberDNOClone/internal/metrics"
	"realNumberDNOClone/internal/models"
	"realNumberDNOClone/internal/querylog"
)

// WebhookFirer is an interface to avoid circular dependency with FeaturesService
type WebhookFirer interface {
	FireWebhooks(event models.WebhookEvent)
}

type DNOService struct {
	db             *appdb.DB
	qlWriter       *querylog.AsyncWriter
	dnoCache       *cache.TTLCache[*models.DNOQueryResponse]
	analyticsCache *cache.TTLCache[*models.AnalyticsSummary]
	webhooks       WebhookFirer
}

func NewDNOService(
	d *appdb.DB,
	qlWriter *querylog.AsyncWriter,
	dnoCache *cache.TTLCache[*models.DNOQueryResponse],
	analyticsCache *cache.TTLCache[*models.AnalyticsSummary],
) *DNOService {
	return &DNOService{
		db:             d,
		qlWriter:       qlWriter,
		dnoCache:       dnoCache,
		analyticsCache: analyticsCache,
	}
}

func (s *DNOService) SetWebhookFirer(wf WebhookFirer) {
	s.webhooks = wf
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
	err := s.db.Reader.QueryRowContext(ctx, s.db.Q(
		`SELECT id, phone_number, dataset, channel, status, updated_at FROM dno_numbers
		 WHERE phone_number = $1 AND (channel = $2 OR channel = 'both') AND status = 'active'
		 LIMIT 1`),
		phone, channel,
	).Scan(&dno.ID, &dno.PhoneNumber, &dno.Dataset, &dno.Channel, &dno.Status, &dno.UpdatedAt)

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
			resp := &models.DNOQueryResponse{PhoneNumber: phone, IsDNO: false, Channel: channel}
			if s.dnoCache != nil {
				s.dnoCache.Set(cacheKey, resp)
			}
			return resp, nil
		}
		return nil, err
	}

	resp := &models.DNOQueryResponse{
		PhoneNumber: phone, IsDNO: true, Dataset: dno.Dataset,
		Channel: dno.Channel, Status: dno.Status, LastUpdated: &dno.UpdatedAt,
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

	type phoneEntry struct {
		original, normalized string
	}
	var toQuery []phoneEntry
	results := make([]models.DNOQueryResponse, 0, len(phoneNumbers))
	hits := 0
	uncachedNormalized := make([]string, 0)
	uncachedMap := make(map[string]bool)

	for _, p := range phoneNumbers {
		n := normalizePhone(p)
		if !phoneRegex.MatchString(n) {
			results = append(results, models.DNOQueryResponse{PhoneNumber: p, IsDNO: false, Channel: channel, Status: "error"})
			continue
		}
		if s.dnoCache != nil {
			if cached, ok := s.dnoCache.Get(dnoCacheKey(n, channel)); ok {
				results = append(results, *cached)
				if cached.IsDNO {
					hits++
				}
				if orgID != nil {
					r := "miss"
					if cached.IsDNO {
						r = "hit"
					}
					s.qlWriter.Log(querylog.Entry{OrgID: *orgID, PhoneNumber: n, Result: r, Channel: channel})
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

	dnoHits := make(map[string]models.DNONumber)
	if len(uncachedNormalized) > 0 {
		// Build parameterized IN clause: $1, $2, ..., $N, $N+1 (for channel)
		placeholders := make([]string, len(uncachedNormalized))
		args := make([]interface{}, 0, len(uncachedNormalized)+1)
		for i, n := range uncachedNormalized {
			placeholders[i] = fmt.Sprintf("$%d", i+1)
			args = append(args, n)
		}
		chanIdx := len(uncachedNormalized) + 1
		args = append(args, channel)

		query := fmt.Sprintf(
			`SELECT phone_number, dataset, channel, status, updated_at FROM dno_numbers
			 WHERE phone_number IN (%s) AND (channel = $%d OR channel = 'both') AND status = 'active'`,
			strings.Join(placeholders, ","), chanIdx,
		)
		rows, err := s.db.Reader.QueryContext(ctx, s.db.Q(query), args...)
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

	for _, pe := range toQuery {
		if hit, ok := dnoHits[pe.normalized]; ok {
			resp := models.DNOQueryResponse{
				PhoneNumber: pe.normalized, IsDNO: true, Dataset: hit.Dataset,
				Channel: hit.Channel, Status: hit.Status, LastUpdated: &hit.UpdatedAt,
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
			resp := models.DNOQueryResponse{PhoneNumber: pe.normalized, IsDNO: false, Channel: channel}
			results = append(results, resp)
			if s.dnoCache != nil {
				s.dnoCache.Set(dnoCacheKey(pe.normalized, channel), &resp)
			}
			if orgID != nil {
				s.qlWriter.Log(querylog.Entry{OrgID: *orgID, PhoneNumber: pe.normalized, Result: "miss", Channel: channel})
			}
		}
	}

	return &models.BulkQueryResponse{Results: results, Total: len(phoneNumbers), Hits: hits, Misses: len(phoneNumbers) - hits}, nil
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

	_, err := s.db.Writer.ExecContext(ctx, s.db.Q(
		`INSERT INTO dno_numbers (phone_number, dataset, number_type, channel, status, reason, added_by_org_id, added_by_user_id)
		 VALUES ($1, 'subscriber', $2, $3, 'active', $4, $5, $6)
		 ON CONFLICT(phone_number, channel) DO UPDATE SET status='active', reason=$7, updated_at=`+s.db.QNow()),
		phone, req.NumberType, req.Channel, req.Reason, orgID, userID, req.Reason,
	)
	if err != nil {
		return nil, fmt.Errorf("adding DNO number: %w", err)
	}

	var id int64
	err = s.db.Reader.QueryRowContext(ctx, s.db.Q(
		`SELECT id FROM dno_numbers WHERE phone_number = $1 AND channel = $2`), phone, req.Channel,
	).Scan(&id)
	if err != nil {
		return nil, fmt.Errorf("retrieving DNO number id: %w", err)
	}

	_, _ = s.db.Writer.ExecContext(ctx, s.db.Q(
		`INSERT INTO audit_log (user_id, org_id, action, entity_type, entity_id, details)
		 VALUES ($1, $2, 'add', 'dno_number', $3, $4)`),
		userID, orgID, id, fmt.Sprintf("Added %s to subscriber DNO list", phone))

	if s.dnoCache != nil {
		s.dnoCache.DeletePrefix(phone + ":")
	}
	if s.analyticsCache != nil {
		s.analyticsCache.Delete("all")
		s.analyticsCache.DeletePrefix("org:")
	}

	// Fire webhooks
	if s.webhooks != nil {
		s.webhooks.FireWebhooks(models.WebhookEvent{
			Event: "dno.added", PhoneNumber: phone, Dataset: "subscriber",
			Channel: req.Channel, Timestamp: time.Now(), OrgID: &orgID,
		})
	}

	return &models.DNONumber{
		ID: id, PhoneNumber: phone, Dataset: "subscriber",
		NumberType: req.NumberType, Channel: req.Channel, Status: "active", Reason: &req.Reason,
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

	result, err := s.db.Writer.ExecContext(ctx, s.db.Q(
		`UPDATE dno_numbers SET status='inactive', updated_at=`+s.db.QNow()+`
		 WHERE phone_number = $1 AND (channel = $2 OR channel = 'both') AND dataset = 'subscriber' AND added_by_org_id = $3`),
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

	_, _ = s.db.Writer.ExecContext(ctx, s.db.Q(
		`INSERT INTO audit_log (user_id, org_id, action, entity_type, details)
		 VALUES ($1, $2, 'remove', 'dno_number', $3)`),
		userID, orgID, fmt.Sprintf("Removed %s from subscriber DNO list", phone))

	if s.dnoCache != nil {
		s.dnoCache.DeletePrefix(phone + ":")
	}
	if s.analyticsCache != nil {
		s.analyticsCache.Delete("all")
		s.analyticsCache.DeletePrefix("org:")
	}

	if s.webhooks != nil {
		s.webhooks.FireWebhooks(models.WebhookEvent{
			Event: "dno.removed", PhoneNumber: phone, Dataset: "subscriber",
			Channel: channel, Timestamp: time.Now(), OrgID: &orgID,
		})
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
	paramIdx := 0
	nextParam := func() string {
		paramIdx++
		return fmt.Sprintf("$%d", paramIdx)
	}

	if orgID != nil {
		where += " AND added_by_org_id = " + nextParam()
		args = append(args, *orgID)
	}
	if dataset != "" {
		where += " AND dataset = " + nextParam()
		args = append(args, dataset)
	}
	if status != "" {
		where += " AND status = " + nextParam()
		args = append(args, status)
	}
	if channel != "" {
		where += " AND (channel = " + nextParam() + " OR channel = 'both')"
		args = append(args, channel)
	}
	if search != "" {
		where += " AND phone_number LIKE " + nextParam()
		args = append(args, "%"+normalizePhone(search)+"%")
	}

	var total int
	countArgs := make([]interface{}, len(args))
	copy(countArgs, args)
	if err := s.db.Reader.QueryRowContext(ctx, s.db.Q("SELECT COUNT(*) FROM dno_numbers "+where), countArgs...).Scan(&total); err != nil {
		return nil, fmt.Errorf("counting numbers: %w", err)
	}

	offset := (page - 1) * pageSize
	limitParam := nextParam()
	offsetParam := nextParam()
	args = append(args, pageSize, offset)
	rows, err := s.db.Reader.QueryContext(ctx, s.db.Q(
		"SELECT id, phone_number, dataset, number_type, channel, status, reason, created_at, updated_at FROM dno_numbers "+
			where+" ORDER BY updated_at DESC LIMIT "+limitParam+" OFFSET "+offsetParam), args...)
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
	return &models.PaginatedResponse[models.DNONumber]{Data: numbers, Total: total, Page: page, PageSize: pageSize, TotalPages: totalPages}, nil
}

func (s *DNOService) StreamNumbers(ctx context.Context, fn func(models.DNONumber) error) error {
	rows, err := s.db.Reader.QueryContext(ctx,
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
		ByDataset: make(map[string]int), ByChannel: make(map[string]int), ByNumberType: make(map[string]int),
	}

	// DNO number counts are always system-wide (the DNO list is a shared resource).
	// Only query logs and audit logs are org-scoped for non-admins.
	dnoWhere := "WHERE status = 'active'"
	var dnoArgs []interface{}

	queryWhere := "WHERE queried_at >= $1"
	auditWhere := "WHERE created_at >= $1"
	var queryExtraArgs []interface{}
	var auditExtraArgs []interface{}
	if orgID != nil {
		queryWhere += " AND org_id = $2"
		queryExtraArgs = append(queryExtraArgs, *orgID)
		auditWhere += " AND org_id = $2"
		auditExtraArgs = append(auditExtraArgs, *orgID)
	}

	ar := s.db.AnalyticsReader()
	if err := ar.QueryRowContext(ctx, s.db.Q("SELECT COUNT(*) FROM dno_numbers "+dnoWhere), dnoArgs...).Scan(&summary.TotalDNONumbers); err != nil {
		return nil, fmt.Errorf("counting active numbers: %w", err)
	}
	summary.ActiveNumbers = summary.TotalDNONumbers

	for _, q := range []struct {
		query string
		args  []interface{}
		dest  map[string]int
	}{
		{"SELECT dataset, COUNT(*) FROM dno_numbers " + dnoWhere + " GROUP BY dataset", dnoArgs, summary.ByDataset},
		{"SELECT channel, COUNT(*) FROM dno_numbers " + dnoWhere + " GROUP BY channel", dnoArgs, summary.ByChannel},
		{"SELECT number_type, COUNT(*) FROM dno_numbers " + dnoWhere + " GROUP BY number_type", dnoArgs, summary.ByNumberType},
	} {
		rows, err := ar.QueryContext(ctx, s.db.Q(q.query), q.args...)
		if err != nil {
			return nil, fmt.Errorf("analytics group query: %w", err)
		}
		for rows.Next() {
			var k string
			var v int
			if err := rows.Scan(&k, &v); err != nil {
				rows.Close()
				return nil, fmt.Errorf("scanning analytics row: %w", err)
			}
			q.dest[k] = v
		}
		rows.Close()
		if err := rows.Err(); err != nil {
			return nil, fmt.Errorf("iterating analytics rows: %w", err)
		}
	}

	since := time.Now().Add(-24 * time.Hour).Format("2006-01-02 15:04:05")
	queryArgs := append([]interface{}{since}, queryExtraArgs...)

	if err := ar.QueryRowContext(ctx, s.db.Q("SELECT COUNT(*) FROM query_log "+queryWhere), queryArgs...).Scan(&summary.TotalQueries24h); err != nil {
		return nil, fmt.Errorf("counting queries 24h: %w", err)
	}

	var hits int
	hitWhere := queryWhere + " AND result = 'hit'"
	if err := ar.QueryRowContext(ctx, s.db.Q("SELECT COUNT(*) FROM query_log "+hitWhere), queryArgs...).Scan(&hits); err != nil {
		return nil, fmt.Errorf("counting hits 24h: %w", err)
	}
	if summary.TotalQueries24h > 0 {
		summary.HitRate24h = float64(hits) / float64(summary.TotalQueries24h) * 100
	}

	hourCol := s.db.QTimeTrunc("queried_at")
	rows, err := ar.QueryContext(ctx, s.db.Q(
		`SELECT `+hourCol+` as hour, COUNT(*) FROM query_log `+queryWhere+` GROUP BY hour ORDER BY hour`), queryArgs...)
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
	if err := ar.QueryRowContext(ctx, s.db.Q("SELECT COUNT(*) FROM audit_log "+auditWhere+" AND action = 'add'"), addArgs...).Scan(&summary.RecentAdditions); err != nil {
		return nil, fmt.Errorf("counting recent additions: %w", err)
	}
	if err := ar.QueryRowContext(ctx, s.db.Q("SELECT COUNT(*) FROM audit_log "+auditWhere+" AND action = 'remove'"), addArgs...).Scan(&summary.RecentRemovals); err != nil {
		return nil, fmt.Errorf("counting recent removals: %w", err)
	}

	if s.analyticsCache != nil {
		s.analyticsCache.Set(cacheKey, summary)
	}
	return summary, nil
}

func (s *DNOService) GetAuditLog(ctx context.Context, orgID *int64, page, pageSize int) (*models.PaginatedResponse[models.AuditLog], error) {
	ar := s.db.AnalyticsReader()
	if page < 1 {
		page = 1
	}
	if pageSize < 1 || pageSize > 100 {
		pageSize = 25
	}

	where := "WHERE 1=1"
	args := []interface{}{}
	paramIdx := 0
	nextParam := func() string {
		paramIdx++
		return fmt.Sprintf("$%d", paramIdx)
	}

	if orgID != nil {
		where += " AND org_id = " + nextParam()
		args = append(args, *orgID)
	}

	var total int
	countArgs := make([]interface{}, len(args))
	copy(countArgs, args)
	if err := ar.QueryRowContext(ctx, s.db.Q("SELECT COUNT(*) FROM audit_log "+where), countArgs...).Scan(&total); err != nil {
		return nil, fmt.Errorf("counting audit entries: %w", err)
	}

	offset := (page - 1) * pageSize
	limitParam := nextParam()
	offsetParam := nextParam()
	args = append(args, pageSize, offset)
	rows, err := ar.QueryContext(ctx, s.db.Q(
		"SELECT id, user_id, org_id, action, entity_type, entity_id, details, created_at FROM audit_log "+
			where+" ORDER BY created_at DESC LIMIT "+limitParam+" OFFSET "+offsetParam), args...)
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
	return &models.PaginatedResponse[models.AuditLog]{Data: logs, Total: total, Page: page, PageSize: pageSize, TotalPages: totalPages}, nil
}
