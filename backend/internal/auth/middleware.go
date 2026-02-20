package auth

import (
	"context"
	"net/http"
	"strings"
)

type contextKey string

const userIDCtxKey contextKey = "user_id"
const userEmailCtxKey contextKey = "user_email"

// Middleware returns an http.Handler middleware that validates JWT from the Authorization header.
// On success it injects user_id and user_email into the request context.
func Middleware(jwtSecret string) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			header := r.Header.Get("Authorization")
			if header == "" {
				http.Error(w, `{"error":"unauthorized","message":"missing Authorization header"}`, http.StatusUnauthorized)
				return
			}

			parts := strings.SplitN(header, " ", 2)
			if len(parts) != 2 || !strings.EqualFold(parts[0], "bearer") {
				http.Error(w, `{"error":"unauthorized","message":"invalid Authorization format, expected: Bearer <token>"}`, http.StatusUnauthorized)
				return
			}

			claims, err := ParseToken(parts[1], jwtSecret)
			if err != nil {
				http.Error(w, `{"error":"unauthorized","message":"`+err.Error()+`"}`, http.StatusUnauthorized)
				return
			}

			ctx := context.WithValue(r.Context(), userIDCtxKey, claims.UserID)
			ctx = context.WithValue(ctx, userEmailCtxKey, claims.Email)
			next.ServeHTTP(w, r.WithContext(ctx))
		})
	}
}

// GetUserID extracts the authenticated user ID from the request context.
func GetUserID(r *http.Request) (int64, bool) {
	id, ok := r.Context().Value(userIDCtxKey).(int64)
	return id, ok
}
