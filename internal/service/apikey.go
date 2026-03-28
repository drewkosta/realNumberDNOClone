package service

import (
	"context"
	"crypto/rand"
	"crypto/sha256"
	"database/sql"
	"encoding/hex"
	"fmt"

	appdb "realNumberDNOClone/internal/db"
)

type APIKeyService struct {
	db *appdb.DB
}

func NewAPIKeyService(d *appdb.DB) *APIKeyService {
	return &APIKeyService{db: d}
}

// GenerateKey creates a new API key for an organization. Returns the raw key
// (shown once to the caller) and stores the SHA-256 hash in the database.
func (s *APIKeyService) GenerateKey(ctx context.Context, orgID int64) (string, error) {
	raw, err := randomKey(32)
	if err != nil {
		return "", fmt.Errorf("generating random key: %w", err)
	}

	prefixed := "dno_" + raw
	hashed := hashKey(prefixed)

	_, err = s.db.Writer.ExecContext(ctx, s.db.Q(
		`UPDATE organizations SET api_key = $1, updated_at = `+s.db.QNow()+` WHERE id = $2`),
		hashed, orgID,
	)
	if err != nil {
		return "", fmt.Errorf("storing api key: %w", err)
	}

	return prefixed, nil
}

// RevokeKey removes the API key for an organization.
func (s *APIKeyService) RevokeKey(ctx context.Context, orgID int64) error {
	_, err := s.db.Writer.ExecContext(ctx, s.db.Q(
		`UPDATE organizations SET api_key = NULL, updated_at = `+s.db.QNow()+` WHERE id = $1`),
		orgID,
	)
	return err
}

// ValidateKey checks an API key against the database and returns the org ID
// if valid. Returns 0 and an error if the key is invalid.
func (s *APIKeyService) ValidateKey(ctx context.Context, rawKey string) (int64, error) {
	hashed := hashKey(rawKey)

	var orgID int64
	err := s.db.Reader.QueryRowContext(ctx, s.db.Q(
		`SELECT id FROM organizations WHERE api_key = $1`),
		hashed,
	).Scan(&orgID)
	if err != nil {
		if err == sql.ErrNoRows {
			return 0, fmt.Errorf("invalid api key")
		}
		return 0, fmt.Errorf("validating api key: %w", err)
	}

	return orgID, nil
}

func hashKey(key string) string {
	h := sha256.Sum256([]byte(key))
	return hex.EncodeToString(h[:])
}

func randomKey(bytes int) (string, error) {
	b := make([]byte, bytes)
	if _, err := rand.Read(b); err != nil {
		return "", err
	}
	return hex.EncodeToString(b), nil
}
