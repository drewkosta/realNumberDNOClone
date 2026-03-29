package api

import (
	"encoding/json"
	"net/http"
	"strconv"

	"realNumberDNOClone/internal/models"
)

// ── Webhooks ────────────────────────────────────────────────────────────────

// CreateWebhook godoc
//
//	@Summary		Create webhook
//	@Description	Register a new webhook subscription
//	@Tags			Webhooks
//	@Accept			json
//	@Produce		json
//	@Param			body	body		object{url=string,secret=string,events=string}	true	"Webhook details"
//	@Success		201		{object}	models.WebhookSubscription
//	@Failure		400		{object}	map[string]string
//	@Failure		500		{object}	map[string]string
//	@Security		BearerAuth
//	@Router			/webhooks [post]
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

// ListWebhooks godoc
//
//	@Summary		List webhooks
//	@Description	Retrieve all webhook subscriptions for the current organization
//	@Tags			Webhooks
//	@Produce		json
//	@Success		200	{array}		models.WebhookSubscription
//	@Failure		500	{object}	map[string]string
//	@Security		BearerAuth
//	@Router			/webhooks [get]
func (h *Handlers) ListWebhooks(w http.ResponseWriter, r *http.Request) {
	orgID, _ := r.Context().Value(OrgIDKey).(int64)
	subs, err := h.features.ListWebhooks(r.Context(), orgID)
	if err != nil {
		writeError(w, http.StatusInternalServerError, err.Error())
		return
	}
	writeJSON(w, http.StatusOK, subs)
}

// DeleteWebhook godoc
//
//	@Summary		Delete webhook
//	@Description	Remove a webhook subscription
//	@Tags			Webhooks
//	@Produce		json
//	@Param			id	query		int	true	"Webhook ID"
//	@Success		200	{object}	map[string]string
//	@Failure		400	{object}	map[string]string
//	@Security		BearerAuth
//	@Router			/webhooks [delete]
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

// ValidateOwnership godoc
//
//	@Summary		Validate number ownership
//	@Description	Check whether the current organization owns a given phone number
//	@Tags			DNO Management
//	@Produce		json
//	@Param			phoneNumber	query		string	true	"Phone number to validate"
//	@Success		200			{object}	models.OwnershipValidation
//	@Failure		400			{object}	map[string]string
//	@Failure		500			{object}	map[string]string
//	@Security		BearerAuth
//	@Router			/dno/validate-ownership [get]
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

// AnalyzeTraffic godoc
//
//	@Summary		Analyze traffic
//	@Description	Analyze call traffic records against the DNO list
//	@Tags			Analyzer
//	@Accept			json
//	@Produce		json
//	@Param			body	body		models.DNOAnalyzerRequest	true	"Traffic records to analyze"
//	@Success		200		{object}	models.DNOAnalyzerReport
//	@Failure		400		{object}	map[string]string
//	@Failure		500		{object}	map[string]string
//	@Security		BearerAuth
//	@Router			/analyzer [post]
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

// ComplianceReport godoc
//
//	@Summary		Generate compliance report
//	@Description	Generate a compliance report for the current organization
//	@Tags			Compliance
//	@Produce		json
//	@Success		200	{object}	models.ComplianceReport
//	@Failure		500	{object}	map[string]string
//	@Security		BearerAuth
//	@Router			/compliance-report [get]
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

// CalculateROI godoc
//
//	@Summary		Calculate ROI
//	@Description	Calculate return on investment based on daily call volume
//	@Tags			Tools
//	@Produce		json
//	@Param			dailyCallVolume	query		int	true	"Daily call volume"
//	@Success		200				{object}	models.ROICalculation
//	@Failure		400				{object}	map[string]string
//	@Security		BearerAuth
//	@Router			/roi-calculator [get]
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
