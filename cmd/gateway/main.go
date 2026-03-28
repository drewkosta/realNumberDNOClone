package main

import (
	"context"
	"log/slog"
	"net/http"
	"net/http/httputil"
	"net/url"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/go-chi/chi/v5"
	chimw "github.com/go-chi/chi/v5/middleware"
	"github.com/go-chi/cors"
	"github.com/prometheus/client_golang/prometheus/promhttp"

	"realNumberDNOClone/internal/metrics"
)

func main() {
	logger := slog.New(slog.NewJSONHandler(os.Stdout, &slog.HandlerOptions{Level: slog.LevelInfo}))
	slog.SetDefault(logger)

	gatewayPort := envOr("GATEWAY_PORT", "8080")
	queryURL := envOr("QUERY_SERVICE_URL", "http://localhost:8081")
	portalURL := envOr("PORTAL_SERVICE_URL", "http://localhost:8082")

	queryTarget, _ := url.Parse(queryURL)
	portalTarget, _ := url.Parse(portalURL)

	queryProxy := httputil.NewSingleHostReverseProxy(queryTarget)
	portalProxy := httputil.NewSingleHostReverseProxy(portalTarget)

	r := chi.NewRouter()
	r.Use(chimw.RequestID, chimw.RealIP, chimw.Recoverer, metrics.Middleware)
	r.Use(cors.Handler(cors.Options{
		AllowedOrigins:   []string{"http://localhost:5173", "http://localhost:3000"},
		AllowedMethods:   []string{"GET", "POST", "PUT", "DELETE", "OPTIONS"},
		AllowedHeaders:   []string{"Accept", "Authorization", "Content-Type", "X-API-Key"},
		ExposedHeaders:   []string{"Content-Disposition"},
		AllowCredentials: true,
		MaxAge:           300,
	}))

	r.Handle("/metrics", promhttp.Handler())
	r.Get("/health", func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.Write([]byte(`{"status":"ok","service":"gateway"}`))
	})

	// Route query endpoints to query-service
	r.Get("/api/dno/query", queryProxy.ServeHTTP)
	r.Post("/api/dno/query/bulk", queryProxy.ServeHTTP)

	// Route everything else to portal-service
	r.HandleFunc("/api/*", portalProxy.ServeHTTP)

	logger.Info("Gateway starting", "port", gatewayPort, "query", queryURL, "portal", portalURL)

	srv := &http.Server{
		Addr:              ":" + gatewayPort,
		Handler:           r,
		ReadHeaderTimeout: 5 * time.Second,
		ReadTimeout:       10 * time.Second,
		WriteTimeout:      30 * time.Second,
		IdleTimeout:       120 * time.Second,
	}

	ctx, stop := signal.NotifyContext(context.Background(), syscall.SIGINT, syscall.SIGTERM)
	defer stop()

	go func() {
		if err := srv.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			logger.Error("Gateway failed", "error", err)
			os.Exit(1)
		}
	}()

	<-ctx.Done()
	logger.Info("Gateway shutting down...")
	shutdownCtx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	srv.Shutdown(shutdownCtx)
	logger.Info("Gateway stopped")
}

func envOr(key, fallback string) string {
	if v := os.Getenv(key); v != "" {
		return v
	}
	return fallback
}
