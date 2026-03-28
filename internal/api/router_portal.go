package api

import (
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

// NewPortalRouter creates a router for the portal service.
// Handles: auth, number management, analytics, compliance, webhooks, admin.
func NewPortalRouter(
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
	featuresService := service.NewFeaturesService(database, logger)
	dnoService.SetWebhookFirer(featuresService)
	h := NewHandlers(database.Writer, dnoService, authService, apiKeyService, featuresService)

	// 10MB body limit (bulk uploads need more)
	r.Use(bodyLimitMiddleware(10 << 20))

	r.Handle("/metrics", promhttp.Handler())
	r.Get("/health", healthHandler(database, cfg, "portal-service"))
	r.Get("/ready", readyHandler(database, "portal-service"))

	// Login + refresh (rate limited)
	r.Group(func(r chi.Router) {
		r.Use(httprate.LimitByIP(5, time.Minute))
		r.Post("/api/auth/login", h.Login)
		r.Post("/api/auth/refresh", h.RefreshToken)
	})

	// JWT-protected routes
	r.Group(func(r chi.Router) {
		r.Use(AuthMiddleware(authService))

		r.Get("/api/auth/me", h.GetMe)

		// DNO management
		r.Post("/api/dno/numbers", h.AddNumber)
		r.Delete("/api/dno/numbers", h.RemoveNumber)
		r.Get("/api/dno/numbers", h.ListNumbers)
		r.Post("/api/dno/bulk-upload", h.BulkUpload)
		r.Get("/api/dno/bulk-job", h.GetBulkJobStatus)
		r.With(timeoutMiddleware(60*time.Second)).Get("/api/dno/export", h.ExportCSV)
		r.Get("/api/dno/validate-ownership", h.ValidateOwnership)

		// Analytics & audit
		r.Get("/api/analytics", h.GetAnalytics)
		r.Get("/api/audit-log", h.GetAuditLog)

		// Features
		r.Get("/api/compliance-report", h.ComplianceReport)
		r.Get("/api/roi-calculator", h.CalculateROI)
		r.With(timeoutMiddleware(30*time.Second)).Post("/api/analyzer", h.AnalyzeTraffic)

		// Webhooks
		r.Post("/api/webhooks", h.CreateWebhook)
		r.Get("/api/webhooks", h.ListWebhooks)
		r.Delete("/api/webhooks", h.DeleteWebhook)

		// Admin
		r.Group(func(r chi.Router) {
			r.Use(AdminOnly)
			r.Post("/api/admin/users", h.CreateUser)
			r.Post("/api/admin/reset-password", h.ResetPassword)
			r.Post("/api/admin/api-keys", h.GenerateAPIKey)
			r.Delete("/api/admin/api-keys", h.RevokeAPIKey)
			r.Post("/api/admin/itg-ingest", h.IngestITGNumber)
			r.Post("/api/admin/npac-event", h.NPACPortingEvent)
			r.Post("/api/admin/tss-sync", h.TSSRegistrySync)
		})
	})

	return r
}
