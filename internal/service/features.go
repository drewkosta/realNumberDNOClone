package service

import (
	"context"
	"crypto/hmac"
	"crypto/sha256"
	"database/sql"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"log/slog"
	"net/http"
	"strings"
	"time"

	appdb "realNumberDNOClone/internal/db"
	"realNumberDNOClone/internal/models"
)

type FeaturesService struct {
	db     *appdb.DB
	logger *slog.Logger
}

func NewFeaturesService(d *appdb.DB, logger *slog.Logger) *FeaturesService {
	return &FeaturesService{db: d, logger: logger}
}

// ── ITG Traceback Ingest ────────────────────────────────────────────────────

func (s *FeaturesService) IngestITGNumber(ctx context.Context, req models.ITGIngestRequest) (*models.DNONumber, error) {
	phone := normalizePhone(req.PhoneNumber)
	if !phoneRegex.MatchString(phone) {
		return nil, fmt.Errorf("invalid phone number format: must be 10 digits")
	}
	if req.Channel == "" {
		req.Channel = "voice"
	}
	if err := models.ValidateChannel(req.Channel); err != nil {
		return nil, err
	}

	reason := fmt.Sprintf("Traceback: %s", req.ThreatCategory)
	_, err := s.db.Writer.ExecContext(ctx, s.db.Q(
		`INSERT INTO dno_numbers (phone_number, dataset, number_type, channel, status, reason, investigation_id, threat_category)
		 VALUES ($1, 'itg', 'local', $2, 'active', $3, $4, $5)
		 ON CONFLICT(phone_number, channel) DO UPDATE SET
		   status='active', reason=$6, investigation_id=$7, threat_category=$8, dataset='itg', updated_at=`+s.db.QNow()),
		phone, req.Channel, reason, req.InvestigationID, req.ThreatCategory,
		reason, req.InvestigationID, req.ThreatCategory,
	)
	if err != nil {
		return nil, fmt.Errorf("ingesting ITG number: %w", err)
	}

	var id int64
	s.db.Reader.QueryRowContext(ctx, s.db.Q(`SELECT id FROM dno_numbers WHERE phone_number = $1 AND channel = $2`), phone, req.Channel).Scan(&id)

	return &models.DNONumber{
		ID:              id,
		PhoneNumber:     phone,
		Dataset:         "itg",
		NumberType:      "local",
		Channel:         req.Channel,
		Status:          "active",
		InvestigationID: &req.InvestigationID,
		ThreatCategory:  &req.ThreatCategory,
	}, nil
}

// ── Number Ownership Validation (Mock Registry) ─────────────────────────────

func (s *FeaturesService) ValidateOwnership(ctx context.Context, phoneNumber string, orgID int64) (bool, string, error) {
	phone := normalizePhone(phoneNumber)
	var entry models.NumberRegistryEntry
	var ownerOrgID sql.NullInt64
	err := s.db.Reader.QueryRowContext(ctx, s.db.Q(
		`SELECT phone_number, owner_org_id, number_type, status, text_enabled FROM number_registry WHERE phone_number = $1`),
		phone,
	).Scan(&entry.PhoneNumber, &ownerOrgID, &entry.NumberType, &entry.Status, &entry.TextEnabled)

	if err == sql.ErrNoRows {
		// Number not in registry -- allow (unregistered numbers can be added)
		return true, "number not in registry, ownership not verified", nil
	}
	if err != nil {
		return false, "", fmt.Errorf("checking registry: %w", err)
	}

	if entry.Status == "disconnected" || entry.Status == "unassigned" {
		return false, fmt.Sprintf("number is %s and cannot be added to subscriber set", entry.Status), nil
	}

	if ownerOrgID.Valid && ownerOrgID.Int64 != orgID {
		return false, "number is owned by a different organization", nil
	}

	return true, "ownership verified", nil
}

// ── Webhooks ────────────────────────────────────────────────────────────────

func (s *FeaturesService) CreateWebhook(ctx context.Context, orgID int64, url, secret, events string) (*models.WebhookSubscription, error) {
	if events == "" {
		events = "dno.added,dno.removed"
	}
	result, err := s.db.Writer.ExecContext(ctx, s.db.Q(
		`INSERT INTO webhook_subscriptions (org_id, url, secret, events) VALUES ($1, $2, $3, $4)`),
		orgID, url, secret, events,
	)
	if err != nil {
		return nil, fmt.Errorf("creating webhook: %w", err)
	}
	id, _ := result.LastInsertId()
	return &models.WebhookSubscription{
		ID: id, OrgID: orgID, URL: url, Events: events, Active: true,
	}, nil
}

func (s *FeaturesService) ListWebhooks(ctx context.Context, orgID int64) ([]models.WebhookSubscription, error) {
	rows, err := s.db.Reader.QueryContext(ctx, s.db.Q(
		`SELECT id, org_id, url, events, active, created_at FROM webhook_subscriptions WHERE org_id = $1`), orgID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var subs []models.WebhookSubscription
	for rows.Next() {
		var sub models.WebhookSubscription
		if err := rows.Scan(&sub.ID, &sub.OrgID, &sub.URL, &sub.Events, &sub.Active, &sub.CreatedAt); err != nil {
			return nil, err
		}
		subs = append(subs, sub)
	}
	return subs, rows.Err()
}

func (s *FeaturesService) DeleteWebhook(ctx context.Context, id, orgID int64) error {
	result, err := s.db.Writer.ExecContext(ctx, s.db.Q(
		`DELETE FROM webhook_subscriptions WHERE id = $1 AND org_id = $2`), id, orgID)
	if err != nil {
		return err
	}
	rows, _ := result.RowsAffected()
	if rows == 0 {
		return fmt.Errorf("webhook not found")
	}
	return nil
}

func (s *FeaturesService) FireWebhooks(event models.WebhookEvent) {
	go func() {
		ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
		defer cancel()

		rows, err := s.db.Reader.QueryContext(ctx,
			`SELECT url, secret, events FROM webhook_subscriptions WHERE active = 1`)
		if err != nil {
			s.logger.Error("webhook: query subscriptions", "error", err)
			return
		}
		defer rows.Close()

		payload, _ := json.Marshal(event)

		for rows.Next() {
			var url, secret, events string
			if err := rows.Scan(&url, &secret, &events); err != nil {
				continue
			}
			if !strings.Contains(events, event.Event) {
				continue
			}
			go s.deliverWebhook(url, secret, payload)
		}
	}()
}

func (s *FeaturesService) deliverWebhook(url, secret string, payload []byte) {
	mac := hmac.New(sha256.New, []byte(secret))
	mac.Write(payload)
	sig := hex.EncodeToString(mac.Sum(nil))

	req, err := http.NewRequest("POST", url, strings.NewReader(string(payload)))
	if err != nil {
		s.logger.Error("webhook: create request", "error", err, "url", url)
		return
	}
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("X-Webhook-Signature", sig)

	client := &http.Client{Timeout: 5 * time.Second}
	resp, err := client.Do(req)
	if err != nil {
		s.logger.Warn("webhook: delivery failed", "error", err, "url", url)
		return
	}
	resp.Body.Close()
	s.logger.Debug("webhook: delivered", "url", url, "status", resp.StatusCode)
}

// ── DNO Analyzer ────────────────────────────────────────────────────────────

func (s *FeaturesService) AnalyzeTraffic(ctx context.Context, req models.DNOAnalyzerRequest) (*models.DNOAnalyzerReport, error) {
	if req.Channel == "" {
		req.Channel = "voice"
	}

	report := &models.DNOAnalyzerReport{
		TotalRecords: len(req.Records),
		ByDataset:    make(map[string]int),
		ByThreat:     make(map[string]int),
	}

	// Collect unique numbers
	numberCounts := make(map[string]int)
	for _, r := range req.Records {
		n := normalizePhone(r.CallerID)
		if phoneRegex.MatchString(n) {
			numberCounts[n]++
		}
	}

	// Batch lookup
	unique := make([]string, 0, len(numberCounts))
	for n := range numberCounts {
		unique = append(unique, n)
	}

	type hitInfo struct {
		dataset        string
		threatCategory sql.NullString
	}
	hits := make(map[string]hitInfo)

	// Query in batches of 500
	for i := 0; i < len(unique); i += 500 {
		end := i + 500
		if end > len(unique) {
			end = len(unique)
		}
		batch := unique[i:end]

		placeholders := make([]string, len(batch))
		args := make([]interface{}, 0, len(batch)+1)
		for j, n := range batch {
			placeholders[j] = fmt.Sprintf("$%d", j+1)
			args = append(args, n)
		}
		args = append(args, req.Channel)
		chanIdx := len(batch) + 1

		query := fmt.Sprintf(
			`SELECT phone_number, dataset, threat_category FROM dno_numbers
			 WHERE phone_number IN (%s) AND (channel = $%d OR channel = 'both') AND status = 'active'`,
			strings.Join(placeholders, ","), chanIdx,
		)
		rows, err := s.db.Reader.QueryContext(ctx, s.db.Q(query), args...)
		if err != nil {
			return nil, fmt.Errorf("analyzer query: %w", err)
		}
		for rows.Next() {
			var phone string
			var info hitInfo
			if err := rows.Scan(&phone, &info.dataset, &info.threatCategory); err != nil {
				rows.Close()
				return nil, err
			}
			hits[phone] = info
		}
		rows.Close()
	}

	// Build report
	spoofedMap := make(map[string]*models.SpoofedNumber)
	for _, r := range req.Records {
		n := normalizePhone(r.CallerID)
		if info, ok := hits[n]; ok {
			report.DNOHits++
			report.ByDataset[info.dataset]++
			if info.threatCategory.Valid && info.threatCategory.String != "" {
				report.ByThreat[info.threatCategory.String]++
			}
			if _, exists := spoofedMap[n]; !exists {
				tc := ""
				if info.threatCategory.Valid {
					tc = info.threatCategory.String
				}
				spoofedMap[n] = &models.SpoofedNumber{
					PhoneNumber: n, Dataset: info.dataset, ThreatCategory: tc,
				}
			}
			spoofedMap[n].Count += numberCounts[n]
		} else {
			report.DNOMisses++
		}
	}

	if report.TotalRecords > 0 {
		report.HitRate = float64(report.DNOHits) / float64(report.TotalRecords) * 100
	}

	// Top spoofed numbers (up to 20)
	for _, sn := range spoofedMap {
		report.TopSpoofed = append(report.TopSpoofed, *sn)
	}
	if len(report.TopSpoofed) > 20 {
		report.TopSpoofed = report.TopSpoofed[:20]
	}

	// Estimate daily blocked (extrapolate from sample)
	report.EstBlockedPerDay = report.DNOHits

	return report, nil
}

// ── Compliance Report ───────────────────────────────────────────────────────

func (s *FeaturesService) GenerateComplianceReport(ctx context.Context, orgID *int64) (*models.ComplianceReport, error) {
	report := &models.ComplianceReport{
		GeneratedAt:       time.Now(),
		DatasetCoverage:   make(map[string]int),
		ChannelCoverage:   make(map[string]int),
		UpdateFrequency:   "Near real-time (database updates propagate within seconds)",
		EnforcementMethod: "Real-time API query at call origination; batch flat file for offline processing",
	}

	// Org name
	if orgID != nil {
		s.db.Reader.QueryRowContext(ctx, s.db.Q(`SELECT name FROM organizations WHERE id = $1`), *orgID).Scan(&report.OrgName)
	}

	// Total numbers
	dnoWhere := "WHERE status = 'active'"
	var dnoArgs []interface{}
	if orgID != nil {
		dnoWhere += " AND added_by_org_id = $1"
		dnoArgs = append(dnoArgs, *orgID)
	}
	s.db.Reader.QueryRowContext(ctx, s.db.Q("SELECT COUNT(*) FROM dno_numbers "+dnoWhere), dnoArgs...).Scan(&report.TotalDNONumbers)

	// Dataset coverage
	rows, _ := s.db.Reader.QueryContext(ctx, s.db.Q("SELECT dataset, COUNT(*) FROM dno_numbers "+dnoWhere+" GROUP BY dataset"), dnoArgs...)
	if rows != nil {
		for rows.Next() {
			var ds string
			var cnt int
			rows.Scan(&ds, &cnt)
			report.DatasetCoverage[ds] = cnt
		}
		rows.Close()
	}

	// Channel coverage
	rows, _ = s.db.Reader.QueryContext(ctx, s.db.Q("SELECT channel, COUNT(*) FROM dno_numbers "+dnoWhere+" GROUP BY channel"), dnoArgs...)
	if rows != nil {
		for rows.Next() {
			var ch string
			var cnt int
			rows.Scan(&ch, &cnt)
			report.ChannelCoverage[ch] = cnt
		}
		rows.Close()
	}

	// 30-day query stats
	since := time.Now().Add(-30 * 24 * time.Hour).Format("2006-01-02 15:04:05")
	queryWhere := "WHERE queried_at >= $1"
	queryArgs := []interface{}{since}
	if orgID != nil {
		queryWhere += " AND org_id = $2"
		queryArgs = append(queryArgs, *orgID)
	}
	s.db.Reader.QueryRowContext(ctx, s.db.Q("SELECT COUNT(*) FROM query_log "+queryWhere), queryArgs...).Scan(&report.Last30DaysQueries)

	var hitCount int
	s.db.Reader.QueryRowContext(ctx, s.db.Q("SELECT COUNT(*) FROM query_log "+queryWhere+" AND result = 'hit'"), queryArgs...).Scan(&hitCount)
	if report.Last30DaysQueries > 0 {
		report.Last30DaysHitRate = float64(hitCount) / float64(report.Last30DaysQueries) * 100
	}
	report.Last30DaysBlocked = hitCount

	// Compliance assessment
	report.ComplianceStatus = "compliant"
	if _, ok := report.DatasetCoverage["auto"]; !ok {
		report.ComplianceStatus = "at_risk"
		report.Recommendations = append(report.Recommendations, "Enable Auto Set dataset for unassigned/disconnected number coverage")
	}
	if _, ok := report.DatasetCoverage["itg"]; !ok {
		report.Recommendations = append(report.Recommendations, "Enable ITG dataset for traceback-identified spoofed numbers")
	}
	if report.TotalDNONumbers == 0 {
		report.ComplianceStatus = "non_compliant"
		report.Recommendations = append(report.Recommendations, "No DNO numbers active -- FCC requires reasonable DNO list enforcement")
	}
	if report.Last30DaysQueries == 0 {
		report.ComplianceStatus = "at_risk"
		report.Recommendations = append(report.Recommendations, "No queries in last 30 days -- ensure DNO checking is integrated into call routing")
	}
	if len(report.Recommendations) == 0 {
		report.Recommendations = append(report.Recommendations, "All checks passed. Maintain current enforcement posture.")
	}

	return report, nil
}

// ── ROI Calculator ──────────────────────────────────────────────────────────

func (s *FeaturesService) CalculateROI(_ context.Context, dailyCallVolume int) *models.ROICalculation {
	// Industry average: 17% of checked calls match DNO
	hitRate := 17.0
	dailyBlocked := int(float64(dailyCallVolume) * hitRate / 100)
	// Industry estimate: $3-5 per robocall complaint handled
	complaintCost := 4.0

	riskLevel := "low"
	if dailyCallVolume > 100000 {
		riskLevel = "high"
	} else if dailyCallVolume > 10000 {
		riskLevel = "medium"
	}

	return &models.ROICalculation{
		DailyCallVolume:     dailyCallVolume,
		EstHitRate:          hitRate,
		EstDailyBlocked:     dailyBlocked,
		EstMonthlyBlocked:   dailyBlocked * 30,
		EstAnnualBlocked:    dailyBlocked * 365,
		AvgComplaintCost:    complaintCost,
		EstAnnualSavings:    float64(dailyBlocked*365) * complaintCost,
		ComplianceRiskLevel: riskLevel,
	}
}

// ── Mock TSS Registry Sync ────────────────────────────────────���─────────────

func (s *FeaturesService) SyncTSSRegistry(ctx context.Context) (int, error) {
	// Find toll-free numbers in number_registry that are NOT text-enabled
	// and add them to DNO as tss_registry dataset
	result, err := s.db.Writer.ExecContext(ctx, s.db.Q(
		`INSERT INTO dno_numbers (phone_number, dataset, number_type, channel, status, reason)
		 SELECT nr.phone_number, 'tss_registry', 'toll_free', 'text', 'active', 'Non-text-enabled toll-free number (TSS Registry sync)'
		 FROM number_registry nr
		 WHERE nr.number_type = 'toll_free' AND nr.text_enabled = 0
		   AND nr.phone_number NOT IN (SELECT phone_number FROM dno_numbers WHERE dataset = 'tss_registry' AND channel = 'text')`,
	))
	if err != nil {
		return 0, fmt.Errorf("TSS registry sync: %w", err)
	}
	rows, _ := result.RowsAffected()
	return int(rows), nil
}

// ── Mock NPAC Porting Event ─────────────────────────────────────────────────

func (s *FeaturesService) ProcessNPACPortingEvent(ctx context.Context, phoneNumber, newStatus string, newOwnerOrgID *int64) error {
	phone := normalizePhone(phoneNumber)
	if !phoneRegex.MatchString(phone) {
		return fmt.Errorf("invalid phone number format")
	}

	// Update registry
	if newStatus == "ported" || newStatus == "assigned" {
		_, err := s.db.Writer.ExecContext(ctx, s.db.Q(
			`INSERT INTO number_registry (phone_number, owner_org_id, number_type, status, updated_at)
			 VALUES ($1, $2, 'local', $3, `+s.db.QNow()+`)
			 ON CONFLICT(phone_number) DO UPDATE SET owner_org_id=$4, status=$5, updated_at=`+s.db.QNow()),
			phone, newOwnerOrgID, newStatus, newOwnerOrgID, newStatus,
		)
		if err != nil {
			return fmt.Errorf("updating registry: %w", err)
		}
	}

	// If disconnected/unassigned, auto-add to DNO auto set
	if newStatus == "disconnected" || newStatus == "unassigned" {
		reason := fmt.Sprintf("NPAC: number %s", newStatus)
		_, err := s.db.Writer.ExecContext(ctx, s.db.Q(
			`INSERT INTO dno_numbers (phone_number, dataset, number_type, channel, status, reason)
			 VALUES ($1, 'auto', 'local', 'voice', 'active', $2)
			 ON CONFLICT(phone_number, channel) DO UPDATE SET status='active', reason=$3, dataset='auto', updated_at=`+s.db.QNow()),
			phone, reason, reason,
		)
		if err != nil {
			return fmt.Errorf("auto-adding to DNO: %w", err)
		}

		s.logger.Info("NPAC: auto-added to DNO", "phone", phone, "status", newStatus)
	}

	// If ported/assigned, remove from auto set (number is now active)
	if newStatus == "ported" || newStatus == "assigned" {
		s.db.Writer.ExecContext(ctx, s.db.Q(
			`UPDATE dno_numbers SET status='inactive', updated_at=`+s.db.QNow()+`
			 WHERE phone_number = $1 AND dataset = 'auto' AND status = 'active'`), phone)
		s.logger.Info("NPAC: removed from auto DNO set", "phone", phone, "status", newStatus)
	}

	return nil
}
