package api

import (
	"database/sql"
	"encoding/csv"
	"encoding/json"
	"fmt"
	"log"
	"net/http"
	"strconv"
	"strings"

	"realNumberDNOClone/internal/jobs"
	"realNumberDNOClone/internal/models"
)

// ── Auth ─────────────────────────────────────────────────────────────────────

func (h *Handlers) Login(w http.ResponseWriter, r *http.Request) {
	var req models.LoginRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeError(w, http.StatusBadRequest, "invalid request body")
		return
	}

	resp, err := h.auth.Login(r.Context(), req.Email, req.Password)
	if err != nil {
		writeError(w, http.StatusUnauthorized, err.Error())
		return
	}

	writeJSON(w, http.StatusOK, resp)
}

func (h *Handlers) GetMe(w http.ResponseWriter, r *http.Request) {
	userID, _ := r.Context().Value(UserIDKey).(int64)
	user, err := h.auth.GetUser(r.Context(), userID)
	if err != nil {
		writeError(w, http.StatusNotFound, "user not found")
		return
	}
	writeJSON(w, http.StatusOK, user)
}

// ── DNO Number Management ───────────────────────────────────────────────────

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

	number, err := h.dno.AddNumber(r.Context(), req, orgID, userID)
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

	if err := h.dno.RemoveNumber(r.Context(), phone, channel, orgID, userID); err != nil {
		writeError(w, http.StatusBadRequest, err.Error())
		return
	}

	writeJSON(w, http.StatusOK, map[string]string{"message": "number removed from DNO list"})
}

func (h *Handlers) ListNumbers(w http.ResponseWriter, r *http.Request) {
	q := r.URL.Query()
	page, _ := strconv.Atoi(q.Get("page"))
	pageSize, _ := strconv.Atoi(q.Get("pageSize"))

	var orgID *int64
	role, _ := r.Context().Value(RoleKey).(string)
	if role != "admin" {
		if id, ok := r.Context().Value(OrgIDKey).(int64); ok {
			orgID = &id
		}
	}

	result, err := h.dno.ListNumbers(r.Context(), orgID, q.Get("dataset"), q.Get("status"), q.Get("channel"), q.Get("search"), page, pageSize)
	if err != nil {
		writeError(w, http.StatusBadRequest, err.Error())
		return
	}

	writeJSON(w, http.StatusOK, result)
}

// ── Bulk Upload & Export ────────────────────────────────────────────────────

func (h *Handlers) BulkUpload(w http.ResponseWriter, r *http.Request) {
	if err := r.ParseMultipartForm(10 << 20); err != nil {
		writeError(w, http.StatusBadRequest, "file too large or invalid form data")
		return
	}

	file, header, err := r.FormFile("file")
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
	csvRecords, err := reader.ReadAll()
	if err != nil {
		writeError(w, http.StatusBadRequest, "invalid CSV file")
		return
	}

	var records []jobs.BulkRecord
	for _, record := range csvRecords {
		if len(record) == 0 {
			continue
		}
		phone := strings.TrimSpace(record[0])
		if phone == "" || phone == "phone_number" || phone == "phoneNumber" {
			continue
		}
		reason := ""
		if len(record) > 1 {
			reason = strings.TrimSpace(record[1])
		}
		records = append(records, jobs.BulkRecord{PhoneNumber: phone, Reason: reason})
	}

	if len(records) == 0 {
		writeError(w, http.StatusBadRequest, "no valid records found in CSV")
		return
	}

	fileName := ""
	if header != nil {
		fileName = header.Filename
	}

	jobID, err := jobs.EnqueueBulkAdd(r.Context(), h.db, orgID, userID, records, channel, numberType, fileName)
	if err != nil {
		writeError(w, http.StatusInternalServerError, fmt.Sprintf("failed to enqueue job: %v", err))
		return
	}

	writeJSON(w, http.StatusAccepted, map[string]interface{}{
		"jobId":        jobID,
		"status":       "pending",
		"totalRecords": len(records),
		"message":      "Bulk upload queued for background processing",
	})
}

func (h *Handlers) GetBulkJobStatus(w http.ResponseWriter, r *http.Request) {
	jobID, err := strconv.ParseInt(r.URL.Query().Get("jobId"), 10, 64)
	if err != nil {
		writeError(w, http.StatusBadRequest, "jobId query parameter required")
		return
	}

	var job models.BulkJob
	var fileName, resultSummary sql.NullString
	var completedAt sql.NullTime
	err = h.db.QueryRowContext(r.Context(),
		`SELECT id, org_id, user_id, job_type, status, total_records, processed_records, success_count, error_count, file_name, result_summary, created_at, completed_at
		 FROM bulk_jobs WHERE id = ?`, jobID,
	).Scan(&job.ID, &job.OrgID, &job.UserID, &job.JobType, &job.Status, &job.TotalRecords, &job.ProcessedRecords,
		&job.SuccessCount, &job.ErrorCount, &fileName, &resultSummary, &job.CreatedAt, &completedAt)
	if err != nil {
		writeError(w, http.StatusNotFound, "job not found")
		return
	}
	if fileName.Valid {
		job.FileName = &fileName.String
	}
	if resultSummary.Valid {
		job.ResultSummary = &resultSummary.String
	}
	if completedAt.Valid {
		job.CompletedAt = &completedAt.Time
	}

	orgID, _ := r.Context().Value(OrgIDKey).(int64)
	role, _ := r.Context().Value(RoleKey).(string)
	if role != "admin" && job.OrgID != orgID {
		writeError(w, http.StatusForbidden, "not authorized to view this job")
		return
	}

	writeJSON(w, http.StatusOK, job)
}

func (h *Handlers) ExportCSV(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "text/csv")
	w.Header().Set("Content-Disposition", "attachment; filename=dno_export.csv")

	writer := csv.NewWriter(w)
	if err := writer.Write([]string{"phone_number", "last_update_date", "status_flag", "dataset", "channel", "number_type"}); err != nil {
		log.Printf("CSV header write error: %v", err)
		return
	}

	err := h.dno.StreamNumbers(r.Context(), func(n models.DNONumber) error {
		statusFlag := "0"
		if n.Dataset == "subscriber" {
			statusFlag = "1"
		}
		return writer.Write([]string{
			n.PhoneNumber, n.UpdatedAt.Format("2006-01-02T15:04:05Z"),
			statusFlag, n.Dataset, n.Channel, n.NumberType,
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

// ── Analytics & Audit ───────────────────────────────────────────────────────

func (h *Handlers) GetAnalytics(w http.ResponseWriter, r *http.Request) {
	var orgID *int64
	role, _ := r.Context().Value(RoleKey).(string)
	if role != "admin" {
		if id, ok := r.Context().Value(OrgIDKey).(int64); ok {
			orgID = &id
		}
	}

	analytics, err := h.dno.GetAnalytics(r.Context(), orgID)
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

	result, err := h.dno.GetAuditLog(r.Context(), orgID, page, pageSize)
	if err != nil {
		writeError(w, http.StatusInternalServerError, err.Error())
		return
	}
	writeJSON(w, http.StatusOK, result)
}
