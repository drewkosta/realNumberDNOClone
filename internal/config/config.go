package config

import (
	"fmt"
	"os"
	"strconv"
)

type Environment string

const (
	EnvLocal      Environment = "local"
	EnvDev        Environment = "dev"
	EnvStaging    Environment = "staging"
	EnvTesting    Environment = "testing"
	EnvPreProd    Environment = "pre-prod"
	EnvProduction Environment = "production"
)

type DBDriver string

const (
	DBDriverSQLite   DBDriver = "sqlite"
	DBDriverPostgres DBDriver = "postgres"
)

type Config struct {
	Env        Environment
	Port       string
	DBDriver   DBDriver
	DBPath     string // SQLite file path
	DBDSN      string // PostgreSQL connection string
	JWTSecret  string
	LogLevel   string
	CORSOrigin string

	AllowSeed      bool
	EnableDebugLog bool

	// Rate limiting
	RateLimitRPS   int // requests per second per key (0 = disabled)
	RateLimitBurst int

	// Cache
	DNOCacheTTLSeconds       int
	AnalyticsCacheTTLSeconds int

	// Query log
	QueryLogFlushSize     int
	QueryLogFlushInterval int // seconds
}

var envDefaults = map[Environment]Config{
	EnvLocal: {
		Port:                     "8080",
		DBDriver:                 DBDriverSQLite,
		DBPath:                   "realnumber_local.db",
		JWTSecret:                "local-dev-secret-not-for-prod",
		LogLevel:                 "debug",
		CORSOrigin:               "http://localhost:5173",
		AllowSeed:                true,
		EnableDebugLog:           true,
		RateLimitRPS:             0,
		RateLimitBurst:           0,
		DNOCacheTTLSeconds:       30,
		AnalyticsCacheTTLSeconds: 30,
		QueryLogFlushSize:        100,
		QueryLogFlushInterval:    5,
	},
	EnvDev: {
		Port:                     "8080",
		DBDriver:                 DBDriverSQLite,
		DBPath:                   "realnumber_dev.db",
		JWTSecret:                "dev-secret-change-in-production",
		LogLevel:                 "debug",
		CORSOrigin:               "http://localhost:5173",
		AllowSeed:                true,
		EnableDebugLog:           true,
		RateLimitRPS:             100,
		RateLimitBurst:           200,
		DNOCacheTTLSeconds:       30,
		AnalyticsCacheTTLSeconds: 30,
		QueryLogFlushSize:        200,
		QueryLogFlushInterval:    3,
	},
	EnvTesting: {
		Port:                     "8081",
		DBDriver:                 DBDriverSQLite,
		DBPath:                   "realnumber_test.db",
		JWTSecret:                "testing-secret",
		LogLevel:                 "info",
		CORSOrigin:               "*",
		AllowSeed:                true,
		EnableDebugLog:           false,
		RateLimitRPS:             0,
		RateLimitBurst:           0,
		DNOCacheTTLSeconds:       0,
		AnalyticsCacheTTLSeconds: 0,
		QueryLogFlushSize:        50,
		QueryLogFlushInterval:    1,
	},
	EnvStaging: {
		Port:                     "8080",
		DBDriver:                 DBDriverPostgres,
		DBDSN:                    "",
		JWTSecret:                "",
		LogLevel:                 "info",
		CORSOrigin:               "",
		AllowSeed:                false,
		EnableDebugLog:           false,
		RateLimitRPS:             500,
		RateLimitBurst:           1000,
		DNOCacheTTLSeconds:       60,
		AnalyticsCacheTTLSeconds: 60,
		QueryLogFlushSize:        500,
		QueryLogFlushInterval:    2,
	},
	EnvPreProd: {
		Port:                     "8080",
		DBDriver:                 DBDriverPostgres,
		DBDSN:                    "",
		JWTSecret:                "",
		LogLevel:                 "warn",
		CORSOrigin:               "",
		AllowSeed:                false,
		EnableDebugLog:           false,
		RateLimitRPS:             1000,
		RateLimitBurst:           2000,
		DNOCacheTTLSeconds:       60,
		AnalyticsCacheTTLSeconds: 60,
		QueryLogFlushSize:        1000,
		QueryLogFlushInterval:    1,
	},
	EnvProduction: {
		Port:                     "8080",
		DBDriver:                 DBDriverPostgres,
		DBDSN:                    "",
		JWTSecret:                "",
		LogLevel:                 "error",
		CORSOrigin:               "",
		AllowSeed:                false,
		EnableDebugLog:           false,
		RateLimitRPS:             2000,
		RateLimitBurst:           5000,
		DNOCacheTTLSeconds:       45,
		AnalyticsCacheTTLSeconds: 60,
		QueryLogFlushSize:        2000,
		QueryLogFlushInterval:    1,
	},
}

func Load(env string) (*Config, error) {
	e := Environment(env)
	defaults, ok := envDefaults[e]
	if !ok {
		return nil, fmt.Errorf("unknown environment %q (valid: local, dev, staging, testing, pre-prod, production)", env)
	}

	cfg := defaults
	cfg.Env = e

	if v := os.Getenv("PORT"); v != "" {
		cfg.Port = v
	}
	if v := os.Getenv("DB_DRIVER"); v != "" {
		cfg.DBDriver = DBDriver(v)
	}
	if v := os.Getenv("DB_PATH"); v != "" {
		cfg.DBPath = v
	}
	if v := os.Getenv("DATABASE_URL"); v != "" {
		cfg.DBDSN = v
	}
	if v := os.Getenv("JWT_SECRET"); v != "" {
		cfg.JWTSecret = v
	}
	if v := os.Getenv("LOG_LEVEL"); v != "" {
		cfg.LogLevel = v
	}
	if v := os.Getenv("CORS_ORIGIN"); v != "" {
		cfg.CORSOrigin = v
	}
	if v := os.Getenv("ALLOW_SEED"); v != "" {
		cfg.AllowSeed, _ = strconv.ParseBool(v)
	}
	if v := os.Getenv("RATE_LIMIT_RPS"); v != "" {
		cfg.RateLimitRPS, _ = strconv.Atoi(v)
	}

	if cfg.JWTSecret == "" && (e == EnvStaging || e == EnvPreProd || e == EnvProduction) {
		return nil, fmt.Errorf("JWT_SECRET is required for %s environment", env)
	}
	if cfg.DBDriver == DBDriverPostgres && cfg.DBDSN == "" {
		return nil, fmt.Errorf("DATABASE_URL is required when using postgres driver (env: %s)", env)
	}

	return &cfg, nil
}

func (c *Config) UseSQLite() bool {
	return c.DBDriver == DBDriverSQLite
}

func (c *Config) IsProduction() bool {
	return c.Env == EnvProduction
}

func (c *Config) IsProdLike() bool {
	return c.Env == EnvStaging || c.Env == EnvPreProd || c.Env == EnvProduction
}
