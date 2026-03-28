package querylog

import (
	"context"
	"database/sql"
	"log/slog"
	"sync"
	"time"
)

type Entry struct {
	OrgID       int64
	PhoneNumber string
	Result      string // "hit" or "miss"
	Channel     string
}

// AsyncWriter buffers query log entries and batch-flushes them to the database
// on a timer or when the buffer reaches a threshold. This removes synchronous
// writes from the hot DNO lookup path.
type AsyncWriter struct {
	db        *sql.DB
	buf       []Entry
	mu        sync.Mutex
	flushSize int
	interval  time.Duration
	done      chan struct{}
	logger    *slog.Logger
}

func NewAsyncWriter(db *sql.DB, flushSize int, interval time.Duration, logger *slog.Logger) *AsyncWriter {
	w := &AsyncWriter{
		db:        db,
		buf:       make([]Entry, 0, flushSize),
		flushSize: flushSize,
		interval:  interval,
		done:      make(chan struct{}),
		logger:    logger,
	}
	go w.flushLoop()
	return w
}

func (w *AsyncWriter) Log(e Entry) {
	w.mu.Lock()
	w.buf = append(w.buf, e)
	shouldFlush := len(w.buf) >= w.flushSize
	w.mu.Unlock()

	if shouldFlush {
		go w.flush()
	}
}

func (w *AsyncWriter) Stop() {
	close(w.done)
	w.flush() // final flush
}

func (w *AsyncWriter) flushLoop() {
	ticker := time.NewTicker(w.interval)
	defer ticker.Stop()
	for {
		select {
		case <-ticker.C:
			w.flush()
		case <-w.done:
			return
		}
	}
}

func (w *AsyncWriter) flush() {
	w.mu.Lock()
	if len(w.buf) == 0 {
		w.mu.Unlock()
		return
	}
	entries := w.buf
	w.buf = make([]Entry, 0, w.flushSize)
	w.mu.Unlock()

	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	tx, err := w.db.BeginTx(ctx, nil)
	if err != nil {
		w.logger.Error("query log flush: begin tx", "error", err, "dropped", len(entries))
		return
	}

	stmt, err := tx.PrepareContext(ctx,
		`INSERT INTO query_log (org_id, phone_number, result, channel) VALUES (?, ?, ?, ?)`)
	if err != nil {
		tx.Rollback()
		w.logger.Error("query log flush: prepare", "error", err, "dropped", len(entries))
		return
	}
	defer stmt.Close()

	for _, e := range entries {
		if _, err := stmt.ExecContext(ctx, e.OrgID, e.PhoneNumber, e.Result, e.Channel); err != nil {
			w.logger.Error("query log flush: exec", "error", err, "phone", e.PhoneNumber)
		}
	}

	if err := tx.Commit(); err != nil {
		w.logger.Error("query log flush: commit", "error", err, "dropped", len(entries))
		return
	}

	w.logger.Debug("query log flushed", "count", len(entries))
}
