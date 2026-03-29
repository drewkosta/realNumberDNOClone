package api

import (
	"fmt"
	"log/slog"
	"net/http"
	"time"

	"github.com/go-chi/chi/v5"
	chimw "github.com/go-chi/chi/v5/middleware"
	"github.com/go-chi/httprate"
	"github.com/prometheus/client_golang/prometheus/promhttp"

	"realNumberDNOClone/internal/cache"
	"realNumberDNOClone/internal/config"
	"realNumberDNOClone/internal/db"
	"realNumberDNOClone/internal/metrics"
	"realNumberDNOClone/internal/models"
	"realNumberDNOClone/internal/querylog"
	"realNumberDNOClone/internal/service"
)

// NewQueryRouter creates a router for the query service (hot path).
// Handles: DNO single/bulk lookups via API key or JWT.
func NewQueryRouter(
	database *db.DB,
	cfg *config.Config,
	qlWriter *querylog.AsyncWriter,
	dnoCache *cache.TTLCache[*models.DNOQueryResponse],
	analyticsCache *cache.TTLCache[*models.AnalyticsSummary],
	logger *slog.Logger,
) http.Handler {
	r := chi.NewRouter()
	r.Use(chimw.RequestID, chimw.RealIP, chimw.Recoverer, metrics.Middleware, slogMiddleware(logger))
	r.Use(corsMiddleware(cfg))

	authService := service.NewAuthService(database, cfg.JWTSecret)
	apiKeyService := service.NewAPIKeyService(database)
	dnoService := service.NewDNOService(database, qlWriter, dnoCache, analyticsCache)
	h := NewHandlers(database.Writer, dnoService, authService, apiKeyService, nil)

	// 1MB body limit for query endpoints
	r.Use(bodyLimitMiddleware(1 << 20))

	r.Handle("/metrics", promhttp.Handler())
	r.Get("/health", healthHandler(database, cfg, "query-service"))
	r.Get("/ready", readyHandler(database, "query-service"))

	r.Group(func(r chi.Router) {
		r.Use(APIKeyMiddleware(apiKeyService, authService))

		if cfg.RateLimitRPS > 0 {
			r.Use(httprate.Limit(cfg.RateLimitRPS, time.Second,
				httprate.WithKeyFuncs(func(r *http.Request) (string, error) {
					if orgID, ok := r.Context().Value(OrgIDKey).(int64); ok {
						return fmt.Sprintf("org:%d", orgID), nil
					}
					return chimw.GetReqID(r.Context()), nil
				}),
			))
		}

		r.Get("/api/v1/dno/query", h.QueryNumber)
		r.Post("/api/v1/dno/query/bulk", h.BulkQuery)
	})

	return r
}
