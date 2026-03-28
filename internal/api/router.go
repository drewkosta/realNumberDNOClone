package api

import (
	"database/sql"
	"fmt"
	"log/slog"
	"net/http"
	"time"

	"github.com/go-chi/chi/v5"
	chimw "github.com/go-chi/chi/v5/middleware"
	"github.com/go-chi/cors"
	"github.com/go-chi/httprate"

	"realNumberDNOClone/internal/cache"
	"realNumberDNOClone/internal/config"
	"realNumberDNOClone/internal/models"
	"realNumberDNOClone/internal/querylog"
	"realNumberDNOClone/internal/service"
)

func NewRouter(
	db *sql.DB,
	cfg *config.Config,
	qlWriter *querylog.AsyncWriter,
	dnoCache *cache.TTLCache[*models.DNOQueryResponse],
	analyticsCache *cache.TTLCache[*models.AnalyticsSummary],
	logger *slog.Logger,
) http.Handler {
	r := chi.NewRouter()

	r.Use(chimw.RequestID)
	r.Use(chimw.RealIP)
	r.Use(chimw.Recoverer)
	r.Use(slogMiddleware(logger))

	allowedOrigins := []string{"http://localhost:5173", "http://localhost:3000"}
	if cfg.CORSOrigin != "" {
		allowedOrigins = append(allowedOrigins, cfg.CORSOrigin)
	}
	if cfg.CORSOrigin == "*" {
		allowedOrigins = []string{"*"}
	}

	r.Use(cors.Handler(cors.Options{
		AllowedOrigins:   allowedOrigins,
		AllowedMethods:   []string{"GET", "POST", "PUT", "DELETE", "OPTIONS"},
		AllowedHeaders:   []string{"Accept", "Authorization", "Content-Type", "X-API-Key"},
		ExposedHeaders:   []string{"Content-Disposition"},
		AllowCredentials: cfg.CORSOrigin != "*",
		MaxAge:           300,
	}))

	authService := service.NewAuthService(db, cfg.JWTSecret)
	dnoService := service.NewDNOService(db, qlWriter, dnoCache, analyticsCache)
	h := NewHandlers(db, dnoService, authService)

	// Health check with DB ping
	r.Get("/health", func(w http.ResponseWriter, r *http.Request) {
		status := "ok"
		dbStatus := "ok"
		if err := db.PingContext(r.Context()); err != nil {
			status = "degraded"
			dbStatus = err.Error()
		}
		writeJSON(w, http.StatusOK, map[string]interface{}{
			"status": status,
			"env":    string(cfg.Env),
			"db":     dbStatus,
		})
	})

	// Rate limit login endpoint
	r.Group(func(r chi.Router) {
		r.Use(httprate.LimitByIP(5, time.Minute))
		r.Post("/api/auth/login", h.Login)
	})

	// Protected routes
	r.Group(func(r chi.Router) {
		r.Use(AuthMiddleware(authService))

		// Rate limit API endpoints if configured
		if cfg.RateLimitRPS > 0 {
			r.Use(httprate.Limit(
				cfg.RateLimitRPS,
				time.Second,
				httprate.WithKeyFuncs(func(r *http.Request) (string, error) {
					// Rate limit by org ID from JWT context
					if orgID, ok := r.Context().Value(OrgIDKey).(int64); ok {
						return fmt.Sprintf("org:%d", orgID), nil
					}
					return chimw.GetReqID(r.Context()), nil
				}),
			))
		}

		r.Get("/api/auth/me", h.GetMe)

		r.Get("/api/dno/query", h.QueryNumber)
		r.Post("/api/dno/query/bulk", h.BulkQuery)

		r.Post("/api/dno/numbers", h.AddNumber)
		r.Delete("/api/dno/numbers", h.RemoveNumber)
		r.Get("/api/dno/numbers", h.ListNumbers)

		r.Post("/api/dno/bulk-upload", h.BulkUpload)
		r.Get("/api/dno/bulk-job", h.GetBulkJobStatus)
		r.Get("/api/dno/export", h.ExportCSV)

		r.Get("/api/analytics", h.GetAnalytics)
		r.Get("/api/audit-log", h.GetAuditLog)

		r.Group(func(r chi.Router) {
			r.Use(AdminOnly)
			r.Post("/api/admin/users", h.CreateUser)
		})
	})

	return r
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
