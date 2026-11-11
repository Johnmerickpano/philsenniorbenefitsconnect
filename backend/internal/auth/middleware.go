package auth

import (
	"context"
	"errors"
	"net/http"
	"strings"

	"github.com/Johnmerickpano/philsenniorbenefitsconnect/internal/database/schema"
	"github.com/gocql/gocql"
)

type authenticatedUserKey struct{}

// AuthenticatedUser contains the identity and current role of an authenticated account.
type AuthenticatedUser struct {
	UserID string
	Role   schema.UserRole
}

// AuthenticatedUserFromContext returns the authenticated principal from a request context.
func AuthenticatedUserFromContext(ctx context.Context) (AuthenticatedUser, bool) {
	user, ok := ctx.Value(authenticatedUserKey{}).(AuthenticatedUser)
	return user, ok
}

// Authenticate validates the access token and revalidates the account against the database.
func (s *Service) Authenticate(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if s == nil || s.tokens == nil || s.repository == nil {
			writeJSON(w, http.StatusInternalServerError, errorResponse{
				Error: "authentication service unavailable",
			})
			return
		}

		parts := strings.Fields(r.Header.Get("Authorization"))
		if len(parts) != 2 || !strings.EqualFold(parts[0], "Bearer") {
			writeJSON(w, http.StatusUnauthorized, errorResponse{
				Error: "valid bearer access token required",
			})
			return
		}

		claims, err := s.tokens.ValidateAccessToken(parts[1])
		if err != nil {
			writeJSON(w, http.StatusUnauthorized, errorResponse{
				Error: "invalid or expired access token",
			})
			return
		}

		userID, err := gocql.ParseUUID(claims.Subject)
		if err != nil {
			writeJSON(w, http.StatusUnauthorized, errorResponse{
				Error: "invalid access token identity",
			})
			return
		}

		account, err := s.repository.GetUserByID(r.Context(), userID)
		if err != nil {
			if errors.Is(err, ErrUserNotFound) {
				writeJSON(w, http.StatusUnauthorized, errorResponse{
					Error: "account is unavailable",
				})
				return
			}

			writeJSON(w, http.StatusServiceUnavailable, errorResponse{
				Error: "authentication temporarily unavailable",
			})
			return
		}

		if account == nil || account.UserID != userID || !account.IsActive ||
			!isValidRole(account.UserRole) {
			writeJSON(w, http.StatusUnauthorized, errorResponse{
				Error: "account is unavailable",
			})
			return
		}

		user := AuthenticatedUser{
			UserID: account.UserID.String(),
			Role:   account.UserRole,
		}

		ctx := context.WithValue(r.Context(), authenticatedUserKey{}, user)
		next.ServeHTTP(w, r.WithContext(ctx))
	})
}

// RequireRoles permits requests only when the authenticated user's role is allowed.
// Apply Authenticate before this middleware.
func RequireRoles(allowedRoles ...schema.UserRole) func(http.Handler) http.Handler {
	allowed := make(map[schema.UserRole]struct{}, len(allowedRoles))
	for _, role := range allowedRoles {
		if isValidRole(role) {
			allowed[role] = struct{}{}
		}
	}

	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			user, ok := AuthenticatedUserFromContext(r.Context())
			if !ok {
				writeJSON(w, http.StatusUnauthorized, errorResponse{
					Error: "authentication required",
				})
				return
			}

			if _, permitted := allowed[user.Role]; !permitted {
				writeJSON(w, http.StatusForbidden, errorResponse{
					Error: "insufficient permissions",
				})
				return
			}

			next.ServeHTTP(w, r)
		})
	}
}
