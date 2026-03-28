package api

import (
	"database/sql"
	"net/http"

	"github.com/go-chi/chi/v5"
	chimw "github.com/go-chi/chi/v5/middleware"
	"github.com/go-chi/cors"

	"realNumberDNOClone/internal/config"
	"realNumberDNOClone/internal/service"
)

func NewRouter(db *sql.DB, cfg *config.Config) http.Handler {
	r := chi.NewRouter()

	r.Use(chimw.RequestID)
	r.Use(chimw.Logger)
	r.Use(chimw.Recoverer)
	r.Use(chimw.RealIP)

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
	dnoService := service.NewDNOService(db)
	h := NewHandlers(dnoService, authService)

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

	// Public routes
	r.Post("/api/auth/login", h.Login)

	// Protected routes
	r.Group(func(r chi.Router) {
		r.Use(AuthMiddleware(authService))

		r.Get("/api/auth/me", h.GetMe)

		r.Get("/api/dno/query", h.QueryNumber)
		r.Post("/api/dno/query/bulk", h.BulkQuery)

		r.Post("/api/dno/numbers", h.AddNumber)
		r.Delete("/api/dno/numbers", h.RemoveNumber)
		r.Get("/api/dno/numbers", h.ListNumbers)

		r.Post("/api/dno/bulk-upload", h.BulkUpload)
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
