package db

import (
	"context"
	"database/sql"
	"fmt"
	"os"
	"strconv"
	"strings"

	_ "github.com/jackc/pgx/v5/stdlib"
	_ "github.com/mattn/go-sqlite3"
	"golang.org/x/crypto/bcrypt"

	"realNumberDNOClone/internal/config"
)

// DB wraps separate reader and writer database connections.
// For SQLite: writer is single-connection, reader is a pool (WAL concurrent reads).
// For PostgreSQL: both point to the same connection pool.
type DB struct {
	Writer *sql.DB
	Reader *sql.DB
	driver config.DBDriver
}

func Initialize(cfg *config.Config) (*DB, error) {
	if cfg.UseSQLite() {
		return initSQLite(cfg.DBPath)
	}
	return initPostgres(cfg.DBDSN)
}

func initSQLite(dbPath string) (*DB, error) {
	dsn := dbPath + "?_journal_mode=WAL&_busy_timeout=5000"

	writer, err := sql.Open("sqlite3", dsn)
	if err != nil {
		return nil, fmt.Errorf("opening sqlite writer: %w", err)
	}
	writer.SetMaxOpenConns(1)

	reader, err := sql.Open("sqlite3", dsn+"&_query_only=true")
	if err != nil {
		writer.Close()
		return nil, fmt.Errorf("opening sqlite reader: %w", err)
	}
	reader.SetMaxOpenConns(10)

	if err := writer.Ping(); err != nil {
		writer.Close()
		reader.Close()
		return nil, fmt.Errorf("pinging sqlite: %w", err)
	}

	d := &DB{Writer: writer, Reader: reader, driver: config.DBDriverSQLite}
	if err := runMigrationsSQLite(writer, config.DBDriverSQLite); err != nil {
		d.Close()
		return nil, fmt.Errorf("running sqlite migrations: %w", err)
	}
	return d, nil
}

func initPostgres(dsn string) (*DB, error) {
	conn, err := sql.Open("pgx", dsn)
	if err != nil {
		return nil, fmt.Errorf("opening postgres: %w", err)
	}
	conn.SetMaxOpenConns(25)
	conn.SetMaxIdleConns(10)

	if err := conn.Ping(); err != nil {
		conn.Close()
		return nil, fmt.Errorf("pinging postgres: %w", err)
	}

	d := &DB{Writer: conn, Reader: conn, driver: config.DBDriverPostgres}
	if err := runMigrationsPostgres(conn, config.DBDriverPostgres); err != nil {
		d.Close()
		return nil, fmt.Errorf("running postgres migrations: %w", err)
	}
	return d, nil
}

func (d *DB) Close() {
	if d.Writer != nil {
		d.Writer.Close()
	}
	if d.Reader != nil && d.Reader != d.Writer {
		d.Reader.Close()
	}
}

func (d *DB) Ping(ctx context.Context) error {
	return d.Writer.PingContext(ctx)
}

func (d *DB) IsPostgres() bool {
	return d.driver == config.DBDriverPostgres
}

// rewritePlaceholders converts ? placeholders to $1, $2, ... for PostgreSQL.
func rewritePlaceholders(query string) string {
	idx := 0
	var b strings.Builder
	b.Grow(len(query) + 20)
	for i := 0; i < len(query); i++ {
		if query[i] == '?' {
			idx++
			b.WriteByte('$')
			b.WriteString(strconv.Itoa(idx))
		} else {
			b.WriteByte(query[i])
		}
	}
	return b.String()
}

// Q returns the query with placeholders rewritten for the current driver.
func (d *DB) Q(query string) string {
	if d.IsPostgres() {
		return rewritePlaceholders(query)
	}
	return query
}

func runMigrationsSQLite(db *sql.DB, driver config.DBDriver) error {
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
		`DROP INDEX IF EXISTS idx_dno_phone`,
		`DROP INDEX IF EXISTS idx_dno_status`,
	}
	return execMigrations(db, driver, migrations)
}

func runMigrationsPostgres(db *sql.DB, driver config.DBDriver) error {
	migrations := []string{
		`CREATE TABLE IF NOT EXISTS organizations (
			id SERIAL PRIMARY KEY,
			name TEXT NOT NULL,
			org_type TEXT NOT NULL CHECK(org_type IN ('resp_org', 'carrier', 'gateway_provider', 'admin')),
			spid TEXT UNIQUE,
			resp_org_id TEXT UNIQUE,
			api_key TEXT UNIQUE,
			created_at TIMESTAMPTZ DEFAULT NOW(),
			updated_at TIMESTAMPTZ DEFAULT NOW()
		)`,
		`CREATE TABLE IF NOT EXISTS users (
			id SERIAL PRIMARY KEY,
			email TEXT NOT NULL UNIQUE,
			password_hash TEXT NOT NULL,
			first_name TEXT NOT NULL,
			last_name TEXT NOT NULL,
			role TEXT NOT NULL CHECK(role IN ('admin', 'org_admin', 'operator', 'viewer')),
			org_id INTEGER REFERENCES organizations(id),
			active BOOLEAN DEFAULT TRUE,
			created_at TIMESTAMPTZ DEFAULT NOW(),
			updated_at TIMESTAMPTZ DEFAULT NOW()
		)`,
		`CREATE TABLE IF NOT EXISTS dno_numbers (
			id SERIAL PRIMARY KEY,
			phone_number TEXT NOT NULL,
			dataset TEXT NOT NULL CHECK(dataset IN ('auto', 'subscriber', 'itg', 'tss_registry')),
			number_type TEXT NOT NULL CHECK(number_type IN ('toll_free', 'local')),
			channel TEXT NOT NULL CHECK(channel IN ('voice', 'text', 'both')),
			status TEXT NOT NULL DEFAULT 'active' CHECK(status IN ('active', 'inactive', 'pending')),
			reason TEXT,
			added_by_org_id INTEGER REFERENCES organizations(id),
			added_by_user_id INTEGER REFERENCES users(id),
			created_at TIMESTAMPTZ DEFAULT NOW(),
			updated_at TIMESTAMPTZ DEFAULT NOW(),
			UNIQUE(phone_number, channel)
		)`,
		`CREATE INDEX IF NOT EXISTS idx_dno_lookup ON dno_numbers(phone_number, channel, status)`,
		`CREATE INDEX IF NOT EXISTS idx_dno_dataset ON dno_numbers(dataset)`,
		`CREATE INDEX IF NOT EXISTS idx_dno_org ON dno_numbers(added_by_org_id)`,
		`CREATE TABLE IF NOT EXISTS query_log (
			id SERIAL PRIMARY KEY,
			org_id INTEGER REFERENCES organizations(id),
			phone_number TEXT NOT NULL,
			result TEXT NOT NULL CHECK(result IN ('hit', 'miss')),
			channel TEXT NOT NULL CHECK(channel IN ('voice', 'text')),
			queried_at TIMESTAMPTZ DEFAULT NOW()
		)`,
		`CREATE INDEX IF NOT EXISTS idx_query_log_org ON query_log(org_id)`,
		`CREATE INDEX IF NOT EXISTS idx_query_log_time ON query_log(queried_at)`,
		`CREATE TABLE IF NOT EXISTS audit_log (
			id SERIAL PRIMARY KEY,
			user_id INTEGER REFERENCES users(id),
			org_id INTEGER REFERENCES organizations(id),
			action TEXT NOT NULL,
			entity_type TEXT NOT NULL,
			entity_id INTEGER,
			details TEXT,
			created_at TIMESTAMPTZ DEFAULT NOW()
		)`,
		`CREATE INDEX IF NOT EXISTS idx_audit_org ON audit_log(org_id)`,
		`CREATE INDEX IF NOT EXISTS idx_audit_time ON audit_log(created_at)`,
		`CREATE TABLE IF NOT EXISTS bulk_jobs (
			id SERIAL PRIMARY KEY,
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
			created_at TIMESTAMPTZ DEFAULT NOW(),
			completed_at TIMESTAMPTZ
		)`,
	}
	return execMigrations(db, driver, migrations)
}

func execMigrations(db *sql.DB, driver config.DBDriver, migrations []string) error {
	for _, m := range migrations {
		if _, err := db.Exec(m); err != nil {
			return fmt.Errorf("executing migration: %w", err)
		}
	}
	return seedAdminUser(db, driver)
}

func seedAdminUser(db *sql.DB, driver config.DBDriver) error {
	q := func(query string) string {
		if driver == config.DBDriverPostgres {
			return rewritePlaceholders(query)
		}
		return query
	}
	var count int
	if err := db.QueryRow("SELECT COUNT(*) FROM users WHERE role = 'admin'").Scan(&count); err != nil {
		return fmt.Errorf("checking admin count: %w", err)
	}
	if count > 0 {
		return nil
	}

	_, err := db.Exec(`INSERT INTO organizations (name, org_type) SELECT 'System Admin', 'admin' WHERE NOT EXISTS (SELECT 1 FROM organizations WHERE name = 'System Admin')`)
	if err != nil {
		return err
	}

	var orgID int64
	if err := db.QueryRow("SELECT id FROM organizations WHERE name = 'System Admin'").Scan(&orgID); err != nil {
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

	_, err = db.Exec(q(
		`INSERT INTO users (email, password_hash, first_name, last_name, role, org_id)
		 SELECT 'admin@realnumber.local', ?, 'System', 'Admin', 'admin', ?
		 WHERE NOT EXISTS (SELECT 1 FROM users WHERE email = 'admin@realnumber.local')`),
		string(hash), orgID)
	return err
}
