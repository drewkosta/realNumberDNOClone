package service

import (
	"context"
	"testing"

	"realNumberDNOClone/internal/db"
	"realNumberDNOClone/internal/config"
	"realNumberDNOClone/internal/models"
)

func setupTestDB(t *testing.T) *db.DB {
	t.Helper()
	tmpFile := t.TempDir() + "/test.db"
	cfg := &config.Config{
		DBDriver: config.DBDriverSQLite,
		DBPath:   tmpFile,
	}
	database, err := db.Initialize(cfg)
	if err != nil {
		t.Fatalf("failed to init test db: %v", err)
	}
	return database
}

func TestAuthService_CreateAndLogin(t *testing.T) {
	database := setupTestDB(t)
	defer database.Close()

	svc := NewAuthService(database, "test-secret")

	// Create user
	user, err := svc.CreateUser(context.Background(), models.CreateUserRequest{
		Email:     "test@example.com",
		Password:  "password123",
		FirstName: "Test",
		LastName:  "User",
		Role:      "operator",
	})
	if err != nil {
		t.Fatalf("CreateUser: %v", err)
	}
	if user.Email != "test@example.com" {
		t.Errorf("email = %q, want test@example.com", user.Email)
	}

	// Login
	resp, err := svc.Login(context.Background(), "test@example.com", "password123")
	if err != nil {
		t.Fatalf("Login: %v", err)
	}
	if resp.Token == "" {
		t.Error("expected non-empty token")
	}
	if resp.User.Email != "test@example.com" {
		t.Errorf("login user email = %q", resp.User.Email)
	}

	// Wrong password
	_, err = svc.Login(context.Background(), "test@example.com", "wrong")
	if err == nil {
		t.Error("expected error for wrong password")
	}

	// Non-existent user
	_, err = svc.Login(context.Background(), "nobody@example.com", "password123")
	if err == nil {
		t.Error("expected error for non-existent user")
	}
}

func TestAuthService_ValidateToken(t *testing.T) {
	database := setupTestDB(t)
	defer database.Close()

	svc := NewAuthService(database, "test-secret")

	svc.CreateUser(context.Background(), models.CreateUserRequest{
		Email: "tok@test.com", Password: "password123",
		FirstName: "T", LastName: "K", Role: "admin",
	})

	resp, _ := svc.Login(context.Background(), "tok@test.com", "password123")

	claims, err := svc.ValidateToken(resp.Token)
	if err != nil {
		t.Fatalf("ValidateToken: %v", err)
	}
	if claims["email"] != "tok@test.com" {
		t.Errorf("email claim = %v", claims["email"])
	}
	if claims["role"] != "admin" {
		t.Errorf("role claim = %v", claims["role"])
	}

	// Invalid token
	_, err = svc.ValidateToken("garbage")
	if err == nil {
		t.Error("expected error for invalid token")
	}
}

func TestAuthService_Validation(t *testing.T) {
	database := setupTestDB(t)
	defer database.Close()

	svc := NewAuthService(database, "test-secret")

	// Short password
	_, err := svc.CreateUser(context.Background(), models.CreateUserRequest{
		Email: "a@b.com", Password: "short", FirstName: "A", LastName: "B", Role: "viewer",
	})
	if err == nil {
		t.Error("expected error for short password")
	}

	// Invalid role
	_, err = svc.CreateUser(context.Background(), models.CreateUserRequest{
		Email: "a@b.com", Password: "password123", FirstName: "A", LastName: "B", Role: "superadmin",
	})
	if err == nil {
		t.Error("expected error for invalid role")
	}

	// Missing fields
	_, err = svc.CreateUser(context.Background(), models.CreateUserRequest{})
	if err == nil {
		t.Error("expected error for missing fields")
	}
}
