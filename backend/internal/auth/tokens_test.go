package auth

import (
	"strings"
	"testing"
	"time"

	"github.com/Johnmerickpano/philsenniorbenefitsconnect/internal/config"
	"github.com/Johnmerickpano/philsenniorbenefitsconnect/internal/database/schema"
	"github.com/golang-jwt/jwt/v5"
)

func testTokenManager(t *testing.T) *TokenManager {
	t.Helper()

	manager, err := NewTokenManager(config.Config{
		JWTAccessSecret:  strings.Repeat("a", 32),
		JWTRefreshSecret: strings.Repeat("b", 32),
		AccessTokenTTL:   15 * time.Minute,
		RefreshTokenTTL:  7 * 24 * time.Hour,
	})
	if err != nil {
		t.Fatalf("NewTokenManager() error: %v", err)
	}

	return manager
}

func TestNewTokenManagerRejectsInvalidConfiguration(t *testing.T) {
	tests := []struct {
		name string
		cfg  config.Config
	}{
		{
			name: "missing secrets",
			cfg: config.Config{
				AccessTokenTTL:  15 * time.Minute,
				RefreshTokenTTL: 7 * 24 * time.Hour,
			},
		},
		{
			name: "short secrets",
			cfg: config.Config{
				JWTAccessSecret:  "short-access",
				JWTRefreshSecret: "short-refresh",
				AccessTokenTTL:   15 * time.Minute,
				RefreshTokenTTL:  7 * 24 * time.Hour,
			},
		},
		{
			name: "identical secrets",
			cfg: config.Config{
				JWTAccessSecret:  strings.Repeat("a", 32),
				JWTRefreshSecret: strings.Repeat("a", 32),
				AccessTokenTTL:   15 * time.Minute,
				RefreshTokenTTL:  7 * 24 * time.Hour,
			},
		},
		{
			name: "invalid lifetime",
			cfg: config.Config{
				JWTAccessSecret:  strings.Repeat("a", 32),
				JWTRefreshSecret: strings.Repeat("b", 32),
				AccessTokenTTL:   0,
				RefreshTokenTTL:  7 * 24 * time.Hour,
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			_, err := NewTokenManager(tt.cfg)
			if err == nil {
				t.Fatal("expected configuration error")
			}
		})
	}
}

func TestGenerateAndValidateAccessToken(t *testing.T) {
	manager := testTokenManager(t)
	userID := "123e4567-e89b-12d3-a456-426614174000"

	rawToken, expiresAt, err := manager.GenerateAccessToken(
		userID,
		schema.RoleOSCAAdmin,
	)
	if err != nil {
		t.Fatalf("GenerateAccessToken() error: %v", err)
	}

	if rawToken == "" {
		t.Fatal("generated access token is empty")
	}

	if !expiresAt.After(time.Now()) {
		t.Fatal("access token expiration must be in the future")
	}

	claims, err := manager.ValidateAccessToken(rawToken)
	if err != nil {
		t.Fatalf("ValidateAccessToken() error: %v", err)
	}

	if claims.Subject != userID {
		t.Errorf("expected subject %q, got %q", userID, claims.Subject)
	}

	if claims.Role != schema.RoleOSCAAdmin {
		t.Errorf("expected role %q, got %q", schema.RoleOSCAAdmin, claims.Role)
	}

	if claims.Issuer != tokenIssuer {
		t.Errorf("expected issuer %q, got %q", tokenIssuer, claims.Issuer)
	}
}

func TestGenerateAccessTokenRejectsInvalidClaims(t *testing.T) {
	manager := testTokenManager(t)

	tests := []struct {
		name   string
		userID string
		role   schema.UserRole
	}{
		{
			name:   "empty user ID",
			userID: "",
			role:   schema.RoleOSCAAdmin,
		},
		{
			name:   "invalid role",
			userID: "user-123",
			role:   schema.UserRole("UNKNOWN"),
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			_, _, err := manager.GenerateAccessToken(tt.userID, tt.role)
			if err == nil {
				t.Fatal("expected invalid claims error")
			}
		})
	}
}

func TestValidateAccessTokenRejectsTampering(t *testing.T) {
	manager := testTokenManager(t)

	rawToken, _, err := manager.GenerateAccessToken(
		"user-123",
		schema.RoleOSCAStaff,
	)
	if err != nil {
		t.Fatalf("GenerateAccessToken() error: %v", err)
	}

	parts := strings.Split(rawToken, ".")
	if len(parts) != 3 {
		t.Fatal("expected three JWT segments")
	}

	tamperedToken := parts[0] + "." + parts[1] + ".invalidsignature"

	if _, err := manager.ValidateAccessToken(tamperedToken); err == nil {
		t.Fatal("accepted tampered access token")
	}
}

func TestValidateAccessTokenRejectsWrongSecret(t *testing.T) {
	manager := testTokenManager(t)

	rawToken, _, err := manager.GenerateAccessToken(
		"user-123",
		schema.RoleSeniorCitizen,
	)
	if err != nil {
		t.Fatalf("GenerateAccessToken() error: %v", err)
	}

	otherManager, err := NewTokenManager(config.Config{
		JWTAccessSecret:  strings.Repeat("c", 32),
		JWTRefreshSecret: strings.Repeat("d", 32),
		AccessTokenTTL:   15 * time.Minute,
		RefreshTokenTTL:  7 * 24 * time.Hour,
	})
	if err != nil {
		t.Fatalf("NewTokenManager() error: %v", err)
	}

	if _, err := otherManager.ValidateAccessToken(rawToken); err == nil {
		t.Fatal("accepted token signed with another secret")
	}
}

func TestValidateAccessTokenRejectsExpiredToken(t *testing.T) {
	manager := testTokenManager(t)

	expiredClaims := AccessClaims{
		Role: schema.RoleOSCAAdmin,
		RegisteredClaims: jwt.RegisteredClaims{
			Issuer:    tokenIssuer,
			Subject:   "user-123",
			IssuedAt:  jwt.NewNumericDate(time.Now().Add(-2 * time.Hour)),
			ExpiresAt: jwt.NewNumericDate(time.Now().Add(-time.Hour)),
		},
	}

	token := jwt.NewWithClaims(jwt.SigningMethodHS256, expiredClaims)
	expiredToken, err := token.SignedString(manager.accessSecret)
	if err != nil {
		t.Fatalf("SignedString() error: %v", err)
	}

	if _, err := manager.ValidateAccessToken(expiredToken); err == nil {
		t.Fatal("accepted expired access token")
	}
}

func TestGenerateRefreshToken(t *testing.T) {
	manager := testTokenManager(t)

	rawToken, tokenHash, err := manager.GenerateRefreshToken()
	if err != nil {
		t.Fatalf("GenerateRefreshToken() error: %v", err)
	}

	if rawToken == "" || tokenHash == "" {
		t.Fatal("refresh token and hash must not be empty")
	}

	if rawToken == tokenHash {
		t.Fatal("refresh token must not equal its stored hash")
	}

	if manager.HashRefreshToken(rawToken) != tokenHash {
		t.Fatal("refresh token hash is not reproducible")
	}
}

func TestGenerateRefreshTokenProducesUniqueTokens(t *testing.T) {
	manager := testTokenManager(t)

	firstToken, firstHash, err := manager.GenerateRefreshToken()
	if err != nil {
		t.Fatalf("first GenerateRefreshToken() error: %v", err)
	}

	secondToken, secondHash, err := manager.GenerateRefreshToken()
	if err != nil {
		t.Fatalf("second GenerateRefreshToken() error: %v", err)
	}

	if firstToken == secondToken {
		t.Fatal("refresh tokens must be unique")
	}

	if firstHash == secondHash {
		t.Fatal("refresh token hashes must be unique")
	}
}

func TestRefreshTokenHashUsesSeparateSecret(t *testing.T) {
	manager := testTokenManager(t)

	otherManager, err := NewTokenManager(config.Config{
		JWTAccessSecret:  strings.Repeat("a", 32),
		JWTRefreshSecret: strings.Repeat("c", 32),
		AccessTokenTTL:   15 * time.Minute,
		RefreshTokenTTL:  7 * 24 * time.Hour,
	})
	if err != nil {
		t.Fatalf("NewTokenManager() error: %v", err)
	}

	rawToken := "sample-refresh-token"

	if manager.HashRefreshToken(rawToken) == otherManager.HashRefreshToken(rawToken) {
		t.Fatal("different refresh secrets should produce different hashes")
	}
}

func TestRefreshTokenExpiresAt(t *testing.T) {
	manager := testTokenManager(t)

	expiresAt := manager.RefreshTokenExpiresAt()

	if !expiresAt.After(time.Now()) {
		t.Fatal("refresh token expiration must be in the future")
	}

	expected := 7 * 24 * time.Hour
	actual := time.Until(expiresAt)

	if actual < expected-time.Second || actual > expected+time.Second {
		t.Fatalf("unexpected refresh token lifetime: %v", actual)
	}
}

func TestValidateAccessTokenRejectsEmptyToken(t *testing.T) {
	manager := testTokenManager(t)

	if _, err := manager.ValidateAccessToken(""); err == nil {
		t.Fatal("accepted empty access token")
	}
}
