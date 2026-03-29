package config

import (
	"os"
	"testing"
)

func TestLoad_ValidEnvironments(t *testing.T) {
	for _, env := range []string{"local", "dev", "testing"} {
		cfg, err := Load(env)
		if err != nil {
			t.Errorf("Load(%q): %v", env, err)
			continue
		}
		if cfg.Env != Environment(env) {
			t.Errorf("env = %q, want %q", cfg.Env, env)
		}
		if cfg.DBDriver != DBDriverSQLite {
			t.Errorf("%s: driver = %q, want sqlite", env, cfg.DBDriver)
		}
	}
}

func TestLoad_InvalidEnvironment(t *testing.T) {
	_, err := Load("invalid")
	if err == nil {
		t.Error("expected error for invalid environment")
	}
}

func TestLoad_StagingRequiresSecrets(t *testing.T) {
	// Ensure no env vars leak
	os.Unsetenv("JWT_SECRET")
	os.Unsetenv("DATABASE_URL")

	_, err := Load("staging")
	if err == nil {
		t.Error("staging should require JWT_SECRET")
	}
}

func TestLoad_EnvVarOverrides(t *testing.T) {
	t.Setenv("PORT", "9999")
	t.Setenv("LOG_LEVEL", "error")

	cfg, err := Load("local")
	if err != nil {
		t.Fatalf("Load: %v", err)
	}
	if cfg.Port != "9999" {
		t.Errorf("port = %q, want 9999", cfg.Port)
	}
	if cfg.LogLevel != "error" {
		t.Errorf("logLevel = %q, want error", cfg.LogLevel)
	}
}

func TestLoad_PostgresRequiresDSN(t *testing.T) {
	t.Setenv("JWT_SECRET", "test")
	t.Setenv("DATABASE_URL", "")

	_, err := Load("staging")
	if err == nil {
		t.Error("postgres driver should require DATABASE_URL")
	}
}

func TestUseSQLite(t *testing.T) {
	cfg, _ := Load("local")
	if !cfg.UseSQLite() {
		t.Error("local should use sqlite")
	}
}
