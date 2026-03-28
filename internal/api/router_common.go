package api

import (
	"context"
	"log/slog"
	"net/http"
	"time"

	chimw "github.com/go-chi/chi/v5/middleware"
	"github.com/go-chi/cors"

	"realNumberDNOClone/internal/config"
	"realNumberDNOClone/internal/db"
)

func corsMiddleware(cfg *config.Config) func(http.Handler) http.Handler {
	allowedOrigins := []string{"http://localhost:5173", "http://localhost:3000"}
	if cfg.CORSOrigin != "" {
		allowedOrigins = append(allowedOrigins, cfg.CORSOrigin)
	}
	if cfg.CORSOrigin == "*" {
		allowedOrigins = []string{"*"}
	}

	return cors.Handler(cors.Options{
		AllowedOrigins:   allowedOrigins,
		AllowedMethods:   []string{"GET", "POST", "PUT", "DELETE", "OPTIONS"},
		AllowedHeaders:   []string{"Accept", "Authorization", "Content-Type", "X-API-Key"},
		ExposedHeaders:   []string{"Content-Disposition"},
		AllowCredentials: cfg.CORSOrigin != "*",
		MaxAge:           300,
	})
}

// healthHandler returns liveness status -- "am I alive?"
func healthHandler(database *db.DB, cfg *config.Config, serviceName string) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		status := "ok"
		dbStatus := "ok"
		if err := database.Ping(r.Context()); err != nil {
			status = "degraded"
			dbStatus = err.Error()
		}
		writeJSON(w, http.StatusOK, map[string]interface{}{
			"status":  status,
			"env":     string(cfg.Env),
			"service": serviceName,
			"db":      dbStatus,
		})
	}
}

// readyHandler returns readiness status -- "am I ready to serve traffic?"
// Returns 503 if the DB is unreachable so load balancers stop sending traffic.
func readyHandler(database *db.DB, serviceName string) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		if err := database.Ping(r.Context()); err != nil {
			w.WriteHeader(http.StatusServiceUnavailable)
			writeJSON(w, http.StatusServiceUnavailable, map[string]interface{}{
				"ready":   false,
				"service": serviceName,
				"error":   err.Error(),
			})
			return
		}
		writeJSON(w, http.StatusOK, map[string]interface{}{
			"ready":   true,
			"service": serviceName,
		})
	}
}

func slogMiddleware(logger *slog.Logger) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			ww := chimw.NewWrapResponseWriter(w, r.ProtoMajor)
			start := time.Now()
			next.ServeHTTP(ww, r)
			logger.Info("request",
				"method", r.Method,
				"path", r.URL.Path,
				"status", ww.Status(),
				"bytes", ww.BytesWritten(),
				"duration_ms", time.Since(start).Milliseconds(),
				"request_id", chimw.GetReqID(r.Context()),
			)
		})
	}
}

// bodyLimitMiddleware restricts request body size.
func bodyLimitMiddleware(maxBytes int64) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			if r.Body != nil {
				r.Body = http.MaxBytesReader(w, r.Body, maxBytes)
			}
			next.ServeHTTP(w, r)
		})
	}
}

// timeoutMiddleware adds a per-request context deadline.
func timeoutMiddleware(d time.Duration) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			ctx, cancel := context.WithTimeout(r.Context(), d)
			defer cancel()
			next.ServeHTTP(w, r.WithContext(ctx))
		})
	}
}
