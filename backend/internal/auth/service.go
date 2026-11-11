package auth

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"time"

	"github.com/Johnmerickpano/philsenniorbenefitsconnect/internal/config"
	"github.com/Johnmerickpano/philsenniorbenefitsconnect/internal/database/schema"
	"github.com/gocql/gocql"
)

var (
	ErrInvalidCredentials = errors.New("invalid username or password")
	ErrAccountLocked      = errors.New("account is temporarily locked")
	ErrAccountInactive    = errors.New("account is inactive")
)

type authRepository interface {
	GetUserByUsername(ctx context.Context, username string) (*schema.User, error)
	GetUserByID(ctx context.Context, userID gocql.UUID) (*schema.User, error)
	GetRefreshToken(ctx context.Context, tokenHash string) (*schema.RefreshToken, error)
	UpdateLoginFailure(ctx context.Context, userID gocql.UUID, failedCount int32, lockedUntil *time.Time) error
	ResetLoginFailures(ctx context.Context, userID gocql.UUID) error
	StoreRefreshToken(ctx context.Context, token *schema.RefreshToken) error
	RotateRefreshToken(ctx context.Context, oldHash string, replacement *schema.RefreshToken) error
}

type Service struct {
	repository authRepository
	tokens     *TokenManager
	config     config.Config
}

type LoginResult struct {
	User             *schema.User `json:"user"`
	AccessToken      string       `json:"access_token"`
	RefreshToken     string       `json:"refresh_token"`
	TokenType        string       `json:"token_type"`
	AccessExpiresAt  time.Time    `json:"access_expires_at"`
	RefreshExpiresAt time.Time    `json:"refresh_expires_at"`
}

func NewService(repository authRepository, cfg config.Config) (*Service, error) {
	if repository == nil {
		return nil, errors.New("authentication repository is required")
	}

	tokenManager, err := NewTokenManager(cfg)
	if err != nil {
		return nil, fmt.Errorf("initialize token manager: %w", err)
	}

	if cfg.MaxFailedLogins < 1 || cfg.LockDuration <= 0 {
		return nil, errors.New("invalid authentication lockout configuration")
	}

	return &Service{
		repository: repository,
		tokens:     tokenManager,
		config:     cfg,
	}, nil
}

func (s *Service) Login(
	ctx context.Context,
	username string,
	password string,
) (*LoginResult, error) {
	if s == nil || s.repository == nil || s.tokens == nil {
		return nil, errors.New("authentication service is not initialized")
	}

	username = strings.TrimSpace(username)
	if username == "" || password == "" {
		return nil, ErrInvalidCredentials
	}

	user, err := s.repository.GetUserByUsername(ctx, username)
	if err != nil {
		if errors.Is(err, ErrUserNotFound) {
			return nil, ErrInvalidCredentials
		}
		return nil, fmt.Errorf("load login account: %w", err)
	}

	if !user.IsActive {
		return nil, ErrAccountInactive
	}

	now := time.Now().UTC()

	if user.LockedUntil != nil && user.LockedUntil.After(now) {
		return nil, ErrAccountLocked
	}

	if !VerifyPassword(password, user.PasswordHash) {
		failedCount := user.FailedLoginCount + 1
		var lockedUntil *time.Time

		if int(failedCount) >= s.config.MaxFailedLogins {
			lockTime := now.Add(s.config.LockDuration)
			lockedUntil = &lockTime
		}

		if err := s.repository.UpdateLoginFailure(
			ctx,
			user.UserID,
			failedCount,
			lockedUntil,
		); err != nil {
			return nil, fmt.Errorf("record failed login: %w", err)
		}

		if lockedUntil != nil {
			return nil, ErrAccountLocked
		}

		return nil, ErrInvalidCredentials
	}

	if err := s.repository.ResetLoginFailures(ctx, user.UserID); err != nil {
		return nil, fmt.Errorf("reset login failures: %w", err)
	}

	result, err := s.issueLoginTokens(ctx, user)
	if err != nil {
		return nil, fmt.Errorf("issue login tokens: %w", err)
	}

	return result, nil
}

func (s *Service) issueLoginTokens(ctx context.Context, user *schema.User) (*LoginResult, error) {
	if user == nil || user.UserID == (gocql.UUID{}) {
		return nil, ErrInvalidUser
	}

	accessToken, accessExpiresAt, err := s.tokens.GenerateAccessToken(
		user.UserID.String(),
		user.UserRole,
	)
	if err != nil {
		return nil, err
	}

	refreshToken, refreshHash, err := s.tokens.GenerateRefreshToken()
	if err != nil {
		return nil, err
	}

	refreshExpiresAt := s.tokens.RefreshTokenExpiresAt()
	familyID := gocql.TimeUUID()

	record := &schema.RefreshToken{
		TokenHash: refreshHash,
		UserID:    user.UserID,
		FamilyID:  familyID,
		IssuedAt:  time.Now().UTC(),
		ExpiresAt: refreshExpiresAt,
		Revoked:   false,
	}

	if err := s.repository.StoreRefreshToken(
		ctx,
		record,
	); err != nil {
		return nil, err
	}

	return &LoginResult{
		User:             user,
		AccessToken:      accessToken,
		RefreshToken:     refreshToken,
		TokenType:        "Bearer",
		AccessExpiresAt:  accessExpiresAt,
		RefreshExpiresAt: refreshExpiresAt,
	}, nil
}

// Refresh validates and rotates a refresh token while preserving its token family.
func (s *Service) Refresh(
	ctx context.Context,
	rawRefreshToken string,
) (*LoginResult, error) {
	if s == nil || s.repository == nil || s.tokens == nil {
		return nil, errors.New("authentication service is not initialized")
	}

	rawRefreshToken = strings.TrimSpace(rawRefreshToken)
	if rawRefreshToken == "" {
		return nil, ErrInvalidToken
	}

	oldHash := s.tokens.HashRefreshToken(rawRefreshToken)

	current, err := s.repository.GetRefreshToken(ctx, oldHash)
	if err != nil {
		if errors.Is(err, ErrInvalidToken) {
			return nil, ErrInvalidToken
		}
		return nil, fmt.Errorf("get refresh token: %w", err)
	}

	now := time.Now().UTC()

	if current == nil ||
		current.Revoked ||
		current.TokenHash != oldHash ||
		current.UserID == (gocql.UUID{}) ||
		current.FamilyID == (gocql.UUID{}) ||
		!current.ExpiresAt.After(now) {
		return nil, ErrInvalidToken
	}

	user, err := s.repository.GetUserByID(ctx, current.UserID)
	if err != nil {
		if errors.Is(err, ErrUserNotFound) {
			return nil, ErrInvalidToken
		}
		return nil, fmt.Errorf("get refresh-token user: %w", err)
	}

	if user == nil || user.UserID != current.UserID {
		return nil, ErrInvalidToken
	}

	if !user.IsActive {
		return nil, ErrAccountInactive
	}

	// Recheck expiry immediately before generating the replacement.
	now = time.Now().UTC()
	if !current.ExpiresAt.After(now) {
		return nil, ErrInvalidToken
	}

	accessToken, accessExpiresAt, err := s.tokens.GenerateAccessToken(
		user.UserID.String(),
		user.UserRole,
	)
	if err != nil {
		return nil, fmt.Errorf("generate access token: %w", err)
	}

	replacementRaw, replacementHash, err := s.tokens.GenerateRefreshToken()
	if err != nil {
		return nil, fmt.Errorf("generate replacement refresh token: %w", err)
	}

	refreshExpiresAt := s.tokens.RefreshTokenExpiresAt()

	replacement := &schema.RefreshToken{
		TokenHash: replacementHash,
		UserID:    current.UserID,
		FamilyID:  current.FamilyID,
		IssuedAt:  now,
		ExpiresAt: refreshExpiresAt,
		Revoked:   false,
	}

	if err := s.repository.RotateRefreshToken(
		ctx,
		oldHash,
		replacement,
	); err != nil {
		if errors.Is(err, ErrInvalidToken) {
			return nil, ErrInvalidToken
		}
		return nil, fmt.Errorf("rotate refresh token: %w", err)
	}

	return &LoginResult{
		User:             user,
		AccessToken:      accessToken,
		RefreshToken:     replacementRaw,
		TokenType:        "Bearer",
		AccessExpiresAt:  accessExpiresAt,
		RefreshExpiresAt: refreshExpiresAt,
	}, nil
}
