package auth

import (
	"context"
	"errors"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/Johnmerickpano/philsenniorbenefitsconnect/internal/database/schema"
	"github.com/golang-jwt/jwt/v5"
)

func newMiddlewareTestService(
	t *testing.T,
	user *schema.User,
) (*Service, *fakeAuthRepository, *TokenManager) {
	t.Helper()

	manager := testTokenManager(t)
	repository := &fakeAuthRepository{user: user}

	service := &Service{
		repository: repository,
		tokens:     manager,
	}

	return service, repository, manager
}

func TestAuthenticateRejectsMissingAndMalformedBearerHeaders(t *testing.T) {
	service, _, _ := newMiddlewareTestService(t, newTestUser(t))

	tests := []struct {
		name   string
		header string
	}{
		{name: "missing header", header: ""},
		{name: "missing token", header: "Bearer"},
		{name: "wrong scheme", header: "Basic abc123"},
		{name: "extra fields", header: "Bearer abc123 extra"},
		{name: "empty token", header: "Bearer   "},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			next := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				t.Fatal("protected handler must not be called")
			})

			req := httptest.NewRequest(http.MethodGet, "/protected", nil)
			req.Header.Set("Authorization", tt.header)
			rec := httptest.NewRecorder()

			service.Authenticate(next).ServeHTTP(rec, req)

			if rec.Code != http.StatusUnauthorized {
				t.Fatalf("expected 401, got %d", rec.Code)
			}
		})
	}
}

func TestAuthenticateRejectsInvalidAndExpiredTokens(t *testing.T) {
	service, _, manager := newMiddlewareTestService(t, newTestUser(t))

	expiredClaims := AccessClaims{
		Role: schema.RoleOSCAAdmin,
		RegisteredClaims: jwt.RegisteredClaims{
			Issuer:    tokenIssuer,
			Subject:   "123e4567-e89b-12d3-a456-426614174000",
			IssuedAt:  jwt.NewNumericDate(time.Now().Add(-2 * time.Hour)),
			ExpiresAt: jwt.NewNumericDate(time.Now().Add(-time.Hour)),
		},
	}

	expiredJWT := jwt.NewWithClaims(jwt.SigningMethodHS256, expiredClaims)
	expiredToken, err := expiredJWT.SignedString(manager.accessSecret)
	if err != nil {
		t.Fatalf("sign expired token: %v", err)
	}

	tests := []struct {
		name  string
		token string
	}{
		{name: "invalid token", token: "not-a-jwt"},
		{name: "expired token", token: expiredToken},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			next := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				t.Fatal("protected handler must not be called")
			})

			req := httptest.NewRequest(http.MethodGet, "/protected", nil)
			req.Header.Set("Authorization", "Bearer "+tt.token)
			rec := httptest.NewRecorder()

			service.Authenticate(next).ServeHTTP(rec, req)

			if rec.Code != http.StatusUnauthorized {
				t.Fatalf("expected 401, got %d", rec.Code)
			}
		})
	}
}

func TestAuthenticateAddsCurrentDatabaseUserToContext(t *testing.T) {
	account := newTestUser(t)
	service, _, manager := newMiddlewareTestService(t, account)

	token, _, err := manager.GenerateAccessToken(
		account.UserID.String(),
		schema.RoleSeniorCitizen,
	)
	if err != nil {
		t.Fatalf("GenerateAccessToken() error: %v", err)
	}

	// Simulate a role change after the access token was issued.
	account.UserRole = schema.RoleOSCAAdmin

	called := false
	next := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		called = true

		user, ok := AuthenticatedUserFromContext(r.Context())
		if !ok {
			t.Fatal("authenticated user missing from request context")
		}

		if user.UserID != account.UserID.String() {
			t.Errorf("expected user ID %q, got %q", account.UserID, user.UserID)
		}

		if user.Role != schema.RoleOSCAAdmin {
			t.Errorf("expected current database role %q, got %q",
				schema.RoleOSCAAdmin, user.Role)
		}

		w.WriteHeader(http.StatusNoContent)
	})

	req := httptest.NewRequest(http.MethodGet, "/protected", nil)
	req.Header.Set("Authorization", "Bearer "+token)
	rec := httptest.NewRecorder()

	service.Authenticate(next).ServeHTTP(rec, req)

	if !called {
		t.Fatal("protected handler was not called")
	}

	if rec.Code != http.StatusNoContent {
		t.Fatalf("expected 204, got %d", rec.Code)
	}
}

func TestAuthenticateRejectsInactiveAccount(t *testing.T) {
	account := newTestUser(t)
	account.IsActive = false

	service, _, manager := newMiddlewareTestService(t, account)

	token, _, err := manager.GenerateAccessToken(
		account.UserID.String(),
		account.UserRole,
	)
	if err != nil {
		t.Fatalf("GenerateAccessToken() error: %v", err)
	}

	req := httptest.NewRequest(http.MethodGet, "/protected", nil)
	req.Header.Set("Authorization", "Bearer "+token)
	rec := httptest.NewRecorder()

	service.Authenticate(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		t.Fatal("inactive account must not reach protected handler")
	})).ServeHTTP(rec, req)

	if rec.Code != http.StatusUnauthorized {
		t.Fatalf("expected 401, got %d", rec.Code)
	}
}

func TestAuthenticateRejectsMissingAccount(t *testing.T) {
	service, _, manager := newMiddlewareTestService(t, nil)

	userID := "123e4567-e89b-12d3-a456-426614174000"
	token, _, err := manager.GenerateAccessToken(userID, schema.RoleSeniorCitizen)
	if err != nil {
		t.Fatalf("GenerateAccessToken() error: %v", err)
	}

	req := httptest.NewRequest(http.MethodGet, "/protected", nil)
	req.Header.Set("Authorization", "Bearer "+token)
	rec := httptest.NewRecorder()

	service.Authenticate(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		t.Fatal("missing account must not reach protected handler")
	})).ServeHTTP(rec, req)

	if rec.Code != http.StatusUnauthorized {
		t.Fatalf("expected 401, got %d", rec.Code)
	}
}

func TestAuthenticateReturnsUnavailableWhenDatabaseLookupFails(t *testing.T) {
	account := newTestUser(t)
	service, repository, manager := newMiddlewareTestService(t, account)
	repository.getUserByIDErr = errors.New("database unavailable")

	token, _, err := manager.GenerateAccessToken(
		account.UserID.String(),
		account.UserRole,
	)
	if err != nil {
		t.Fatalf("GenerateAccessToken() error: %v", err)
	}

	req := httptest.NewRequest(http.MethodGet, "/protected", nil)
	req.Header.Set("Authorization", "Bearer "+token)
	rec := httptest.NewRecorder()

	service.Authenticate(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		t.Fatal("database failure must not reach protected handler")
	})).ServeHTTP(rec, req)

	if rec.Code != http.StatusServiceUnavailable {
		t.Fatalf("expected 503, got %d", rec.Code)
	}
}

func TestAuthenticateRejectsInvalidUserIDInToken(t *testing.T) {
	service, _, manager := newMiddlewareTestService(t, newTestUser(t))

	claims := AccessClaims{
		Role: schema.RoleSeniorCitizen,
		RegisteredClaims: jwt.RegisteredClaims{
			Issuer:    tokenIssuer,
			Subject:   "not-a-uuid",
			IssuedAt:  jwt.NewNumericDate(time.Now()),
			ExpiresAt: jwt.NewNumericDate(time.Now().Add(time.Minute)),
		},
	}

	signed := jwt.NewWithClaims(jwt.SigningMethodHS256, claims)
	token, err := signed.SignedString(manager.accessSecret)
	if err != nil {
		t.Fatalf("sign token: %v", err)
	}

	req := httptest.NewRequest(http.MethodGet, "/protected", nil)
	req.Header.Set("Authorization", "Bearer "+token)
	rec := httptest.NewRecorder()

	service.Authenticate(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		t.Fatal("invalid identity must not reach protected handler")
	})).ServeHTTP(rec, req)

	if rec.Code != http.StatusUnauthorized {
		t.Fatalf("expected 401, got %d", rec.Code)
	}
}

func TestRequireRolesRejectsMissingIdentity(t *testing.T) {
	handler := RequireRoles(schema.RoleOSCAAdmin)(
		http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			t.Fatal("protected handler must not be called")
		}),
	)

	req := httptest.NewRequest(http.MethodGet, "/admin", nil)
	rec := httptest.NewRecorder()

	handler.ServeHTTP(rec, req)

	if rec.Code != http.StatusUnauthorized {
		t.Fatalf("expected 401, got %d", rec.Code)
	}
}

func TestRequireRolesRejectsDisallowedRole(t *testing.T) {
	handler := RequireRoles(schema.RoleOSCAAdmin)(
		http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			t.Fatal("protected handler must not be called")
		}),
	)

	user := AuthenticatedUser{
		UserID: "user-123",
		Role:   schema.RoleSeniorCitizen,
	}

	ctx := context.WithValue(context.Background(), authenticatedUserKey{}, user)
	req := httptest.NewRequest(http.MethodGet, "/admin", nil).WithContext(ctx)
	rec := httptest.NewRecorder()

	handler.ServeHTTP(rec, req)

	if rec.Code != http.StatusForbidden {
		t.Fatalf("expected 403, got %d", rec.Code)
	}
}

func TestRequireRolesAllowsPermittedRole(t *testing.T) {
	handler := RequireRoles(
		schema.RoleOSCAAdmin,
		schema.RoleSuperAdmin,
	)(
		http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			w.WriteHeader(http.StatusNoContent)
		}),
	)

	user := AuthenticatedUser{
		UserID: "user-123",
		Role:   schema.RoleOSCAAdmin,
	}

	ctx := context.WithValue(context.Background(), authenticatedUserKey{}, user)
	req := httptest.NewRequest(http.MethodGet, "/admin", nil).WithContext(ctx)
	rec := httptest.NewRecorder()

	handler.ServeHTTP(rec, req)

	if rec.Code != http.StatusNoContent {
		t.Fatalf("expected 204, got %d", rec.Code)
	}
}

func TestRequireRolesIgnoresInvalidAllowedRoles(t *testing.T) {
	handler := RequireRoles(schema.UserRole("UNKNOWN"))(
		http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			t.Fatal("protected handler must not be called")
		}),
	)

	user := AuthenticatedUser{
		UserID: "user-123",
		Role:   schema.RoleOSCAAdmin,
	}

	ctx := context.WithValue(context.Background(), authenticatedUserKey{}, user)
	req := httptest.NewRequest(http.MethodGet, "/admin", nil).WithContext(ctx)
	rec := httptest.NewRecorder()

	handler.ServeHTTP(rec, req)

	if rec.Code != http.StatusForbidden {
		t.Fatalf("expected 403, got %d", rec.Code)
	}
}

func TestAuthenticateRejectsNilService(t *testing.T) {
	var service *Service

	handler := service.Authenticate(
		http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			t.Fatal("protected handler must not be called")
		}),
	)

	req := httptest.NewRequest(http.MethodGet, "/protected", nil)
	req.Header.Set("Authorization", "Bearer "+strings.Repeat("a", 10))
	rec := httptest.NewRecorder()

	handler.ServeHTTP(rec, req)

	if rec.Code != http.StatusInternalServerError {
		t.Fatalf("expected 500, got %d", rec.Code)
	}
}
