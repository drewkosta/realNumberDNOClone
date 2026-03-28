package api

import (
	"encoding/json"
	"net/http"
	"strconv"

	"realNumberDNOClone/internal/models"
)

// ── User Management ─────────────────────────────────────────────────────────

func (h *Handlers) CreateUser(w http.ResponseWriter, r *http.Request) {
	var req models.CreateUserRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeError(w, http.StatusBadRequest, "invalid request body")
		return
	}

	user, err := h.auth.CreateUser(r.Context(), req)
	if err != nil {
		writeError(w, http.StatusBadRequest, err.Error())
		return
	}

	writeJSON(w, http.StatusCreated, user)
}

// ── API Key Management ──────────────────────────────────────────────────────

func (h *Handlers) GenerateAPIKey(w http.ResponseWriter, r *http.Request) {
	orgID, err := strconv.ParseInt(r.URL.Query().Get("orgId"), 10, 64)
	if err != nil {
		writeError(w, http.StatusBadRequest, "orgId query parameter required")
		return
	}

	key, err := h.apiKeys.GenerateKey(r.Context(), orgID)
	if err != nil {
		writeError(w, http.StatusInternalServerError, err.Error())
		return
	}

	writeJSON(w, http.StatusCreated, map[string]interface{}{
		"orgId":  orgID,
		"apiKey": key,
		"note":   "Store this key securely. It cannot be retrieved again.",
	})
}

func (h *Handlers) RevokeAPIKey(w http.ResponseWriter, r *http.Request) {
	orgID, err := strconv.ParseInt(r.URL.Query().Get("orgId"), 10, 64)
	if err != nil {
		writeError(w, http.StatusBadRequest, "orgId query parameter required")
		return
	}

	if err := h.apiKeys.RevokeKey(r.Context(), orgID); err != nil {
		writeError(w, http.StatusInternalServerError, err.Error())
		return
	}

	writeJSON(w, http.StatusOK, map[string]string{"message": "API key revoked"})
}

// ── ITG Traceback Ingest ────────────────────────────────────────────────────

func (h *Handlers) IngestITGNumber(w http.ResponseWriter, r *http.Request) {
	var req models.ITGIngestRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeError(w, http.StatusBadRequest, "invalid request body")
		return
	}
	if req.PhoneNumber == "" || req.InvestigationID == "" || req.ThreatCategory == "" {
		writeError(w, http.StatusBadRequest, "phoneNumber, investigationId, and threatCategory are required")
		return
	}

	number, err := h.features.IngestITGNumber(r.Context(), req)
	if err != nil {
		writeError(w, http.StatusBadRequest, err.Error())
		return
	}

	writeJSON(w, http.StatusCreated, number)
}

// ── Mock NPAC Porting Event ─────────────────────────────────────────────────

func (h *Handlers) NPACPortingEvent(w http.ResponseWriter, r *http.Request) {
	var req struct {
		PhoneNumber   string `json:"phoneNumber"`
		NewStatus     string `json:"newStatus"`
		NewOwnerOrgID *int64 `json:"newOwnerOrgId,omitempty"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeError(w, http.StatusBadRequest, "invalid request body")
		return
	}
	if req.PhoneNumber == "" || req.NewStatus == "" {
		writeError(w, http.StatusBadRequest, "phoneNumber and newStatus required")
		return
	}

	if err := h.features.ProcessNPACPortingEvent(r.Context(), req.PhoneNumber, req.NewStatus, req.NewOwnerOrgID); err != nil {
		writeError(w, http.StatusBadRequest, err.Error())
		return
	}

	writeJSON(w, http.StatusOK, map[string]string{"message": "porting event processed"})
}

// ── Mock TSS Registry Sync ──────────────────────────────────────────────────

func (h *Handlers) TSSRegistrySync(w http.ResponseWriter, r *http.Request) {
	count, err := h.features.SyncTSSRegistry(r.Context())
	if err != nil {
		writeError(w, http.StatusInternalServerError, err.Error())
		return
	}

	writeJSON(w, http.StatusOK, map[string]interface{}{
		"message":      "TSS Registry sync complete",
		"numbersAdded": count,
	})
}
