package db

import (
	"context"
	"database/sql"
	"fmt"
	"os"
	"regexp"
	"strings"
	"time"

	_ "github.com/jackc/pgx/v5/stdlib"
	_ "github.com/mattn/go-sqlite3"
	"golang.org/x/crypto/bcrypt"

	"realNumberDNOClone/internal/config"
)

// DB wraps separate reader and writer database connections.
// PostgreSQL is the default dialect -- all service SQL uses $N placeholders.
// For SQLite (local dev only), Q() rewrites $N -> ? automatically.
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
	if err := runMigrationsSQLite(writer); err != nil {
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
	if err := withAdvisoryLock(conn, func() error { return runMigrationsPostgres(conn) }); err != nil {
		d.Close()
		return nil, fmt.Errorf("running postgres migrations: %w", err)
	}
	if err := d.EnsureQueryLogPartitions(); err != nil {
		d.Close()
		return nil, fmt.Errorf("creating query_log partitions: %w", err)
	}
	return d, nil
}

// withAdvisoryLock acquires a PostgreSQL advisory lock to prevent multiple
// instances from running migrations simultaneously. Lock ID is arbitrary but
// fixed for this application.
func withAdvisoryLock(db *sql.DB, fn func() error) error {
	const lockID = 42_000_001 // arbitrary unique ID for migration lock
	_, err := db.Exec("SELECT pg_advisory_lock($1)", lockID)
	if err != nil {
		return fmt.Errorf("acquiring migration lock: %w", err)
	}
	defer db.Exec("SELECT pg_advisory_unlock($1)", lockID)
	return fn()
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

func (d *DB) IsSQLite() bool {
	return d.driver == config.DBDriverSQLite
}

var dollarParamRe = regexp.MustCompile(`\$\d+`)

// Q adapts a query for the current driver. Queries are written in PostgreSQL
// dialect ($1, $2, ...) by default. For SQLite, $N placeholders are rewritten
// to ? and strftime is used instead of date_trunc.
func (d *DB) Q(query string) string {
	if d.IsSQLite() {
		return dollarParamRe.ReplaceAllString(query, "?")
	}
	return query
}

// QTimeTrunc returns a SQL expression that truncates a timestamp column to
// the hour. PostgreSQL uses date_trunc; SQLite uses strftime.
func (d *DB) QTimeTrunc(col string) string {
	if d.IsSQLite() {
		return fmt.Sprintf("strftime('%%Y-%%m-%%d %%H:00', %s)", col)
	}
	return fmt.Sprintf("date_trunc('hour', %s)::text", col)
}

// QUpsertTimestamp returns the expression for "current timestamp" in an
// ON CONFLICT update clause.
func (d *DB) QNow() string {
	if d.IsSQLite() {
		return "CURRENT_TIMESTAMP"
	}
	return "NOW()"
}

func runMigrationsSQLite(db *sql.DB) error {
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
		// ITG traceback metadata columns
		`ALTER TABLE dno_numbers ADD COLUMN investigation_id TEXT`,
		`ALTER TABLE dno_numbers ADD COLUMN threat_category TEXT`,
		// Webhook subscriptions
		`CREATE TABLE IF NOT EXISTS webhook_subscriptions (
			id INTEGER PRIMARY KEY AUTOINCREMENT,
			org_id INTEGER NOT NULL REFERENCES organizations(id),
			url TEXT NOT NULL,
			secret TEXT NOT NULL,
			events TEXT NOT NULL DEFAULT 'dno.added,dno.removed',
			active INTEGER DEFAULT 1,
			created_at DATETIME DEFAULT CURRENT_TIMESTAMP
		)`,
		// Mock number registry (simulates TFNRegistry/NPAC ownership data)
		`CREATE TABLE IF NOT EXISTS number_registry (
			phone_number TEXT PRIMARY KEY,
			owner_org_id INTEGER REFERENCES organizations(id),
			number_type TEXT NOT NULL CHECK(number_type IN ('toll_free', 'local')),
			status TEXT NOT NULL DEFAULT 'assigned' CHECK(status IN ('assigned', 'unassigned', 'ported', 'disconnected')),
			text_enabled INTEGER DEFAULT 0,
			updated_at DATETIME DEFAULT CURRENT_TIMESTAMP
		)`,
	}
	return execMigrations(db, migrations)
}

func runMigrationsPostgres(db *sql.DB) error {
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
		// Partitioned query_log for PostgreSQL -- partition by month
		`CREATE TABLE IF NOT EXISTS query_log (
			id SERIAL,
			org_id INTEGER REFERENCES organizations(id),
			phone_number TEXT NOT NULL,
			result TEXT NOT NULL CHECK(result IN ('hit', 'miss')),
			channel TEXT NOT NULL CHECK(channel IN ('voice', 'text')),
			queried_at TIMESTAMPTZ DEFAULT NOW(),
			PRIMARY KEY (id, queried_at)
		) PARTITION BY RANGE (queried_at)`,
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
		// ITG traceback metadata columns
		`ALTER TABLE dno_numbers ADD COLUMN IF NOT EXISTS investigation_id TEXT`,
		`ALTER TABLE dno_numbers ADD COLUMN IF NOT EXISTS threat_category TEXT`,
		// Webhook subscriptions
		`CREATE TABLE IF NOT EXISTS webhook_subscriptions (
			id SERIAL PRIMARY KEY,
			org_id INTEGER NOT NULL REFERENCES organizations(id),
			url TEXT NOT NULL,
			secret TEXT NOT NULL,
			events TEXT NOT NULL DEFAULT 'dno.added,dno.removed',
			active BOOLEAN DEFAULT TRUE,
			created_at TIMESTAMPTZ DEFAULT NOW()
		)`,
		// Mock number registry (simulates TFNRegistry/NPAC ownership data)
		`CREATE TABLE IF NOT EXISTS number_registry (
			phone_number TEXT PRIMARY KEY,
			owner_org_id INTEGER REFERENCES organizations(id),
			number_type TEXT NOT NULL CHECK(number_type IN ('toll_free', 'local')),
			status TEXT NOT NULL DEFAULT 'assigned' CHECK(status IN ('assigned', 'unassigned', 'ported', 'disconnected')),
			text_enabled BOOLEAN DEFAULT FALSE,
			updated_at TIMESTAMPTZ DEFAULT NOW()
		)`,
	}
	return execMigrations(db, migrations)
}

func execMigrations(db *sql.DB, migrations []string) error {
	for _, m := range migrations {
		_, err := db.Exec(m)
		if err != nil {
			// SQLite doesn't support IF NOT EXISTS for ALTER TABLE -- skip duplicate column errors
			errStr := err.Error()
			if strings.Contains(errStr, "duplicate column") || strings.Contains(errStr, "already exists") {
				continue
			}
			return fmt.Errorf("executing migration: %w", err)
		}
	}
	return seedAdminUser(db)
}

func seedAdminUser(sqldb *sql.DB) error {
	var count int
	if err := sqldb.QueryRow("SELECT COUNT(*) FROM users WHERE role = 'admin'").Scan(&count); err != nil {
		return fmt.Errorf("checking admin count: %w", err)
	}
	if count > 0 {
		return nil
	}

	_, err := sqldb.Exec(`INSERT INTO organizations (name, org_type) SELECT 'System Admin', 'admin' WHERE NOT EXISTS (SELECT 1 FROM organizations WHERE name = 'System Admin')`)
	if err != nil {
		return err
	}

	var orgID int64
	if err := sqldb.QueryRow("SELECT id FROM organizations WHERE name = 'System Admin'").Scan(&orgID); err != nil {
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

	// Seed uses portable SQL without parameterized placeholders for simplicity
	// (only runs once on first boot, not in hot path)
	_, err = sqldb.Exec(
		`INSERT INTO users (email, password_hash, first_name, last_name, role, org_id)
		 SELECT 'admin@realnumber.local', '`+string(hash)+`', 'System', 'Admin', 'admin', `+fmt.Sprintf("%d", orgID)+`
		 WHERE NOT EXISTS (SELECT 1 FROM users WHERE email = 'admin@realnumber.local')`)
	return err
}

// EnsureQueryLogPartitions creates monthly partitions for the query_log table
// for the current month and next 2 months. Safe to call repeatedly (idempotent).
// Only applies to PostgreSQL; no-op for SQLite.
func (d *DB) EnsureQueryLogPartitions() error {
	if d.IsSQLite() {
		return nil
	}

	now := time.Now().UTC()
	for i := 0; i < 3; i++ {
		t := now.AddDate(0, i, 0)
		partName := fmt.Sprintf("query_log_%d_%02d", t.Year(), t.Month())
		start := time.Date(t.Year(), t.Month(), 1, 0, 0, 0, 0, time.UTC)
		end := start.AddDate(0, 1, 0)

		sql := fmt.Sprintf(
			`CREATE TABLE IF NOT EXISTS %s PARTITION OF query_log FOR VALUES FROM ('%s') TO ('%s')`,
			partName,
			start.Format("2006-01-02"),
			end.Format("2006-01-02"),
		)
		if _, err := d.Writer.Exec(sql); err != nil {
			// Partition might already exist -- that's fine
			if !strings.Contains(err.Error(), "already exists") {
				return fmt.Errorf("creating partition %s: %w", partName, err)
			}
		}
	}
	return nil
}
