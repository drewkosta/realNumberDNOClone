package main

import (
	"context"
	"flag"
	"log/slog"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"realNumberDNOClone/internal/api"
	"realNumberDNOClone/internal/cache"
	"realNumberDNOClone/internal/config"
	"realNumberDNOClone/internal/db"
	"realNumberDNOClone/internal/models"
	"realNumberDNOClone/internal/querylog"
)

func main() {
	env := flag.String("env", "local", "environment: local, dev, staging, testing, pre-prod, production")
	seed := flag.Bool("seed", false, "seed the database with mock data (only allowed in local/dev/testing)")
	flag.Parse()

	cfg, err := config.Load(*env)
	if err != nil {
		slog.Error("Failed to load config", "error", err)
		os.Exit(1)
	}

	// Structured logger
	logLevel := slog.LevelInfo
	switch cfg.LogLevel {
	case "debug":
		logLevel = slog.LevelDebug
	case "warn":
		logLevel = slog.LevelWarn
	case "error":
		logLevel = slog.LevelError
	}
	logger := slog.New(slog.NewJSONHandler(os.Stdout, &slog.HandlerOptions{Level: logLevel}))
	slog.SetDefault(logger)

	logger.Info("Initializing database", "env", cfg.Env, "path", cfg.DBPath)
	database, err := db.Initialize(cfg.DBPath)
	if err != nil {
		logger.Error("Failed to initialize database", "error", err)
		os.Exit(1)
	}
	defer database.Close()

	if *seed {
		if !cfg.AllowSeed {
			logger.Error("Seeding is not allowed", "env", cfg.Env)
			os.Exit(1)
		}
		if err := db.SeedLocalData(database); err != nil {
			logger.Error("Failed to seed database", "error", err)
			os.Exit(1)
		}
	}

	// Async query log writer
	qlWriter := querylog.NewAsyncWriter(
		database,
		cfg.QueryLogFlushSize,
		time.Duration(cfg.QueryLogFlushInterval)*time.Second,
		logger,
	)

	// DNO lookup cache
	var dnoCache *cache.TTLCache[*models.DNOQueryResponse]
	if cfg.DNOCacheTTLSeconds > 0 {
		dnoCache = cache.New[*models.DNOQueryResponse](
			time.Duration(cfg.DNOCacheTTLSeconds)*time.Second,
			100000,
		)
	}

	// Analytics cache
	var analyticsCache *cache.TTLCache[*models.AnalyticsSummary]
	if cfg.AnalyticsCacheTTLSeconds > 0 {
		analyticsCache = cache.New[*models.AnalyticsSummary](
			time.Duration(cfg.AnalyticsCacheTTLSeconds)*time.Second,
			100,
		)
	}

	router := api.NewRouter(database, cfg, qlWriter, dnoCache, analyticsCache, logger)

	srv := &http.Server{
		Addr:              ":" + cfg.Port,
		Handler:           router,
		ReadHeaderTimeout: 5 * time.Second,
		ReadTimeout:       10 * time.Second,
		WriteTimeout:      30 * time.Second,
		IdleTimeout:       120 * time.Second,
	}

	ctx, stop := signal.NotifyContext(context.Background(), syscall.SIGINT, syscall.SIGTERM)
	defer stop()

	go func() {
		logger.Info("Server starting", "env", cfg.Env, "port", cfg.Port)
		if err := srv.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			logger.Error("Server failed", "error", err)
			os.Exit(1)
		}
	}()

	<-ctx.Done()
	logger.Info("Shutting down server...")

	shutdownCtx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	qlWriter.Stop()

	if err := srv.Shutdown(shutdownCtx); err != nil {
		logger.Error("Server forced to shutdown", "error", err)
		os.Exit(1)
	}

	logger.Info("Server stopped")
}
