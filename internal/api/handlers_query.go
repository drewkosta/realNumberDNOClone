package api

import (
	"encoding/json"
	"net/http"

	"realNumberDNOClone/internal/models"
)

// QueryNumber godoc
//
//	@Summary		Single DNO lookup
//	@Description	Check if a phone number is on the Do Not Originate list
//	@Tags			Query
//	@Accept			json
//	@Produce		json
//	@Param			phoneNumber	query		string	true	"10-digit phone number"	example(5551234567)
//	@Param			channel		query		string	false	"voice or text"			default(voice)
//	@Success		200			{object}	models.DNOQueryResponse
//	@Failure		400			{object}	map[string]string
//	@Failure		401			{object}	map[string]string
//	@Security		BearerAuth || APIKeyAuth
//	@Router			/dno/query [get]
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

// BulkQuery godoc
//
//	@Summary		Bulk DNO lookup
//	@Description	Check up to 1000 phone numbers against the DNO list in a single request
//	@Tags			Query
//	@Accept			json
//	@Produce		json
//	@Param			body	body		models.BulkQueryRequest	true	"Phone numbers and channel"
//	@Success		200		{object}	models.BulkQueryResponse
//	@Failure		400		{object}	map[string]string
//	@Failure		401		{object}	map[string]string
//	@Security		BearerAuth || APIKeyAuth
//	@Router			/dno/query/bulk [post]
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
