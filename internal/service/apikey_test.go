package service

import (
	"context"
	"testing"

	"realNumberDNOClone/internal/config"
	"realNumberDNOClone/internal/db"
)

func setupAPIKeyTest(t *testing.T) (*APIKeyService, *db.DB) {
	t.Helper()
	tmpFile := t.TempDir() + "/apikey_test.db"
	cfg := &config.Config{DBDriver: config.DBDriverSQLite, DBPath: tmpFile}
	database, err := db.Initialize(cfg)
	if err != nil {
		t.Fatalf("init db: %v", err)
	}
	return NewAPIKeyService(database), database
}

func TestAPIKeyService_GenerateAndValidate(t *testing.T) {
	svc, database := setupAPIKeyTest(t)
	defer database.Close()
	ctx := context.Background()

	// Org 1 is the seeded "System Admin" org
	key, err := svc.GenerateKey(ctx, 1)
	if err != nil {
		t.Fatalf("GenerateKey: %v", err)
	}
	if key == "" {
		t.Fatal("expected non-empty key")
	}
	if key[:4] != "dno_" {
		t.Errorf("key prefix = %q, want dno_", key[:4])
	}

	// Validate
	orgID, err := svc.ValidateKey(ctx, key)
	if err != nil {
		t.Fatalf("ValidateKey: %v", err)
	}
	if orgID != 1 {
		t.Errorf("orgID = %d, want 1", orgID)
	}
}

func TestAPIKeyService_InvalidKey(t *testing.T) {
	svc, database := setupAPIKeyTest(t)
	defer database.Close()

	_, err := svc.ValidateKey(context.Background(), "bad_key")
	if err == nil {
		t.Error("expected error for invalid key")
	}
}

func TestAPIKeyService_Revoke(t *testing.T) {
	svc, database := setupAPIKeyTest(t)
	defer database.Close()
	ctx := context.Background()

	key, _ := svc.GenerateKey(ctx, 1)

	// Revoke
	err := svc.RevokeKey(ctx, 1)
	if err != nil {
		t.Fatalf("RevokeKey: %v", err)
	}

	// Should no longer validate
	_, err = svc.ValidateKey(ctx, key)
	if err == nil {
		t.Error("expected error after revocation")
	}
}

func TestAPIKeyService_RegenerateReplacesOld(t *testing.T) {
	svc, database := setupAPIKeyTest(t)
	defer database.Close()
	ctx := context.Background()

	key1, _ := svc.GenerateKey(ctx, 1)
	key2, _ := svc.GenerateKey(ctx, 1)

	if key1 == key2 {
		t.Error("regenerated key should be different")
	}

	// Old key should be invalid
	_, err := svc.ValidateKey(ctx, key1)
	if err == nil {
		t.Error("old key should be invalid after regeneration")
	}

	// New key should work
	orgID, err := svc.ValidateKey(ctx, key2)
	if err != nil {
		t.Fatalf("new key validation: %v", err)
	}
	if orgID != 1 {
		t.Errorf("orgID = %d, want 1", orgID)
	}
}
