package api

import (
	"context"
	"net/http"
	"strings"

	"realNumberDNOClone/internal/service"
)

type contextKey string

const (
	UserIDKey contextKey = "userID"
	OrgIDKey  contextKey = "orgID"
	RoleKey   contextKey = "role"
)

// AuthMiddleware validates a JWT Bearer token and sets user/org/role context.
func AuthMiddleware(authService *service.AuthService) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			authHeader := r.Header.Get("Authorization")
			if authHeader == "" {
				http.Error(w, `{"error":"missing authorization header"}`, http.StatusUnauthorized)
				return
			}

			tokenString := strings.TrimPrefix(authHeader, "Bearer ")
			if tokenString == authHeader {
				http.Error(w, `{"error":"invalid authorization format"}`, http.StatusUnauthorized)
				return
			}

			claims, err := authService.ValidateToken(tokenString)
			if err != nil {
				http.Error(w, `{"error":"invalid or expired token"}`, http.StatusUnauthorized)
				return
			}

			ctx := r.Context()
			if sub, ok := claims["sub"].(float64); ok {
				ctx = context.WithValue(ctx, UserIDKey, int64(sub))
			}
			if orgID, ok := claims["org_id"].(float64); ok {
				ctx = context.WithValue(ctx, OrgIDKey, int64(orgID))
			}
			if role, ok := claims["role"].(string); ok {
				ctx = context.WithValue(ctx, RoleKey, role)
			}

			next.ServeHTTP(w, r.WithContext(ctx))
		})
	}
}

// APIKeyMiddleware validates an X-API-Key header against the organizations table.
// On success it sets OrgIDKey in context (no UserIDKey or RoleKey -- API key
// callers are org-level, not user-level). Falls back to JWT auth if no API key
// header is present.
func APIKeyMiddleware(apiKeyService *service.APIKeyService, authService *service.AuthService) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			apiKey := r.Header.Get("X-API-Key")
			if apiKey == "" {
				// No API key -- fall through to JWT auth
				AuthMiddleware(authService)(next).ServeHTTP(w, r)
				return
			}

			orgID, err := apiKeyService.ValidateKey(r.Context(), apiKey)
			if err != nil {
				http.Error(w, `{"error":"invalid api key"}`, http.StatusUnauthorized)
				return
			}

			ctx := context.WithValue(r.Context(), OrgIDKey, orgID)
			ctx = context.WithValue(ctx, RoleKey, "api_key")
			next.ServeHTTP(w, r.WithContext(ctx))
		})
	}
}

func AdminOnly(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		role, _ := r.Context().Value(RoleKey).(string)
		if role != "admin" {
			http.Error(w, `{"error":"admin access required"}`, http.StatusForbidden)
			return
		}
		next.ServeHTTP(w, r)
	})
}
