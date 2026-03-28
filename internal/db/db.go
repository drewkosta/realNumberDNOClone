package db

import (
	"database/sql"
	"fmt"
	"os"

	_ "github.com/mattn/go-sqlite3"
	"golang.org/x/crypto/bcrypt"
)

func Initialize(dbPath string) (*sql.DB, error) {
	database, err := sql.Open("sqlite3", dbPath+"?_journal_mode=WAL&_busy_timeout=5000")
	if err != nil {
		return nil, fmt.Errorf("opening database: %w", err)
	}

	// SQLite only supports one concurrent writer; serialize all access to avoid SQLITE_BUSY
	database.SetMaxOpenConns(1)

	if err := database.Ping(); err != nil {
		return nil, fmt.Errorf("pinging database: %w", err)
	}

	if err := runMigrations(database); err != nil {
		return nil, fmt.Errorf("running migrations: %w", err)
	}

	return database, nil
}

func runMigrations(db *sql.DB) error {
	migrations := []string{
		`CREATE TABLE IF NOT EXISTS organizations (
			id INTEGER PRIMARY KEY AUTOINCREMENT,
			name TEXT NOT NULL,
			org_type TEXT NOT NULL CHECK(org_type IN ('resp_org', 'carrier', 'gateway_provider', 'admin')),
			spid TEXT UNIQUE,
			resp_org_id TEXT UNIQUE,
			api_key TEXT UNIQUE,
			created_at DATETIME DEFAULT CURRENT_TIMESTAMP,
			updated_at DATETIME DEFAULT CURRENT_TIMESTAMP
		)`,
		`CREATE TABLE IF NOT EXISTS users (
			id INTEGER PRIMARY KEY AUTOINCREMENT,
			email TEXT NOT NULL UNIQUE,
			password_hash TEXT NOT NULL,
			first_name TEXT NOT NULL,
			last_name TEXT NOT NULL,
			role TEXT NOT NULL CHECK(role IN ('admin', 'org_admin', 'operator', 'viewer')),
			org_id INTEGER REFERENCES organizations(id),
			active INTEGER DEFAULT 1,
			created_at DATETIME DEFAULT CURRENT_TIMESTAMP,
			updated_at DATETIME DEFAULT CURRENT_TIMESTAMP
		)`,
		`CREATE TABLE IF NOT EXISTS dno_numbers (
			id INTEGER PRIMARY KEY AUTOINCREMENT,
			phone_number TEXT NOT NULL,
			dataset TEXT NOT NULL CHECK(dataset IN ('auto', 'subscriber', 'itg', 'tss_registry')),
			number_type TEXT NOT NULL CHECK(number_type IN ('toll_free', 'local')),
			channel TEXT NOT NULL CHECK(channel IN ('voice', 'text', 'both')),
			status TEXT NOT NULL DEFAULT 'active' CHECK(status IN ('active', 'inactive', 'pending')),
			reason TEXT,
			added_by_org_id INTEGER REFERENCES organizations(id),
			added_by_user_id INTEGER REFERENCES users(id),
			created_at DATETIME DEFAULT CURRENT_TIMESTAMP,
			updated_at DATETIME DEFAULT CURRENT_TIMESTAMP,
			UNIQUE(phone_number, channel)
		)`,
		`CREATE INDEX IF NOT EXISTS idx_dno_lookup ON dno_numbers(phone_number, channel, status)`,
		`CREATE INDEX IF NOT EXISTS idx_dno_dataset ON dno_numbers(dataset)`,
		`CREATE INDEX IF NOT EXISTS idx_dno_org ON dno_numbers(added_by_org_id)`,
		`CREATE TABLE IF NOT EXISTS query_log (
			id INTEGER PRIMARY KEY AUTOINCREMENT,
			org_id INTEGER REFERENCES organizations(id),
			phone_number TEXT NOT NULL,
			result TEXT NOT NULL CHECK(result IN ('hit', 'miss')),
			channel TEXT NOT NULL CHECK(channel IN ('voice', 'text')),
			queried_at DATETIME DEFAULT CURRENT_TIMESTAMP
		)`,
		`CREATE INDEX IF NOT EXISTS idx_query_log_org ON query_log(org_id)`,
		`CREATE INDEX IF NOT EXISTS idx_query_log_time ON query_log(queried_at)`,
		`CREATE TABLE IF NOT EXISTS audit_log (
			id INTEGER PRIMARY KEY AUTOINCREMENT,
			user_id INTEGER REFERENCES users(id),
			org_id INTEGER REFERENCES organizations(id),
			action TEXT NOT NULL,
			entity_type TEXT NOT NULL,
			entity_id INTEGER,
			details TEXT,
			created_at DATETIME DEFAULT CURRENT_TIMESTAMP
		)`,
		`CREATE INDEX IF NOT EXISTS idx_audit_org ON audit_log(org_id)`,
		`CREATE INDEX IF NOT EXISTS idx_audit_time ON audit_log(created_at)`,
		`CREATE TABLE IF NOT EXISTS bulk_jobs (
			id INTEGER PRIMARY KEY AUTOINCREMENT,
			org_id INTEGER REFERENCES organizations(id),
			user_id INTEGER REFERENCES users(id),
			job_type TEXT NOT NULL CHECK(job_type IN ('add', 'remove', 'query')),
			status TEXT NOT NULL DEFAULT 'pending' CHECK(status IN ('pending', 'processing', 'completed', 'failed')),
			total_records INTEGER DEFAULT 0,
			processed_records INTEGER DEFAULT 0,
			success_count INTEGER DEFAULT 0,
			error_count INTEGER DEFAULT 0,
			file_name TEXT,
			result_summary TEXT,
			created_at DATETIME DEFAULT CURRENT_TIMESTAMP,
			completed_at DATETIME
		)`,
		// Drop old single-column indexes replaced by composite
		`DROP INDEX IF EXISTS idx_dno_phone`,
		`DROP INDEX IF EXISTS idx_dno_status`,
	}

	for _, m := range migrations {
		if _, err := db.Exec(m); err != nil {
			return fmt.Errorf("executing migration: %w", err)
		}
	}

	return seedAdminUser(db)
}

func seedAdminUser(db *sql.DB) error {
	var count int
	if err := db.QueryRow("SELECT COUNT(*) FROM users WHERE role = 'admin'").Scan(&count); err != nil {
		return fmt.Errorf("checking admin count: %w", err)
	}
	if count > 0 {
		return nil
	}

	_, err := db.Exec(`INSERT OR IGNORE INTO organizations (name, org_type) VALUES ('System Admin', 'admin')`)
	if err != nil {
		return err
	}

	var orgID int64
	err = db.QueryRow("SELECT id FROM organizations WHERE name = 'System Admin'").Scan(&orgID)
	if err != nil {
		return err
	}

	password := os.Getenv("ADMIN_PASSWORD")
	if password == "" {
		password = "admin123"
	}

	hash, err := bcrypt.GenerateFromPassword([]byte(password), bcrypt.DefaultCost)
	if err != nil {
		return fmt.Errorf("hashing default password: %w", err)
	}
	_, err = db.Exec(`INSERT INTO users (email, password_hash, first_name, last_name, role, org_id)
		VALUES ('admin@realnumber.local', ?, 'System', 'Admin', 'admin', ?)`, hash, orgID)
	return err
}
