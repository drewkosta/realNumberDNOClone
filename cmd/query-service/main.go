package main

import (
	"flag"
	"os"
	"time"

	"realNumberDNOClone/internal/api"
	"realNumberDNOClone/internal/boot"
	"realNumberDNOClone/internal/cache"
	"realNumberDNOClone/internal/models"
	"realNumberDNOClone/internal/querylog"
)

func main() {
	env := flag.String("env", "local", "environment")
	flag.Parse()

	app, err := boot.Init(*env)
	if err != nil {
		panic(err)
	}
	defer app.Close()

	app.Logger.Info("Starting query-service", "env", app.Cfg.Env)

	qlWriter := querylog.NewAsyncWriter(
		app.DB.Writer,
		app.Cfg.QueryLogFlushSize,
		time.Duration(app.Cfg.QueryLogFlushInterval)*time.Second,
		app.Logger,
	)

	var dnoCache *cache.TTLCache[*models.DNOQueryResponse]
	if app.Cfg.DNOCacheTTLSeconds > 0 {
		dnoCache = cache.New[*models.DNOQueryResponse](
			time.Duration(app.Cfg.DNOCacheTTLSeconds)*time.Second, 100000)
	}

	var analyticsCache *cache.TTLCache[*models.AnalyticsSummary]
	if app.Cfg.AnalyticsCacheTTLSeconds > 0 {
		analyticsCache = cache.New[*models.AnalyticsSummary](
			time.Duration(app.Cfg.AnalyticsCacheTTLSeconds)*time.Second, 100)
	}

	port := app.Cfg.Port
	if p := os.Getenv("QUERY_PORT"); p != "" {
		port = p
	}

	router := api.NewQueryRouter(app.DB, app.Cfg, qlWriter, dnoCache, analyticsCache, app.Logger)
	boot.Serve(app, port, router, func() { qlWriter.Stop() })
}
