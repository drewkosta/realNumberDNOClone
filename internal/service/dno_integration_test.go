package service

import (
	"context"
	"log/slog"
	"os"
	"testing"
	"time"

	"realNumberDNOClone/internal/cache"
	"realNumberDNOClone/internal/config"
	"realNumberDNOClone/internal/db"
	"realNumberDNOClone/internal/models"
	"realNumberDNOClone/internal/querylog"
)

func setupDNOTest(t *testing.T) (*DNOService, *db.DB) {
	t.Helper()
	tmpFile := t.TempDir() + "/dno_test.db"
	cfg := &config.Config{DBDriver: config.DBDriverSQLite, DBPath: tmpFile}
	database, err := db.Initialize(cfg)
	if err != nil {
		t.Fatalf("init db: %v", err)
	}

	logger := slog.New(slog.NewTextHandler(os.Stderr, &slog.HandlerOptions{Level: slog.LevelError}))
	qlWriter := querylog.NewAsyncWriter(database.Writer, 100, 5*time.Second, logger)
	dnoCache := cache.New[*models.DNOQueryResponse](30*time.Second, 1000)
	analyticsCache := cache.New[*models.AnalyticsSummary](30*time.Second, 10)

	svc := NewDNOService(database, qlWriter, dnoCache, analyticsCache)
	return svc, database
}

func TestDNOService_AddAndQuery(t *testing.T) {
	svc, database := setupDNOTest(t)
	defer database.Close()
	ctx := context.Background()

	// Query non-existent number
	resp, err := svc.QueryNumber(ctx, "5551234567", "voice", nil)
	if err != nil {
		t.Fatalf("QueryNumber: %v", err)
	}
	if resp.IsDNO {
		t.Error("expected miss for non-existent number")
	}

	// Add number
	num, err := svc.AddNumber(ctx, models.AddDNORequest{
		PhoneNumber: "5551234567",
		NumberType:  "local",
		Channel:     "voice",
		Reason:      "test inbound only",
	}, 1, 1)
	if err != nil {
		t.Fatalf("AddNumber: %v", err)
	}
	if num.PhoneNumber != "5551234567" {
		t.Errorf("phone = %q", num.PhoneNumber)
	}
	if num.Dataset != "subscriber" {
		t.Errorf("dataset = %q", num.Dataset)
	}

	// Query should now hit (cache might serve it, either way should be DNO)
	resp, err = svc.QueryNumber(ctx, "5551234567", "voice", nil)
	if err != nil {
		t.Fatalf("QueryNumber after add: %v", err)
	}
	if !resp.IsDNO {
		t.Error("expected hit after adding number")
	}
	if resp.Dataset != "subscriber" {
		t.Errorf("dataset = %q, want subscriber", resp.Dataset)
	}
}

func TestDNOService_AddAndRemove(t *testing.T) {
	svc, database := setupDNOTest(t)
	defer database.Close()
	ctx := context.Background()

	svc.AddNumber(ctx, models.AddDNORequest{
		PhoneNumber: "5559876543", NumberType: "local", Channel: "voice",
	}, 1, 1)

	// Remove
	err := svc.RemoveNumber(ctx, "5559876543", "voice", 1, 1)
	if err != nil {
		t.Fatalf("RemoveNumber: %v", err)
	}

	// Query should miss
	resp, _ := svc.QueryNumber(ctx, "5559876543", "voice", nil)
	if resp.IsDNO {
		t.Error("expected miss after removal")
	}
}

func TestDNOService_RemoveUnauthorized(t *testing.T) {
	svc, database := setupDNOTest(t)
	defer database.Close()
	ctx := context.Background()

	svc.AddNumber(ctx, models.AddDNORequest{
		PhoneNumber: "5551111111", NumberType: "local", Channel: "voice",
	}, 1, 1)

	// Try to remove with different org
	err := svc.RemoveNumber(ctx, "5551111111", "voice", 999, 999)
	if err == nil {
		t.Error("expected error when removing with wrong org")
	}
}

func TestDNOService_UpsertUpdatesReason(t *testing.T) {
	svc, database := setupDNOTest(t)
	defer database.Close()
	ctx := context.Background()

	svc.AddNumber(ctx, models.AddDNORequest{
		PhoneNumber: "5552222222", NumberType: "local", Channel: "voice", Reason: "original",
	}, 1, 1)

	num, err := svc.AddNumber(ctx, models.AddDNORequest{
		PhoneNumber: "5552222222", NumberType: "local", Channel: "voice", Reason: "updated",
	}, 1, 1)
	if err != nil {
		t.Fatalf("upsert: %v", err)
	}
	if num.Reason == nil || *num.Reason != "updated" {
		t.Errorf("reason = %v, want updated", num.Reason)
	}
}

func TestDNOService_BulkQuery(t *testing.T) {
	svc, database := setupDNOTest(t)
	defer database.Close()
	ctx := context.Background()

	svc.AddNumber(ctx, models.AddDNORequest{PhoneNumber: "5550000001", Channel: "voice"}, 1, 1)
	svc.AddNumber(ctx, models.AddDNORequest{PhoneNumber: "5550000002", Channel: "voice"}, 1, 1)

	resp, err := svc.BulkQuery(ctx, []string{"5550000001", "5550000002", "5550000003", "bad"}, "voice", nil)
	if err != nil {
		t.Fatalf("BulkQuery: %v", err)
	}
	if resp.Total != 4 {
		t.Errorf("total = %d, want 4", resp.Total)
	}
	if resp.Hits != 2 {
		t.Errorf("hits = %d, want 2", resp.Hits)
	}
	if resp.Misses != 2 {
		t.Errorf("misses = %d, want 2 (1 miss + 1 error)", resp.Misses)
	}
	// "bad" should be an error entry
	errorCount := 0
	for _, r := range resp.Results {
		if r.Status == "error" {
			errorCount++
		}
	}
	if errorCount != 1 {
		t.Errorf("error entries = %d, want 1", errorCount)
	}
}

func TestDNOService_BulkQueryLimit(t *testing.T) {
	svc, database := setupDNOTest(t)
	defer database.Close()

	phones := make([]string, 1001)
	for i := range phones {
		phones[i] = "5550000000"
	}
	_, err := svc.BulkQuery(context.Background(), phones, "voice", nil)
	if err == nil {
		t.Error("expected error for >1000 numbers")
	}
}

func TestDNOService_ChannelValidation(t *testing.T) {
	svc, database := setupDNOTest(t)
	defer database.Close()

	_, err := svc.QueryNumber(context.Background(), "5551234567", "fax", nil)
	if err == nil {
		t.Error("expected error for invalid channel")
	}

	_, err = svc.AddNumber(context.Background(), models.AddDNORequest{
		PhoneNumber: "5551234567", Channel: "fax",
	}, 1, 1)
	if err == nil {
		t.Error("expected error for invalid channel on add")
	}
}

func TestDNOService_PhoneFormatting(t *testing.T) {
	svc, database := setupDNOTest(t)
	defer database.Close()
	ctx := context.Background()

	// Add with formatted number
	svc.AddNumber(ctx, models.AddDNORequest{
		PhoneNumber: "(555) 333-4444", Channel: "voice",
	}, 1, 1)

	// Query with different formatting should still match
	resp, err := svc.QueryNumber(ctx, "+1-555-333-4444", "voice", nil)
	if err != nil {
		t.Fatalf("query: %v", err)
	}
	if !resp.IsDNO {
		t.Error("expected hit with different phone formatting")
	}
}

func TestDNOService_ListNumbers(t *testing.T) {
	svc, database := setupDNOTest(t)
	defer database.Close()
	ctx := context.Background()

	for i := 0; i < 30; i++ {
		phone := "555000" + padInt(i, 4)
		svc.AddNumber(ctx, models.AddDNORequest{
			PhoneNumber: phone, NumberType: "local", Channel: "voice",
		}, 1, 1)
	}

	// Default page
	result, err := svc.ListNumbers(ctx, nil, "", "", "", "", 1, 10)
	if err != nil {
		t.Fatalf("ListNumbers: %v", err)
	}
	if result.Total != 30 {
		t.Errorf("total = %d, want 30", result.Total)
	}
	if len(result.Data) != 10 {
		t.Errorf("data len = %d, want 10", len(result.Data))
	}
	if result.TotalPages != 3 {
		t.Errorf("totalPages = %d, want 3", result.TotalPages)
	}

	// Page 2
	result, err = svc.ListNumbers(ctx, nil, "", "", "", "", 2, 10)
	if err != nil {
		t.Fatalf("ListNumbers page 2: %v", err)
	}
	if result.Page != 2 {
		t.Errorf("page = %d, want 2", result.Page)
	}

	// Filter by dataset
	result, err = svc.ListNumbers(ctx, nil, "subscriber", "", "", "", 1, 100)
	if err != nil {
		t.Fatalf("ListNumbers filtered: %v", err)
	}
	if result.Total != 30 {
		t.Errorf("filtered total = %d, want 30", result.Total)
	}

	// Invalid dataset
	_, err = svc.ListNumbers(ctx, nil, "invalid", "", "", "", 1, 10)
	if err == nil {
		t.Error("expected error for invalid dataset filter")
	}
}

func TestDNOService_Analytics(t *testing.T) {
	svc, database := setupDNOTest(t)
	defer database.Close()
	ctx := context.Background()

	svc.AddNumber(ctx, models.AddDNORequest{PhoneNumber: "5550000001", Channel: "voice"}, 1, 1)
	svc.AddNumber(ctx, models.AddDNORequest{PhoneNumber: "5550000002", Channel: "text"}, 1, 1)

	summary, err := svc.GetAnalytics(ctx, nil)
	if err != nil {
		t.Fatalf("GetAnalytics: %v", err)
	}
	if summary.TotalDNONumbers != 2 {
		t.Errorf("total = %d, want 2", summary.TotalDNONumbers)
	}
	if summary.ByDataset["subscriber"] != 2 {
		t.Errorf("subscriber count = %d, want 2", summary.ByDataset["subscriber"])
	}
}

func TestDNOService_TextChannel(t *testing.T) {
	svc, database := setupDNOTest(t)
	defer database.Close()
	ctx := context.Background()

	svc.AddNumber(ctx, models.AddDNORequest{
		PhoneNumber: "8001234567", NumberType: "toll_free", Channel: "text",
	}, 1, 1)

	// Voice query should miss
	resp, _ := svc.QueryNumber(ctx, "8001234567", "voice", nil)
	if resp.IsDNO {
		t.Error("voice query should miss for text-only number")
	}

	// Text query should hit
	resp, _ = svc.QueryNumber(ctx, "8001234567", "text", nil)
	if !resp.IsDNO {
		t.Error("text query should hit")
	}
}

func TestDNOService_BothChannel(t *testing.T) {
	svc, database := setupDNOTest(t)
	defer database.Close()
	ctx := context.Background()

	svc.AddNumber(ctx, models.AddDNORequest{
		PhoneNumber: "8005551234", Channel: "both",
	}, 1, 1)

	// Both voice and text should hit
	for _, ch := range []string{"voice", "text"} {
		resp, _ := svc.QueryNumber(ctx, "8005551234", ch, nil)
		if !resp.IsDNO {
			t.Errorf("%s query should hit for both-channel number", ch)
		}
	}
}

func padInt(n, width int) string {
	s := ""
	for i := 0; i < width; i++ {
		s = string(rune('0'+n%10)) + s
		n /= 10
	}
	return s
}
