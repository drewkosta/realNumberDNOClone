package api

import (
	"encoding/csv"
	"encoding/json"
	"fmt"
	"log"
	"net/http"
	"strconv"
	"strings"

	"realNumberDNOClone/internal/models"
	"realNumberDNOClone/internal/service"
)

type Handlers struct {
	dnoService  *service.DNOService
	authService *service.AuthService
}

func NewHandlers(dnoService *service.DNOService, authService *service.AuthService) *Handlers {
	return &Handlers{dnoService: dnoService, authService: authService}
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

// Auth handlers

func (h *Handlers) Login(w http.ResponseWriter, r *http.Request) {
	var req models.LoginRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeError(w, http.StatusBadRequest, "invalid request body")
		return
	}

	resp, err := h.authService.Login(r.Context(), req.Email, req.Password)
	if err != nil {
		writeError(w, http.StatusUnauthorized, err.Error())
		return
	}

	writeJSON(w, http.StatusOK, resp)
}

func (h *Handlers) GetMe(w http.ResponseWriter, r *http.Request) {
	userID, _ := r.Context().Value(UserIDKey).(int64)
	user, err := h.authService.GetUser(r.Context(), userID)
	if err != nil {
		writeError(w, http.StatusNotFound, "user not found")
		return
	}
	writeJSON(w, http.StatusOK, user)
}

func (h *Handlers) CreateUser(w http.ResponseWriter, r *http.Request) {
	var req models.CreateUserRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeError(w, http.StatusBadRequest, "invalid request body")
		return
	}

	user, err := h.authService.CreateUser(r.Context(), req)
	if err != nil {
		writeError(w, http.StatusBadRequest, err.Error())
		return
	}

	writeJSON(w, http.StatusCreated, user)
}

// DNO Query handlers

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

	result, err := h.dnoService.QueryNumber(r.Context(), phone, channel, orgID)
	if err != nil {
		writeError(w, http.StatusBadRequest, err.Error())
		return
	}

	writeJSON(w, http.StatusOK, result)
}

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

	result, err := h.dnoService.BulkQuery(r.Context(), req.PhoneNumbers, req.Channel, orgID)
	if err != nil {
		writeError(w, http.StatusBadRequest, err.Error())
		return
	}

	writeJSON(w, http.StatusOK, result)
}

// DNO Management handlers

func (h *Handlers) AddNumber(w http.ResponseWriter, r *http.Request) {
	var req models.AddDNORequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeError(w, http.StatusBadRequest, "invalid request body")
		return
	}

	if req.PhoneNumber == "" {
		writeError(w, http.StatusBadRequest, "phoneNumber is required")
		return
	}

	userID, _ := r.Context().Value(UserIDKey).(int64)
	orgID, _ := r.Context().Value(OrgIDKey).(int64)

	number, err := h.dnoService.AddNumber(r.Context(), req, orgID, userID)
	if err != nil {
		writeError(w, http.StatusBadRequest, err.Error())
		return
	}

	writeJSON(w, http.StatusCreated, number)
}

func (h *Handlers) RemoveNumber(w http.ResponseWriter, r *http.Request) {
	phone := r.URL.Query().Get("phoneNumber")
	channel := r.URL.Query().Get("channel")
	if phone == "" {
		writeError(w, http.StatusBadRequest, "phoneNumber query parameter required")
		return
	}
	if channel == "" {
		channel = "voice"
	}

	userID, _ := r.Context().Value(UserIDKey).(int64)
	orgID, _ := r.Context().Value(OrgIDKey).(int64)

	if err := h.dnoService.RemoveNumber(r.Context(), phone, channel, orgID, userID); err != nil {
		writeError(w, http.StatusBadRequest, err.Error())
		return
	}

	writeJSON(w, http.StatusOK, map[string]string{"message": "number removed from DNO list"})
}

func (h *Handlers) ListNumbers(w http.ResponseWriter, r *http.Request) {
	q := r.URL.Query()
	page, _ := strconv.Atoi(q.Get("page"))
	pageSize, _ := strconv.Atoi(q.Get("pageSize"))
	dataset := q.Get("dataset")
	status := q.Get("status")
	channel := q.Get("channel")
	search := q.Get("search")

	var orgID *int64
	role, _ := r.Context().Value(RoleKey).(string)
	if role != "admin" {
		if id, ok := r.Context().Value(OrgIDKey).(int64); ok {
			orgID = &id
		}
	}

	result, err := h.dnoService.ListNumbers(r.Context(), orgID, dataset, status, channel, search, page, pageSize)
	if err != nil {
		writeError(w, http.StatusBadRequest, err.Error())
		return
	}

	writeJSON(w, http.StatusOK, result)
}

// Bulk upload via CSV

func (h *Handlers) BulkUpload(w http.ResponseWriter, r *http.Request) {
	if err := r.ParseMultipartForm(10 << 20); err != nil { // 10MB max
		writeError(w, http.StatusBadRequest, "file too large or invalid form data")
		return
	}

	file, _, err := r.FormFile("file")
	if err != nil {
		writeError(w, http.StatusBadRequest, "file field required")
		return
	}
	defer file.Close()

	channel := r.FormValue("channel")
	if channel == "" {
		channel = "voice"
	}
	numberType := r.FormValue("numberType")
	if numberType == "" {
		numberType = "local"
	}

	userID, _ := r.Context().Value(UserIDKey).(int64)
	orgID, _ := r.Context().Value(OrgIDKey).(int64)

	reader := csv.NewReader(file)
	records, err := reader.ReadAll()
	if err != nil {
		writeError(w, http.StatusBadRequest, "invalid CSV file")
		return
	}

	successCount := 0
	errorCount := 0
	var errors []string

	for i, record := range records {
		if len(record) == 0 {
			continue
		}
		phone := strings.TrimSpace(record[0])
		if phone == "" || phone == "phone_number" || phone == "phoneNumber" {
			continue // skip header
		}

		reason := ""
		if len(record) > 1 {
			reason = strings.TrimSpace(record[1])
		}

		req := models.AddDNORequest{
			PhoneNumber: phone,
			NumberType:  numberType,
			Channel:     channel,
			Reason:      reason,
		}

		_, err := h.dnoService.AddNumber(r.Context(), req, orgID, userID)
		if err != nil {
			errorCount++
			errors = append(errors, fmt.Sprintf("row %d (%s): %s", i+1, phone, err.Error()))
		} else {
			successCount++
		}
	}

	writeJSON(w, http.StatusOK, map[string]interface{}{
		"total":   successCount + errorCount,
		"success": successCount,
		"errors":  errorCount,
		"details": errors,
	})
}

// Export as CSV flat file (streaming)

func (h *Handlers) ExportCSV(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "text/csv")
	w.Header().Set("Content-Disposition", "attachment; filename=dno_export.csv")

	writer := csv.NewWriter(w)
	if err := writer.Write([]string{"phone_number", "last_update_date", "status_flag", "dataset", "channel", "number_type"}); err != nil {
		log.Printf("CSV header write error: %v", err)
		return
	}

	err := h.dnoService.StreamNumbers(r.Context(), func(n models.DNONumber) error {
		statusFlag := "0"
		if n.Dataset == "subscriber" {
			statusFlag = "1"
		}
		return writer.Write([]string{
			n.PhoneNumber,
			n.UpdatedAt.Format("2006-01-02T15:04:05Z"),
			statusFlag,
			n.Dataset,
			n.Channel,
			n.NumberType,
		})
	})
	if err != nil {
		log.Printf("CSV export error: %v", err)
	}

	writer.Flush()
	if err := writer.Error(); err != nil {
		log.Printf("CSV flush error: %v", err)
	}
}

// Analytics handlers

func (h *Handlers) GetAnalytics(w http.ResponseWriter, r *http.Request) {
	var orgID *int64
	role, _ := r.Context().Value(RoleKey).(string)
	if role != "admin" {
		if id, ok := r.Context().Value(OrgIDKey).(int64); ok {
			orgID = &id
		}
	}

	analytics, err := h.dnoService.GetAnalytics(r.Context(), orgID)
	if err != nil {
		writeError(w, http.StatusInternalServerError, err.Error())
		return
	}

	writeJSON(w, http.StatusOK, analytics)
}

func (h *Handlers) GetAuditLog(w http.ResponseWriter, r *http.Request) {
	page, _ := strconv.Atoi(r.URL.Query().Get("page"))
	pageSize, _ := strconv.Atoi(r.URL.Query().Get("pageSize"))

	var orgID *int64
	role, _ := r.Context().Value(RoleKey).(string)
	if role != "admin" {
		if id, ok := r.Context().Value(OrgIDKey).(int64); ok {
			orgID = &id
		}
	}

	result, err := h.dnoService.GetAuditLog(r.Context(), orgID, page, pageSize)
	if err != nil {
		writeError(w, http.StatusInternalServerError, err.Error())
		return
	}

	writeJSON(w, http.StatusOK, result)
}
