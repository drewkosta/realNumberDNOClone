package api

import (
	"encoding/json"
	"net/http"
	"strconv"

	"realNumberDNOClone/internal/models"
	"realNumberDNOClone/internal/service"
)

type FeaturesHandlers struct {
	features *service.FeaturesService
}

func NewFeaturesHandlers(features *service.FeaturesService) *FeaturesHandlers {
	return &FeaturesHandlers{features: features}
}

// ── ITG Traceback Ingest (admin only) ───────────────────────────────────────

func (h *FeaturesHandlers) IngestITGNumber(w http.ResponseWriter, r *http.Request) {
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

// ── Number Ownership Validation ─────────────────────────────────────────────

func (h *FeaturesHandlers) ValidateOwnership(w http.ResponseWriter, r *http.Request) {
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

// ── Webhooks ────────────────────────────────────────────────────────────────

func (h *FeaturesHandlers) CreateWebhook(w http.ResponseWriter, r *http.Request) {
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

func (h *FeaturesHandlers) ListWebhooks(w http.ResponseWriter, r *http.Request) {
	orgID, _ := r.Context().Value(OrgIDKey).(int64)
	subs, err := h.features.ListWebhooks(r.Context(), orgID)
	if err != nil {
		writeError(w, http.StatusInternalServerError, err.Error())
		return
	}
	writeJSON(w, http.StatusOK, subs)
}

func (h *FeaturesHandlers) DeleteWebhook(w http.ResponseWriter, r *http.Request) {
	idStr := r.URL.Query().Get("id")
	id, err := strconv.ParseInt(idStr, 10, 64)
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

// ── DNO Analyzer ────────────────────────────────────────────────────────────

func (h *FeaturesHandlers) AnalyzeTraffic(w http.ResponseWriter, r *http.Request) {
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

func (h *FeaturesHandlers) ComplianceReport(w http.ResponseWriter, r *http.Request) {
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

// ── ROI Calculator (public-ish, requires auth) ──────────────────────────────

func (h *FeaturesHandlers) CalculateROI(w http.ResponseWriter, r *http.Request) {
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

// ── Mock NPAC Porting Event (admin only) ────────────────────────────────────

func (h *FeaturesHandlers) NPACPortingEvent(w http.ResponseWriter, r *http.Request) {
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

// ── Mock TSS Registry Sync (admin only) ─────────────────────────────────────

func (h *FeaturesHandlers) TSSRegistrySync(w http.ResponseWriter, r *http.Request) {
	count, err := h.features.SyncTSSRegistry(r.Context())
	if err != nil {
		writeError(w, http.StatusInternalServerError, err.Error())
		return
	}

	writeJSON(w, http.StatusOK, map[string]interface{}{
		"message":     "TSS Registry sync complete",
		"numbersAdded": count,
	})
}
