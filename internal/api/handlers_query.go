package api

import (
	"encoding/json"
	"net/http"

	"realNumberDNOClone/internal/models"
)

// QueryNumber handles single DNO lookups.
func (h *Handlers) QueryNumber(w http.ResponseWriter, r *http.Request) {
	phone := r.URL.Query().Get("phoneNumber")
	channel := r.URL.Query().Get("channel")
	if phone == "" {
		writeError(w, http.StatusBadRequest, "phoneNumber query parameter required")
		return
	}
	if channel == "" {
		channel = "voice"
	}

	var orgID *int64
	if id, ok := r.Context().Value(OrgIDKey).(int64); ok {
		orgID = &id
	}

	result, err := h.dno.QueryNumber(r.Context(), phone, channel, orgID)
	if err != nil {
		writeError(w, http.StatusBadRequest, err.Error())
		return
	}

	writeJSON(w, http.StatusOK, result)
}

// BulkQuery handles batch DNO lookups.
func (h *Handlers) BulkQuery(w http.ResponseWriter, r *http.Request) {
	var req models.BulkQueryRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeError(w, http.StatusBadRequest, "invalid request body")
		return
	}

	if len(req.PhoneNumbers) == 0 {
		writeError(w, http.StatusBadRequest, "phoneNumbers array required")
		return
	}
	if req.Channel == "" {
		req.Channel = "voice"
	}

	var orgID *int64
	if id, ok := r.Context().Value(OrgIDKey).(int64); ok {
		orgID = &id
	}

	result, err := h.dno.BulkQuery(r.Context(), req.PhoneNumbers, req.Channel, orgID)
	if err != nil {
		writeError(w, http.StatusBadRequest, err.Error())
		return
	}

	writeJSON(w, http.StatusOK, result)
}
