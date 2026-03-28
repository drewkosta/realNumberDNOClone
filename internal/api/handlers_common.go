package api

import (
	"database/sql"
	"encoding/json"
	"log"
	"net/http"

	"realNumberDNOClone/internal/service"
)

// Handlers is the shared handler struct for all API domains.
type Handlers struct {
	db       *sql.DB
	dno      *service.DNOService
	auth     *service.AuthService
	apiKeys  *service.APIKeyService
	features *service.FeaturesService
}

func NewHandlers(
	db *sql.DB,
	dno *service.DNOService,
	auth *service.AuthService,
	apiKeys *service.APIKeyService,
	features *service.FeaturesService,
) *Handlers {
	return &Handlers{db: db, dno: dno, auth: auth, apiKeys: apiKeys, features: features}
}

func writeJSON(w http.ResponseWriter, status int, data interface{}) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	if err := json.NewEncoder(w).Encode(data); err != nil {
		log.Printf("writeJSON encode error: %v", err)
	}
}

func writeError(w http.ResponseWriter, status int, msg string) {
	writeJSON(w, status, map[string]string{"error": msg})
}
