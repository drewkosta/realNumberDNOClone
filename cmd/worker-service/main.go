package main

import (
	"context"
	"flag"
	"os/signal"
	"syscall"
	"time"

	"realNumberDNOClone/internal/boot"
	"realNumberDNOClone/internal/cache"
	"realNumberDNOClone/internal/db"
	"realNumberDNOClone/internal/jobs"
	"realNumberDNOClone/internal/models"
	"realNumberDNOClone/internal/querylog"
	"realNumberDNOClone/internal/service"
)

func main() {
	env := flag.String("env", "local", "environment")
	seed := flag.Bool("seed", false, "seed database with mock data")
	flag.Parse()

	app, err := boot.Init(*env)
	if err != nil {
		panic(err)
	}
	defer app.Close()

	app.Logger.Info("Starting worker-service", "env", app.Cfg.Env)

	if *seed {
		if !app.Cfg.AllowSeed {
			app.Logger.Error("Seeding not allowed", "env", app.Cfg.Env)
			return
		}
		if err := db.SeedLocalData(app.DB); err != nil {
			app.Logger.Error("Seed failed", "error", err)
			return
		}
	}

	qlWriter := querylog.NewAsyncWriter(
		app.DB.Writer,
		app.Cfg.QueryLogFlushSize,
		time.Duration(app.Cfg.QueryLogFlushInterval)*time.Second,
		app.Logger,
	)

	dnoService := service.NewDNOService(app.DB, qlWriter,
		cache.New[*models.DNOQueryResponse](30*time.Second, 1000),
		cache.New[*models.AnalyticsSummary](60*time.Second, 10),
	)
	featuresService := service.NewFeaturesService(app.DB, app.Logger)
	dnoService.SetWebhookFirer(featuresService)

	jobWorker := jobs.NewWorker(app.DB.Writer, dnoService.AddNumber, app.Logger)
	jobWorker.Start()

	app.Logger.Info("Worker running, polling for jobs...")

	ctx, stop := signal.NotifyContext(context.Background(), syscall.SIGINT, syscall.SIGTERM)
	defer stop()
	<-ctx.Done()

	app.Logger.Info("Shutting down worker...")
	jobWorker.Stop()
	qlWriter.Stop()
	app.Logger.Info("Worker stopped")
}
