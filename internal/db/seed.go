package db

import (
	"database/sql"
	"fmt"
	"log"
	"math/rand"
	"time"

	"golang.org/x/crypto/bcrypt"
)

func SeedLocalData(d *DB) error {
	db := d.Writer
	var count int
	if err := db.QueryRow("SELECT COUNT(*) FROM dno_numbers").Scan(&count); err != nil {
		return fmt.Errorf("checking existing data: %w", err)
	}
	if count > 0 {
		log.Println("[seed] Database already has data, skipping seed")
		return nil
	}

	log.Println("[seed] Seeding local database with mock data...")

	if err := seedOrganizations(db); err != nil {
		return fmt.Errorf("seeding organizations: %w", err)
	}
	if err := seedUsers(db); err != nil {
		return fmt.Errorf("seeding users: %w", err)
	}
	if err := seedDNONumbers(db); err != nil {
		return fmt.Errorf("seeding DNO numbers: %w", err)
	}
	if err := seedQueryLogs(db); err != nil {
		return fmt.Errorf("seeding query logs: %w", err)
	}
	if err := seedAuditLogs(db); err != nil {
		return fmt.Errorf("seeding audit logs: %w", err)
	}

	log.Println("[seed] Seeding complete")
	return nil
}

func seedOrganizations(db *sql.DB) error {
	orgs := []struct {
		name, orgType, spid, respOrgID string
	}{
		{"Acme Telecom", "carrier", "5001", ""},
		{"National Voice Corp", "carrier", "5002", ""},
		{"SecureGate Systems", "gateway_provider", "5003", ""},
		{"Pacific Bell Services", "resp_org", "", "RPC01"},
		{"Atlantic TF Management", "resp_org", "", "ATL02"},
		{"Midwest Carrier Group", "carrier", "5004", ""},
		{"Coastal Gateway Inc", "gateway_provider", "5005", ""},
		{"Liberty Toll-Free Services", "resp_org", "", "LTF03"},
	}

	for _, o := range orgs {
		var spid, respOrgID interface{}
		if o.spid != "" {
			spid = o.spid
		}
		if o.respOrgID != "" {
			respOrgID = o.respOrgID
		}
		_, err := db.Exec(
			`INSERT OR IGNORE INTO organizations (name, org_type, spid, resp_org_id) VALUES (?, ?, ?, ?)`,
			o.name, o.orgType, spid, respOrgID,
		)
		if err != nil {
			return err
		}
	}
	log.Println("[seed]   8 organizations created")
	return nil
}

func seedUsers(db *sql.DB) error {
	hash, err := bcrypt.GenerateFromPassword([]byte("password123"), bcrypt.DefaultCost)
	if err != nil {
		return err
	}

	users := []struct {
		email, first, last, role string
		orgID                    int64
	}{
		{"jsmith@acmetelecom.com", "John", "Smith", "org_admin", 2},
		{"mjones@acmetelecom.com", "Maria", "Jones", "operator", 2},
		{"bwilson@nationalvoice.com", "Bob", "Wilson", "org_admin", 3},
		{"alee@securegate.com", "Alice", "Lee", "operator", 4},
		{"tgarcia@pacificbell.com", "Tom", "Garcia", "org_admin", 5},
		{"kpatel@atlantictf.com", "Kavitha", "Patel", "org_admin", 6},
		{"dkim@midwestcarrier.com", "David", "Kim", "operator", 7},
		{"lchen@coastalgateway.com", "Lisa", "Chen", "operator", 8},
		{"viewer@realnumber.local", "View", "Only", "viewer", 1},
		{"operator@realnumber.local", "Test", "Operator", "operator", 1},
	}

	for _, u := range users {
		_, err := db.Exec(
			`INSERT OR IGNORE INTO users (email, password_hash, first_name, last_name, role, org_id) VALUES (?, ?, ?, ?, ?, ?)`,
			u.email, string(hash), u.first, u.last, u.role, u.orgID,
		)
		if err != nil {
			return err
		}
	}
	log.Println("[seed]   10 users created (password: password123)")
	return nil
}

func seedDNONumbers(db *sql.DB) error {
	rng := rand.New(rand.NewSource(42))
	now := time.Now()

	tx, err := db.Begin()
	if err != nil {
		return err
	}
	defer tx.Rollback()

	stmt, err := tx.Prepare(
		`INSERT OR IGNORE INTO dno_numbers (phone_number, dataset, number_type, channel, status, reason, added_by_org_id, added_by_user_id, created_at, updated_at)
		 VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?)`,
	)
	if err != nil {
		return err
	}
	defer stmt.Close()

	inserted := 0

	// --- Auto Set: unassigned/disconnected numbers (bulk, system-generated) ---
	areaCodes := []string{"201", "212", "213", "305", "312", "404", "415", "503", "617", "702", "713", "818", "917", "949", "972"}
	for i := 0; i < 500; i++ {
		ac := areaCodes[rng.Intn(len(areaCodes))]
		phone := fmt.Sprintf("%s%07d", ac, rng.Intn(10000000))
		ch := "voice"
		reason := pickReason(rng, "auto")
		daysAgo := rng.Intn(365)
		created := now.Add(-time.Duration(daysAgo) * 24 * time.Hour)
		_, err := stmt.Exec(phone, "auto", "local", ch, "active", reason, nil, nil, created, created)
		if err == nil {
			inserted++
		}
	}

	// Auto set toll-free
	tfPrefixes := []string{"800", "833", "844", "855", "866", "877", "888"}
	for i := 0; i < 300; i++ {
		pf := tfPrefixes[rng.Intn(len(tfPrefixes))]
		phone := fmt.Sprintf("%s%07d", pf, rng.Intn(10000000))
		ch := "voice"
		if rng.Float32() < 0.2 {
			ch = "both"
		}
		reason := pickReason(rng, "auto")
		daysAgo := rng.Intn(365)
		created := now.Add(-time.Duration(daysAgo) * 24 * time.Hour)
		_, err := stmt.Exec(phone, "auto", "toll_free", ch, "active", reason, nil, nil, created, created)
		if err == nil {
			inserted++
		}
	}

	// --- Subscriber Set: manually flagged by org owners ---
	subscriberReasons := []string{
		"Customer service inbound only",
		"IVR system - never originates calls",
		"Conference bridge number",
		"Vanity advertising number",
		"Fax-only line",
		"Inbound sales hotline",
		"Support queue number",
		"Emergency callback line",
	}
	orgIDs := []int64{2, 3, 5, 6}
	userIDs := []int64{2, 3, 5, 6}
	for i := 0; i < 200; i++ {
		ac := areaCodes[rng.Intn(len(areaCodes))]
		phone := fmt.Sprintf("%s%07d", ac, rng.Intn(10000000))
		orgIdx := rng.Intn(len(orgIDs))
		ch := "voice"
		if rng.Float32() < 0.15 {
			ch = "both"
		}
		reason := subscriberReasons[rng.Intn(len(subscriberReasons))]
		daysAgo := rng.Intn(180)
		created := now.Add(-time.Duration(daysAgo) * 24 * time.Hour)
		_, err := stmt.Exec(phone, "subscriber", "local", ch, "active", reason, orgIDs[orgIdx], userIDs[orgIdx], created, created)
		if err == nil {
			inserted++
		}
	}

	// Subscriber toll-free
	for i := 0; i < 100; i++ {
		pf := tfPrefixes[rng.Intn(len(tfPrefixes))]
		phone := fmt.Sprintf("%s%07d", pf, rng.Intn(10000000))
		orgIdx := rng.Intn(len(orgIDs))
		ch := "voice"
		reason := subscriberReasons[rng.Intn(len(subscriberReasons))]
		daysAgo := rng.Intn(180)
		created := now.Add(-time.Duration(daysAgo) * 24 * time.Hour)
		_, err := stmt.Exec(phone, "subscriber", "toll_free", ch, "active", reason, orgIDs[orgIdx], userIDs[orgIdx], created, created)
		if err == nil {
			inserted++
		}
	}

	// --- ITG Set: traceback-identified spoofed numbers ---
	itgReasons := []string{
		"Traceback: illegal robocall campaign - auto warranty",
		"Traceback: IRS impersonation scam",
		"Traceback: Medicare fraud campaign",
		"Traceback: student loan scam calls",
		"Traceback: tech support scam",
		"Traceback: utility impersonation",
		"Traceback: bank fraud spoofing",
	}
	for i := 0; i < 50; i++ {
		ac := areaCodes[rng.Intn(len(areaCodes))]
		phone := fmt.Sprintf("%s%07d", ac, rng.Intn(10000000))
		reason := itgReasons[rng.Intn(len(itgReasons))]
		daysAgo := rng.Intn(90)
		created := now.Add(-time.Duration(daysAgo) * 24 * time.Hour)
		_, err := stmt.Exec(phone, "itg", "local", "voice", "active", reason, nil, nil, created, created)
		if err == nil {
			inserted++
		}
	}

	// --- TSS Registry Set: non-text-enabled toll-free numbers ---
	for i := 0; i < 150; i++ {
		pf := tfPrefixes[rng.Intn(len(tfPrefixes))]
		phone := fmt.Sprintf("%s%07d", pf, rng.Intn(10000000))
		daysAgo := rng.Intn(365)
		created := now.Add(-time.Duration(daysAgo) * 24 * time.Hour)
		_, err := stmt.Exec(phone, "tss_registry", "toll_free", "text", "active", "Non-text-enabled toll-free number", nil, nil, created, created)
		if err == nil {
			inserted++
		}
	}

	// Some inactive numbers for variety
	for i := 0; i < 30; i++ {
		ac := areaCodes[rng.Intn(len(areaCodes))]
		phone := fmt.Sprintf("%s%07d", ac, rng.Intn(10000000))
		daysAgo := rng.Intn(365)
		created := now.Add(-time.Duration(daysAgo) * 24 * time.Hour)
		_, err := stmt.Exec(phone, "subscriber", "local", "voice", "inactive", "Number reassigned", orgIDs[rng.Intn(len(orgIDs))], nil, created, created)
		if err == nil {
			inserted++
		}
	}

	if err := tx.Commit(); err != nil {
		return err
	}

	log.Printf("[seed]   %d DNO numbers created across all 4 datasets", inserted)
	return nil
}

func seedQueryLogs(db *sql.DB) error {
	rng := rand.New(rand.NewSource(99))
	now := time.Now()

	tx, err := db.Begin()
	if err != nil {
		return err
	}
	defer tx.Rollback()

	stmt, err := tx.Prepare(
		`INSERT INTO query_log (org_id, phone_number, result, channel, queried_at) VALUES (?, ?, ?, ?, ?)`,
	)
	if err != nil {
		return err
	}
	defer stmt.Close()

	queryOrgIDs := []int64{2, 3, 4, 7, 8}
	areaCodes := []string{"201", "212", "305", "312", "415", "617", "713", "800", "855", "888"}

	count := 0
	// Generate queries spread over last 48 hours for good dashboard charts
	for i := 0; i < 2000; i++ {
		orgID := queryOrgIDs[rng.Intn(len(queryOrgIDs))]
		ac := areaCodes[rng.Intn(len(areaCodes))]
		phone := fmt.Sprintf("%s%07d", ac, rng.Intn(10000000))
		result := "miss"
		if rng.Float32() < 0.17 { // ~17% hit rate like real-world
			result = "hit"
		}
		ch := "voice"
		if rng.Float32() < 0.1 {
			ch = "text"
		}
		minutesAgo := rng.Intn(48 * 60)
		queriedAt := now.Add(-time.Duration(minutesAgo) * time.Minute)
		if _, err := stmt.Exec(orgID, phone, result, ch, queriedAt); err != nil {
			return fmt.Errorf("inserting query log row %d: %w", i, err)
		}
		count++
	}

	if err := tx.Commit(); err != nil {
		return fmt.Errorf("committing query logs: %w", err)
	}
	log.Printf("[seed]   %d query log entries created (last 48h)", count)
	return nil
}

func seedAuditLogs(db *sql.DB) error {
	rng := rand.New(rand.NewSource(77))
	now := time.Now()

	tx, err := db.Begin()
	if err != nil {
		return err
	}
	defer tx.Rollback()

	stmt, err := tx.Prepare(
		`INSERT INTO audit_log (user_id, org_id, action, entity_type, entity_id, details, created_at) VALUES (?, ?, ?, ?, ?, ?, ?)`,
	)
	if err != nil {
		return err
	}
	defer stmt.Close()

	actions := []struct {
		action, entityType, detailTpl string
	}{
		{"add", "dno_number", "Added %s to subscriber DNO list"},
		{"add", "dno_number", "Added %s to subscriber DNO list (toll-free)"},
		{"remove", "dno_number", "Removed %s from subscriber DNO list"},
		{"add", "dno_number", "Added %s via bulk upload"},
	}

	areaCodes := []string{"201", "212", "305", "415", "617", "800", "855"}
	userIDs := []int64{1, 2, 3, 5, 6, 7}
	orgIDs := []int64{1, 2, 3, 5, 6, 7}

	count := 0
	for i := 0; i < 200; i++ {
		a := actions[rng.Intn(len(actions))]
		userIdx := rng.Intn(len(userIDs))
		ac := areaCodes[rng.Intn(len(areaCodes))]
		phone := fmt.Sprintf("%s%07d", ac, rng.Intn(10000000))
		detail := fmt.Sprintf(a.detailTpl, phone)
		daysAgo := rng.Intn(30)
		created := now.Add(-time.Duration(daysAgo) * 24 * time.Hour)
		if _, err := stmt.Exec(userIDs[userIdx], orgIDs[userIdx], a.action, a.entityType, rng.Int63n(1000)+1, detail, created); err != nil {
			return fmt.Errorf("inserting audit log row %d: %w", i, err)
		}
		count++
	}

	if err := tx.Commit(); err != nil {
		return fmt.Errorf("committing audit logs: %w", err)
	}
	log.Printf("[seed]   %d audit log entries created", count)
	return nil
}

func pickReason(rng *rand.Rand, dataset string) string {
	autoReasons := []string{
		"Unassigned number in NANP",
		"Disconnected - no active carrier",
		"Spare number pool",
		"Reserved number block",
		"Unassigned in TFNRegistry",
		"Disconnected toll-free number",
		"Number returned to spare pool",
	}

	if dataset == "auto" {
		return autoReasons[rng.Intn(len(autoReasons))]
	}
	return ""
}
