package api

import (
	"encoding/json"
	"net/http"
	"strconv"

	"realNumberDNOClone/internal/models"
)

// ── Webhooks ────────────────────────────────────────────────────────────────

func (h *Handlers) CreateWebhook(w http.ResponseWriter, r *http.Request) {
	var req struct {
		URL    string `json:"url"`
		Secret string `json:"secret"`
		Events string `json:"events"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeError(w, http.StatusBadRequest, "invalid request body")
		return
	}
	if req.URL == "" || req.Secret == "" {
		writeError(w, http.StatusBadRequest, "url and secret are required")
		return
	}

	orgID, _ := r.Context().Value(OrgIDKey).(int64)
	sub, err := h.features.CreateWebhook(r.Context(), orgID, req.URL, req.Secret, req.Events)
	if err != nil {
		writeError(w, http.StatusInternalServerError, err.Error())
		return
	}

	writeJSON(w, http.StatusCreated, sub)
}

func (h *Handlers) ListWebhooks(w http.ResponseWriter, r *http.Request) {
	orgID, _ := r.Context().Value(OrgIDKey).(int64)
	subs, err := h.features.ListWebhooks(r.Context(), orgID)
	if err != nil {
		writeError(w, http.StatusInternalServerError, err.Error())
		return
	}
	writeJSON(w, http.StatusOK, subs)
}

func (h *Handlers) DeleteWebhook(w http.ResponseWriter, r *http.Request) {
	id, err := strconv.ParseInt(r.URL.Query().Get("id"), 10, 64)
	if err != nil {
		writeError(w, http.StatusBadRequest, "id query parameter required")
		return
	}
	orgID, _ := r.Context().Value(OrgIDKey).(int64)

	if err := h.features.DeleteWebhook(r.Context(), id, orgID); err != nil {
		writeError(w, http.StatusBadRequest, err.Error())
		return
	}
	writeJSON(w, http.StatusOK, map[string]string{"message": "webhook deleted"})
}

// ── Number Ownership Validation ─────────────────────────────────────────────

func (h *Handlers) ValidateOwnership(w http.ResponseWriter, r *http.Request) {
	phone := r.URL.Query().Get("phoneNumber")
	if phone == "" {
		writeError(w, http.StatusBadRequest, "phoneNumber required")
		return
	}
	orgID, _ := r.Context().Value(OrgIDKey).(int64)

	valid, reason, err := h.features.ValidateOwnership(r.Context(), phone, orgID)
	if err != nil {
		writeError(w, http.StatusInternalServerError, err.Error())
		return
	}

	writeJSON(w, http.StatusOK, map[string]interface{}{
		"phoneNumber": phone,
		"valid":       valid,
		"reason":      reason,
	})
}

// ── DNO Analyzer ────────────────────────────────────────────────────────────

func (h *Handlers) AnalyzeTraffic(w http.ResponseWriter, r *http.Request) {
	var req models.DNOAnalyzerRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeError(w, http.StatusBadRequest, "invalid request body")
		return
	}
	if len(req.Records) == 0 {
		writeError(w, http.StatusBadRequest, "records array required")
		return
	}
	if len(req.Records) > 100000 {
		writeError(w, http.StatusBadRequest, "maximum 100,000 records per analysis")
		return
	}

	report, err := h.features.AnalyzeTraffic(r.Context(), req)
	if err != nil {
		writeError(w, http.StatusInternalServerError, err.Error())
		return
	}

	writeJSON(w, http.StatusOK, report)
}

// ── Compliance Report ───────────────────────────────────────────────────────

func (h *Handlers) ComplianceReport(w http.ResponseWriter, r *http.Request) {
	var orgID *int64
	role, _ := r.Context().Value(RoleKey).(string)
	if role != "admin" {
		if id, ok := r.Context().Value(OrgIDKey).(int64); ok {
			orgID = &id
		}
	}

	report, err := h.features.GenerateComplianceReport(r.Context(), orgID)
	if err != nil {
		writeError(w, http.StatusInternalServerError, err.Error())
		return
	}

	writeJSON(w, http.StatusOK, report)
}

// ── ROI Calculator ──────────────────────────────────────────────────────────

func (h *Handlers) CalculateROI(w http.ResponseWriter, r *http.Request) {
	volStr := r.URL.Query().Get("dailyCallVolume")
	if volStr == "" {
		writeError(w, http.StatusBadRequest, "dailyCallVolume query parameter required")
		return
	}
	volume, err := strconv.Atoi(volStr)
	if err != nil || volume <= 0 {
		writeError(w, http.StatusBadRequest, "dailyCallVolume must be a positive integer")
		return
	}

	result := h.features.CalculateROI(r.Context(), volume)
	writeJSON(w, http.StatusOK, result)
}
