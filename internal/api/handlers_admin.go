package api

import (
	"encoding/json"
	"net/http"
	"strconv"

	"realNumberDNOClone/internal/models"
)

// ── User Management ─────────────────────────────────────────────────────────

// CreateUser godoc
//
//	@Summary		Create a user
//	@Description	Create a new user account (admin only)
//	@Tags			Admin
//	@Accept			json
//	@Produce		json
//	@Param			body	body		models.CreateUserRequest	true	"User details"
//	@Success		201		{object}	models.User
//	@Failure		400		{object}	map[string]string
//	@Security		BearerAuth
//	@Router			/admin/users [post]
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

// ── Password Reset (admin resets another user's password) ────────────────────

// ResetPassword godoc
//
//	@Summary		Reset user password
//	@Description	Admin resets another user's password
//	@Tags			Admin
//	@Accept			json
//	@Produce		json
//	@Param			body	body		object{userId=int,newPassword=string}	true	"User ID and new password"
//	@Success		200		{object}	map[string]string
//	@Failure		400		{object}	map[string]string
//	@Security		BearerAuth
//	@Router			/admin/reset-password [post]
func (h *Handlers) ResetPassword(w http.ResponseWriter, r *http.Request) {
	var req struct {
		UserID      int64  `json:"userId"`
		NewPassword string `json:"newPassword"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeError(w, http.StatusBadRequest, "invalid request body")
		return
	}
	if req.UserID == 0 || req.NewPassword == "" {
		writeError(w, http.StatusBadRequest, "userId and newPassword are required")
		return
	}

	if err := h.auth.ResetPassword(r.Context(), req.UserID, req.NewPassword); err != nil {
		writeError(w, http.StatusBadRequest, err.Error())
		return
	}

	writeJSON(w, http.StatusOK, map[string]string{"message": "password reset successfully"})
}

// ── API Key Management ──────────────────────────────────────────────────────

// GenerateAPIKey godoc
//
//	@Summary		Generate API key
//	@Description	Generate a new API key for an organization
//	@Tags			Admin
//	@Produce		json
//	@Param			orgId	query		int	true	"Organization ID"
//	@Success		201		{object}	object{orgId=int,apiKey=string,note=string}
//	@Failure		400		{object}	map[string]string
//	@Failure		500		{object}	map[string]string
//	@Security		BearerAuth
//	@Router			/admin/api-keys [post]
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

// RevokeAPIKey godoc
//
//	@Summary		Revoke API key
//	@Description	Revoke an organization's API key
//	@Tags			Admin
//	@Produce		json
//	@Param			orgId	query		int	true	"Organization ID"
//	@Success		200		{object}	map[string]string
//	@Failure		400		{object}	map[string]string
//	@Failure		500		{object}	map[string]string
//	@Security		BearerAuth
//	@Router			/admin/api-keys [delete]
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

// IngestITGNumber godoc
//
//	@Summary		Ingest ITG number
//	@Description	Ingest a phone number from an ITG traceback investigation
//	@Tags			Admin
//	@Accept			json
//	@Produce		json
//	@Param			body	body		models.ITGIngestRequest	true	"ITG ingest details"
//	@Success		201		{object}	models.DNONumber
//	@Failure		400		{object}	map[string]string
//	@Security		BearerAuth
//	@Router			/admin/itg-ingest [post]
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

// NPACPortingEvent godoc
//
//	@Summary		NPAC porting event
//	@Description	Process a mock NPAC number porting event
//	@Tags			Admin
//	@Accept			json
//	@Produce		json
//	@Param			body	body		object{phoneNumber=string,newStatus=string,newOwnerOrgId=int}	true	"Porting event details"
//	@Success		200		{object}	map[string]string
//	@Failure		400		{object}	map[string]string
//	@Security		BearerAuth
//	@Router			/admin/npac-event [post]
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

// TSSRegistrySync godoc
//
//	@Summary		TSS registry sync
//	@Description	Trigger a mock TSS registry synchronization
//	@Tags			Admin
//	@Produce		json
//	@Success		200	{object}	object{message=string,numbersAdded=int}
//	@Failure		500	{object}	map[string]string
//	@Security		BearerAuth
//	@Router			/admin/tss-sync [post]
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
