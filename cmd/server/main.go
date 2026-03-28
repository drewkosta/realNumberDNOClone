package main

import (
	"context"
	"flag"
	"log"
	"net/http"
	"os/signal"
	"syscall"
	"time"

	"realNumberDNOClone/internal/api"
	"realNumberDNOClone/internal/config"
	"realNumberDNOClone/internal/db"
)

func main() {
	env := flag.String("env", "local", "environment: local, dev, staging, testing, pre-prod, production")
	seed := flag.Bool("seed", false, "seed the database with mock data (only allowed in local/dev/testing)")
	flag.Parse()

	cfg, err := config.Load(*env)
	if err != nil {
		log.Fatalf("Failed to load config: %v", err)
	}

	log.Printf("[%s] Initializing database at %s", cfg.Env, cfg.DBPath)
	database, err := db.Initialize(cfg.DBPath)
	if err != nil {
		log.Fatalf("Failed to initialize database: %v", err)
	}
	defer database.Close()

	if *seed {
		if !cfg.AllowSeed {
			log.Fatalf("Seeding is not allowed in %s environment", cfg.Env)
		}
		if err := db.SeedLocalData(database); err != nil {
			log.Fatalf("Failed to seed database: %v", err)
		}
	}

	router := api.NewRouter(database, cfg)

	srv := &http.Server{
		Addr:    ":" + cfg.Port,
		Handler: router,
	}

	// Listen for SIGINT/SIGTERM in a separate goroutine
	ctx, stop := signal.NotifyContext(context.Background(), syscall.SIGINT, syscall.SIGTERM)
	defer stop()

	go func() {
		log.Printf("[%s] RealNumber DNO server starting on :%s", cfg.Env, cfg.Port)
		if err := srv.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			log.Fatalf("Server failed: %v", err)
		}
	}()

	<-ctx.Done()
	log.Println("Shutting down server...")

	shutdownCtx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	if err := srv.Shutdown(shutdownCtx); err != nil {
		log.Fatalf("Server forced to shutdown: %v", err)
	}

	log.Println("Server stopped")
}
