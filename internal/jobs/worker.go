package jobs

import (
	"context"
	"database/sql"
	"encoding/json"
	"fmt"
	"log/slog"
	"time"

	appdb "realNumberDNOClone/internal/db"
	"realNumberDNOClone/internal/models"
)

// AddNumberFunc is the function signature for adding a DNO number.
// This avoids a circular dependency on the service package.
type AddNumberFunc func(ctx context.Context, req models.AddDNORequest, orgID, userID int64) (*models.DNONumber, error)

type Worker struct {
	db       *sql.DB
	appDB    *appdb.DB
	addFn    AddNumberFunc
	logger   *slog.Logger
	done     chan struct{}
	interval time.Duration
}

func NewWorker(database *appdb.DB, addFn AddNumberFunc, logger *slog.Logger) *Worker {
	return &Worker{
		db:       database.Writer,
		appDB:    database,
		addFn:    addFn,
		logger:   logger,
		done:     make(chan struct{}),
		interval: 2 * time.Second,
	}
}

func (w *Worker) Start() {
	go w.loop()
}

func (w *Worker) Stop() {
	close(w.done)
}

func (w *Worker) loop() {
	jobTicker := time.NewTicker(w.interval)
	retryTicker := time.NewTicker(30 * time.Second)
	cleanupTicker := time.NewTicker(1 * time.Hour)
	defer jobTicker.Stop()
	defer retryTicker.Stop()
	defer cleanupTicker.Stop()
	for {
		select {
		case <-jobTicker.C:
			w.processPending()
		case <-retryTicker.C:
			w.retryFailed()
		case <-cleanupTicker.C:
			w.cleanupQueryLog()
			w.ensurePartitions()
		case <-w.done:
			return
		}
	}
}

const maxRetries = 3

type bulkJobPayload struct {
	Records    []bulkRecord `json:"records"`
	Channel    string       `json:"channel"`
	NumberType string       `json:"numberType"`
}

type bulkRecord struct {
	PhoneNumber string `json:"phoneNumber"`
	Reason      string `json:"reason"`
}

func (w *Worker) processPending() {
	row := w.db.QueryRow(
		`SELECT id, org_id, user_id, result_summary FROM bulk_jobs WHERE status = 'pending' ORDER BY created_at LIMIT 1`)

	var jobID, orgID, userID int64
	var payloadJSON sql.NullString
	if err := row.Scan(&jobID, &orgID, &userID, &payloadJSON); err != nil {
		if err == sql.ErrNoRows {
			return
		}
		w.logger.Error("worker: scan pending job", "error", err)
		return
	}

	w.logger.Info("worker: processing bulk job", "jobID", jobID)

	// Mark as processing
	w.db.Exec(`UPDATE bulk_jobs SET status = 'processing' WHERE id = ?`, jobID)

	if !payloadJSON.Valid {
		w.db.Exec(`UPDATE bulk_jobs SET status = 'failed', result_summary = 'no payload', completed_at = CURRENT_TIMESTAMP WHERE id = ?`, jobID)
		return
	}

	var payload bulkJobPayload
	if err := json.Unmarshal([]byte(payloadJSON.String), &payload); err != nil {
		w.db.Exec(`UPDATE bulk_jobs SET status = 'failed', result_summary = ?, completed_at = CURRENT_TIMESTAMP WHERE id = ?`,
			fmt.Sprintf("invalid payload: %v", err), jobID)
		return
	}

	ctx := context.Background()
	total := len(payload.Records)
	successCount := 0
	errorCount := 0
	var errors []string

	for i, rec := range payload.Records {
		req := models.AddDNORequest{
			PhoneNumber: rec.PhoneNumber,
			NumberType:  payload.NumberType,
			Channel:     payload.Channel,
			Reason:      rec.Reason,
		}

		_, err := w.addFn(ctx, req, orgID, userID)
		if err != nil {
			errorCount++
			errors = append(errors, fmt.Sprintf("row %d (%s): %s", i+1, rec.PhoneNumber, err.Error()))
		} else {
			successCount++
		}

		// Update progress every 100 records
		if (i+1)%100 == 0 || i == total-1 {
			w.db.Exec(`UPDATE bulk_jobs SET processed_records = ?, success_count = ?, error_count = ? WHERE id = ?`,
				i+1, successCount, errorCount, jobID)
		}
	}

	resultJSON, _ := json.Marshal(map[string]interface{}{
		"errors": errors,
	})

	w.db.Exec(
		`UPDATE bulk_jobs SET status = 'completed', processed_records = ?, success_count = ?, error_count = ?, result_summary = ?, completed_at = CURRENT_TIMESTAMP WHERE id = ?`,
		total, successCount, errorCount, string(resultJSON), jobID)

	w.logger.Info("worker: bulk job completed", "jobID", jobID, "success", successCount, "errors", errorCount)
}

// EnqueueBulkAdd creates a bulk_jobs record with the payload stored in result_summary (as JSON).
func EnqueueBulkAdd(ctx context.Context, db *sql.DB, orgID, userID int64, records []bulkRecord, channel, numberType, fileName string) (int64, error) {
	payload, err := json.Marshal(bulkJobPayload{
		Records:    records,
		Channel:    channel,
		NumberType: numberType,
	})
	if err != nil {
		return 0, fmt.Errorf("marshaling payload: %w", err)
	}

	result, err := db.ExecContext(ctx,
		`INSERT INTO bulk_jobs (org_id, user_id, job_type, status, total_records, file_name, result_summary)
		 VALUES (?, ?, 'add', 'pending', ?, ?, ?)`,
		orgID, userID, len(records), fileName, string(payload))
	if err != nil {
		return 0, fmt.Errorf("inserting bulk job: %w", err)
	}

	id, err := result.LastInsertId()
	if err != nil {
		return 0, err
	}
	return id, nil
}

// ensurePartitions creates upcoming monthly query_log partitions (PostgreSQL only).
func (w *Worker) ensurePartitions() {
	if err := w.appDB.EnsureQueryLogPartitions(); err != nil {
		w.logger.Error("worker: partition maintenance", "error", err)
	}
}

// BulkRecord is exported for use by handlers.
type BulkRecord = bulkRecord

// retryFailed re-queues failed jobs that haven't exceeded max retries.
func (w *Worker) retryFailed() {
	// Count how many times each job has been attempted by checking if it has a
	// result_summary containing prior errors. Simple approach: just re-queue
	// failed jobs up to maxRetries times by resetting to pending.
	result, err := w.db.Exec(
		`UPDATE bulk_jobs SET status = 'pending'
		 WHERE status = 'failed' AND error_count < ?
		 AND completed_at < datetime('now', '-1 minute')`,
		maxRetries,
	)
	if err != nil {
		w.logger.Error("worker: retry query", "error", err)
		return
	}
	rows, _ := result.RowsAffected()
	if rows > 0 {
		w.logger.Info("worker: retrying failed jobs", "count", rows)
	}
}

// cleanupQueryLog deletes query log entries older than 90 days.
func (w *Worker) cleanupQueryLog() {
	result, err := w.db.Exec(
		`DELETE FROM query_log WHERE queried_at < datetime('now', '-90 days')`)
	if err != nil {
		w.logger.Error("worker: query log cleanup", "error", err)
		return
	}
	rows, _ := result.RowsAffected()
	if rows > 0 {
		w.logger.Info("worker: cleaned up old query logs", "deleted", rows)
	}
}

