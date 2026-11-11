package auth

import (
	"crypto/hmac"
	"crypto/rand"
	"crypto/sha256"
	"encoding/base64"
	"errors"
	"fmt"
	"strings"
	"time"

	"github.com/Johnmerickpano/philsenniorbenefitsconnect/internal/config"
	"github.com/Johnmerickpano/philsenniorbenefitsconnect/internal/database/schema"
	"github.com/golang-jwt/jwt/v5"
)

const tokenIssuer = "phil-senior-benefits-connect"

var (
	ErrInvalidToken = errors.New("invalid or expired token")
	ErrTokenConfig  = errors.New("invalid token manager configuration")
)

type TokenManager struct {
	accessSecret  []byte
	refreshSecret []byte
	accessTTL     time.Duration
	refreshTTL    time.Duration
}

type AccessClaims struct {
	Role schema.UserRole `json:"role"`
	jwt.RegisteredClaims
}

func NewTokenManager(cfg config.Config) (*TokenManager, error) {
	if len(cfg.JWTAccessSecret) < 32 ||
		len(cfg.JWTRefreshSecret) < 32 {
		return nil, fmt.Errorf("%w: JWT secrets must each contain at least 32 bytes", ErrTokenConfig)
	}

	if cfg.JWTAccessSecret == cfg.JWTRefreshSecret {
		return nil, fmt.Errorf("%w: JWT secrets must be different", ErrTokenConfig)
	}

	if cfg.AccessTokenTTL <= 0 || cfg.RefreshTokenTTL <= 0 {
		return nil, fmt.Errorf("%w: token lifetimes must be positive", ErrTokenConfig)
	}

	return &TokenManager{
		accessSecret:  []byte(cfg.JWTAccessSecret),
		refreshSecret: []byte(cfg.JWTRefreshSecret),
		accessTTL:     cfg.AccessTokenTTL,
		refreshTTL:    cfg.RefreshTokenTTL,
	}, nil
}

func (m *TokenManager) GenerateAccessToken(
	userID string,
	role schema.UserRole,
) (string, time.Time, error) {
	if m == nil || len(m.accessSecret) == 0 {
		return "", time.Time{}, ErrTokenConfig
	}

	userID = strings.TrimSpace(userID)
	if userID == "" || !isValidRole(role) {
		return "", time.Time{}, ErrInvalidToken
	}

	now := time.Now().UTC()
	expiresAt := now.Add(m.accessTTL)

	claims := AccessClaims{
		Role: role,
		RegisteredClaims: jwt.RegisteredClaims{
			Issuer:    tokenIssuer,
			Subject:   userID,
			IssuedAt:  jwt.NewNumericDate(now),
			ExpiresAt: jwt.NewNumericDate(expiresAt),
		},
	}

	token := jwt.NewWithClaims(jwt.SigningMethodHS256, claims)

	signedToken, err := token.SignedString(m.accessSecret)
	if err != nil {
		return "", time.Time{}, fmt.Errorf("sign access token: %w", err)
	}

	return signedToken, expiresAt, nil
}

func (m *TokenManager) ValidateAccessToken(
	rawToken string,
) (*AccessClaims, error) {
	if m == nil || len(m.accessSecret) == 0 {
		return nil, ErrTokenConfig
	}

	if strings.TrimSpace(rawToken) == "" {
		return nil, ErrInvalidToken
	}

	claims := &AccessClaims{}

	token, err := jwt.ParseWithClaims(
		rawToken,
		claims,
		func(token *jwt.Token) (interface{}, error) {
			if token.Method != jwt.SigningMethodHS256 {
				return nil, ErrInvalidToken
			}
			return m.accessSecret, nil
		},
		jwt.WithValidMethods([]string{jwt.SigningMethodHS256.Alg()}),
		jwt.WithIssuer(tokenIssuer),
		jwt.WithExpirationRequired(),
	)
	if err != nil || token == nil || !token.Valid {
		return nil, ErrInvalidToken
	}

	if strings.TrimSpace(claims.Subject) == "" || !isValidRole(claims.Role) {
		return nil, ErrInvalidToken
	}

	return claims, nil
}

func (m *TokenManager) GenerateRefreshToken() (
	rawToken string,
	tokenHash string,
	err error,
) {
	if m == nil || len(m.refreshSecret) == 0 {
		return "", "", ErrTokenConfig
	}

	randomBytes := make([]byte, 32)
	if _, err := rand.Read(randomBytes); err != nil {
		return "", "", fmt.Errorf("generate refresh token: %w", err)
	}

	rawToken = base64.RawURLEncoding.EncodeToString(randomBytes)
	tokenHash = m.HashRefreshToken(rawToken)

	return rawToken, tokenHash, nil
}

func (m *TokenManager) HashRefreshToken(rawToken string) string {
	if m == nil || len(m.refreshSecret) == 0 || rawToken == "" {
		return ""
	}

	mac := hmac.New(sha256.New, m.refreshSecret)
	_, _ = mac.Write([]byte(rawToken))

	return base64.RawURLEncoding.EncodeToString(mac.Sum(nil))
}

func (m *TokenManager) RefreshTokenExpiresAt() time.Time {
	if m == nil || m.refreshTTL <= 0 {
		return time.Time{}
	}

	return time.Now().UTC().Add(m.refreshTTL)
}

func isValidRole(role schema.UserRole) bool {
	switch role {
	case schema.RoleSuperAdmin,
		schema.RoleOSCAAdmin,
		schema.RoleOSCAStaff,
		schema.RoleSeniorCitizen,
		schema.RoleAuthorizedRepresentative:
		return true
	default:
		return false
	}
}
