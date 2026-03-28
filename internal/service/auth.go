package service

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"time"

	"github.com/golang-jwt/jwt/v5"
	"golang.org/x/crypto/bcrypt"

	"realNumberDNOClone/internal/models"
)

type AuthService struct {
	db        *sql.DB
	jwtSecret []byte
}

func NewAuthService(db *sql.DB, jwtSecret string) *AuthService {
	return &AuthService{db: db, jwtSecret: []byte(jwtSecret)}
}

func (s *AuthService) Login(ctx context.Context, email, password string) (*models.LoginResponse, error) {
	var user models.User
	var orgID sql.NullInt64
	err := s.db.QueryRowContext(ctx,
		`SELECT id, email, password_hash, first_name, last_name, role, org_id, active FROM users WHERE email = ?`,
		email,
	).Scan(&user.ID, &user.Email, &user.PasswordHash, &user.FirstName, &user.LastName, &user.Role, &orgID, &user.Active)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return nil, fmt.Errorf("invalid credentials")
		}
		return nil, err
	}

	if !user.Active {
		return nil, fmt.Errorf("invalid credentials")
	}

	if err := bcrypt.CompareHashAndPassword([]byte(user.PasswordHash), []byte(password)); err != nil {
		return nil, fmt.Errorf("invalid credentials")
	}

	if orgID.Valid {
		user.OrgID = &orgID.Int64
	}

	token, err := s.generateToken(user)
	if err != nil {
		return nil, err
	}

	return &models.LoginResponse{Token: token, User: user}, nil
}

func (s *AuthService) generateToken(user models.User) (string, error) {
	claims := jwt.MapClaims{
		"sub":   user.ID,
		"email": user.Email,
		"role":  user.Role,
		"exp":   time.Now().Add(24 * time.Hour).Unix(),
		"iat":   time.Now().Unix(),
	}
	if user.OrgID != nil {
		claims["org_id"] = *user.OrgID
	}

	token := jwt.NewWithClaims(jwt.SigningMethodHS256, claims)
	return token.SignedString(s.jwtSecret)
}

func (s *AuthService) ValidateToken(tokenString string) (jwt.MapClaims, error) {
	token, err := jwt.Parse(tokenString, func(token *jwt.Token) (interface{}, error) {
		if _, ok := token.Method.(*jwt.SigningMethodHMAC); !ok {
			return nil, fmt.Errorf("unexpected signing method: %v", token.Header["alg"])
		}
		return s.jwtSecret, nil
	})
	if err != nil {
		return nil, err
	}

	claims, ok := token.Claims.(jwt.MapClaims)
	if !ok || !token.Valid {
		return nil, fmt.Errorf("invalid token")
	}

	return claims, nil
}

func (s *AuthService) CreateUser(ctx context.Context, req models.CreateUserRequest) (*models.User, error) {
	if req.Email == "" || req.Password == "" || req.FirstName == "" || req.LastName == "" {
		return nil, fmt.Errorf("email, password, firstName, and lastName are required")
	}
	if len(req.Password) < 8 {
		return nil, fmt.Errorf("password must be at least 8 characters")
	}
	if err := models.ValidateRole(req.Role); err != nil {
		return nil, err
	}

	hash, err := bcrypt.GenerateFromPassword([]byte(req.Password), bcrypt.DefaultCost)
	if err != nil {
		return nil, fmt.Errorf("hashing password: %w", err)
	}

	result, err := s.db.ExecContext(ctx,
		`INSERT INTO users (email, password_hash, first_name, last_name, role, org_id) VALUES (?, ?, ?, ?, ?, ?)`,
		req.Email, string(hash), req.FirstName, req.LastName, req.Role, req.OrgID,
	)
	if err != nil {
		return nil, fmt.Errorf("creating user: %w", err)
	}

	id, err := result.LastInsertId()
	if err != nil {
		return nil, fmt.Errorf("getting new user id: %w", err)
	}

	return &models.User{
		ID:        id,
		Email:     req.Email,
		FirstName: req.FirstName,
		LastName:  req.LastName,
		Role:      req.Role,
		OrgID:     req.OrgID,
		Active:    true,
	}, nil
}

func (s *AuthService) GetUser(ctx context.Context, id int64) (*models.User, error) {
	var user models.User
	var orgID sql.NullInt64
	err := s.db.QueryRowContext(ctx,
		`SELECT id, email, first_name, last_name, role, org_id, active, created_at FROM users WHERE id = ?`, id,
	).Scan(&user.ID, &user.Email, &user.FirstName, &user.LastName, &user.Role, &orgID, &user.Active, &user.CreatedAt)
	if err != nil {
		return nil, err
	}
	if orgID.Valid {
		user.OrgID = &orgID.Int64
	}
	return &user, nil
}
