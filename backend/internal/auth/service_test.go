package auth

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/Johnmerickpano/philsenniorbenefitsconnect/internal/config"
	"github.com/Johnmerickpano/philsenniorbenefitsconnect/internal/database/schema"
	"github.com/gocql/gocql"
)

type fakeAuthRepository struct {
	user             *schema.User
	getUserErr       error
	failedCount      int32
	lockedUntil      *time.Time
	loginFailureCall int
	resetCall        int
	storedToken      *schema.RefreshToken
	storeContext     context.Context
	storeErr         error
	refreshToken     *schema.RefreshToken
	getRefreshErr    error
	getUserByIDErr   error
	rotateCall       int
	rotatedOldHash   string
	rotatedToken     *schema.RefreshToken
	rotateErr        error
}

func (f *fakeAuthRepository) GetUserByUsername(
	ctx context.Context,
	username string,
) (*schema.User, error) {
	if f.getUserErr != nil {
		return nil, f.getUserErr
	}
	return f.user, nil
}

func (f *fakeAuthRepository) UpdateLoginFailure(
	ctx context.Context,
	userID gocql.UUID,
	failedCount int32,
	lockedUntil *time.Time,
) error {
	f.loginFailureCall++
	f.failedCount = failedCount
	f.lockedUntil = lockedUntil
	return nil
}

func (f *fakeAuthRepository) ResetLoginFailures(
	ctx context.Context,
	userID gocql.UUID,
) error {
	f.resetCall++
	return nil
}

func (f *fakeAuthRepository) StoreRefreshToken(
	ctx context.Context,
	token *schema.RefreshToken,
) error {
	f.storeContext = ctx
	if f.storeErr != nil {
		return f.storeErr
	}
	f.storedToken = token
	return nil
}

func (f *fakeAuthRepository) GetUserByID(
	ctx context.Context,
	userID gocql.UUID,
) (*schema.User, error) {
	if f.getUserByIDErr != nil {
		return nil, f.getUserByIDErr
	}
	if f.user == nil || f.user.UserID != userID {
		return nil, ErrUserNotFound
	}
	return f.user, nil
}

func (f *fakeAuthRepository) GetRefreshToken(
	ctx context.Context,
	tokenHash string,
) (*schema.RefreshToken, error) {
	if f.getRefreshErr != nil {
		return nil, f.getRefreshErr
	}
	if f.refreshToken == nil || f.refreshToken.TokenHash != tokenHash {
		return nil, ErrInvalidToken
	}
	return f.refreshToken, nil
}

func (f *fakeAuthRepository) RotateRefreshToken(
	ctx context.Context,
	oldHash string,
	replacement *schema.RefreshToken,
) error {
	f.rotateCall++
	f.rotatedOldHash = oldHash
	f.rotatedToken = replacement
	return f.rotateErr
}

type serviceTestContextKey struct{}

func testServiceConfig() config.Config {
	return config.Config{
		JWTAccessSecret:  "test-access-secret-0123456789-abcdefghijkl",
		JWTRefreshSecret: "test-refresh-secret-9876543210-zyxwvuts",
		AccessTokenTTL:   15 * time.Minute,
		RefreshTokenTTL:  168 * time.Hour,
		MaxFailedLogins:  5,
		LockDuration:     15 * time.Minute,
	}
}

func newTestUser(t *testing.T) *schema.User {
	t.Helper()

	hash, err := HashPassword("CorrectPassword@2026")
	if err != nil {
		t.Fatalf("HashPassword() error: %v", err)
	}

	return &schema.User{
		UserID:           gocql.TimeUUID(),
		Username:         "test-senior",
		PasswordHash:     hash,
		UserRole:         "SENIOR_CITIZEN",
		FullName:         "Test Senior",
		IsActive:         true,
		FailedLoginCount: 0,
	}
}

func TestLoginSuccess(t *testing.T) {
	repository := &fakeAuthRepository{
		user: newTestUser(t),
	}

	service, err := NewService(repository, testServiceConfig())
	if err != nil {
		t.Fatalf("NewService() error: %v", err)
	}

	ctx := context.WithValue(
		context.Background(),
		serviceTestContextKey{},
		"request-context",
	)

	result, err := service.Login(ctx, "test-senior", "CorrectPassword@2026")
	if err != nil {
		t.Fatalf("Login() error: %v", err)
	}

	if result == nil {
		t.Fatal("Login() returned a nil result")
	}
	if result.AccessToken == "" || result.RefreshToken == "" {
		t.Fatal("Login() returned an empty token")
	}
	if result.TokenType != "Bearer" {
		t.Fatalf("TokenType = %q, want Bearer", result.TokenType)
	}
	if result.User == nil || result.User.Username != "test-senior" {
		t.Fatal("Login() returned the wrong user")
	}
	if repository.resetCall != 1 {
		t.Fatalf("ResetLoginFailures() calls = %d, want 1", repository.resetCall)
	}
	if repository.storedToken == nil {
		t.Fatal("Login() did not store a refresh token")
	}
	if repository.storeContext != ctx {
		t.Fatal("refresh-token storage did not receive the request context")
	}
	if repository.storeContext.Value(serviceTestContextKey{}) != "request-context" {
		t.Fatal("request context value was not propagated")
	}
}

func TestLoginInvalidPassword(t *testing.T) {
	repository := &fakeAuthRepository{
		user: newTestUser(t),
	}

	service, err := NewService(repository, testServiceConfig())
	if err != nil {
		t.Fatalf("NewService() error: %v", err)
	}

	_, err = service.Login(context.Background(), "test-senior", "WrongPassword@2026")
	if !errors.Is(err, ErrInvalidCredentials) {
		t.Fatalf("Login() error = %v, want ErrInvalidCredentials", err)
	}
	if repository.loginFailureCall != 1 {
		t.Fatalf("UpdateLoginFailure() calls = %d, want 1", repository.loginFailureCall)
	}
	if repository.failedCount != 1 {
		t.Fatalf("failed login count = %d, want 1", repository.failedCount)
	}
	if repository.resetCall != 0 {
		t.Fatal("ResetLoginFailures() should not be called")
	}
}

func TestLoginLocksAccountAtFailureLimit(t *testing.T) {
	repository := &fakeAuthRepository{
		user: newTestUser(t),
	}

	cfg := testServiceConfig()
	cfg.MaxFailedLogins = 1

	service, err := NewService(repository, cfg)
	if err != nil {
		t.Fatalf("NewService() error: %v", err)
	}

	_, err = service.Login(context.Background(), "test-senior", "WrongPassword@2026")
	if !errors.Is(err, ErrAccountLocked) {
		t.Fatalf("Login() error = %v, want ErrAccountLocked", err)
	}
	if repository.lockedUntil == nil || !repository.lockedUntil.After(time.Now()) {
		t.Fatal("account lock expiration was not set in the future")
	}
}

func TestLoginRejectsInactiveAccount(t *testing.T) {
	user := newTestUser(t)
	user.IsActive = false

	repository := &fakeAuthRepository{user: user}
	service, err := NewService(repository, testServiceConfig())
	if err != nil {
		t.Fatalf("NewService() error: %v", err)
	}

	_, err = service.Login(context.Background(), "test-senior", "CorrectPassword@2026")
	if !errors.Is(err, ErrAccountInactive) {
		t.Fatalf("Login() error = %v, want ErrAccountInactive", err)
	}
	if repository.loginFailureCall != 0 || repository.resetCall != 0 {
		t.Fatal("inactive account should not update login counters")
	}
}

func TestLoginRejectsAlreadyLockedAccount(t *testing.T) {
	user := newTestUser(t)
	lockExpiration := time.Now().Add(10 * time.Minute)
	user.LockedUntil = &lockExpiration

	repository := &fakeAuthRepository{user: user}
	service, err := NewService(repository, testServiceConfig())
	if err != nil {
		t.Fatalf("NewService() error: %v", err)
	}

	_, err = service.Login(context.Background(), "test-senior", "CorrectPassword@2026")
	if !errors.Is(err, ErrAccountLocked) {
		t.Fatalf("Login() error = %v, want ErrAccountLocked", err)
	}
	if repository.resetCall != 0 {
		t.Fatal("locked account should not reset login failures")
	}
}

func TestLoginMapsUnknownUsernameToInvalidCredentials(t *testing.T) {
	repository := &fakeAuthRepository{
		getUserErr: ErrUserNotFound,
	}

	service, err := NewService(repository, testServiceConfig())
	if err != nil {
		t.Fatalf("NewService() error: %v", err)
	}

	_, err = service.Login(context.Background(), "unknown-user", "AnyPassword@2026")
	if !errors.Is(err, ErrInvalidCredentials) {
		t.Fatalf("Login() error = %v, want ErrInvalidCredentials", err)
	}
}
