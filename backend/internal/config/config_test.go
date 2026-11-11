package config

import (
	"testing"
	"time"
)

func clearConfigEnv(t *testing.T) {
	t.Helper()

	keys := []string{
		"APP_ENV",
		"HTTP_ADDR",
		"SCYLLA_HOSTS",
		"SCYLLA_PORT",
		"SCYLLA_KEYSPACE",
		"SCYLLA_USERNAME",
		"SCYLLA_PASSWORD",
		"SCYLLA_CONNECT_TIMEOUT",
		"SCYLLA_NUM_CONNS",
		"JWT_ACCESS_SECRET",
		"JWT_REFRESH_SECRET",
		"ACCESS_TOKEN_TTL_MINUTES",
		"REFRESH_TOKEN_TTL_HOURS",
		"AUTH_MAX_FAILED_LOGINS",
		"AUTH_LOCK_DURATION",
		"AUTH_ACTIVATION_CODE_TTL",
	}

	for _, key := range keys {
		t.Setenv(key, "")
	}
}

func TestLoadDefaults(t *testing.T) {
	clearConfigEnv(t)

	cfg, err := Load()
	if err != nil {
		t.Fatalf("Load() returned error: %v", err)
	}

	if cfg.HTTPAddr != ":8080" {
		t.Errorf("HTTPAddr = %q, want :8080", cfg.HTTPAddr)
	}
	if cfg.ScyllaPort != 9042 {
		t.Errorf("ScyllaPort = %d, want 9042", cfg.ScyllaPort)
	}
	if cfg.AccessTokenTTL != 15*time.Minute {
		t.Errorf("AccessTokenTTL = %v, want 15m", cfg.AccessTokenTTL)
	}
	if cfg.RefreshTokenTTL != 168*time.Hour {
		t.Errorf("RefreshTokenTTL = %v, want 168h", cfg.RefreshTokenTTL)
	}
	if cfg.MaxFailedLogins != 5 {
		t.Errorf("MaxFailedLogins = %d, want 5", cfg.MaxFailedLogins)
	}
	if cfg.LockDuration != 15*time.Minute {
		t.Errorf("LockDuration = %v, want 15m", cfg.LockDuration)
	}
	if cfg.ActivationCodeTTL != 15*time.Minute {
		t.Errorf("ActivationCodeTTL = %v, want 15m", cfg.ActivationCodeTTL)
	}
}

func TestLoadAuthenticationSecrets(t *testing.T) {
	clearConfigEnv(t)

	access := "0123456789abcdef0123456789abcdef"
	refresh := "abcdef0123456789abcdef0123456789"

	t.Setenv("JWT_ACCESS_SECRET", access)
	t.Setenv("JWT_REFRESH_SECRET", refresh)

	cfg, err := Load()
	if err != nil {
		t.Fatalf("Load() returned error: %v", err)
	}

	if cfg.JWTAccessSecret != access {
		t.Error("JWT access secret was not loaded correctly")
	}
	if cfg.JWTRefreshSecret != refresh {
		t.Error("JWT refresh secret was not loaded correctly")
	}
}

func TestLoadRejectsIncompleteSecrets(t *testing.T) {
	clearConfigEnv(t)
	t.Setenv("JWT_ACCESS_SECRET", "0123456789abcdef0123456789abcdef")

	if _, err := Load(); err == nil {
		t.Fatal("Load() accepted an incomplete JWT secret configuration")
	}
}

func TestLoadRejectsShortSecrets(t *testing.T) {
	clearConfigEnv(t)
	t.Setenv("JWT_ACCESS_SECRET", "short-access-secret")
	t.Setenv("JWT_REFRESH_SECRET", "short-refresh-secret")

	if _, err := Load(); err == nil {
		t.Fatal("Load() accepted short JWT secrets")
	}
}

func TestLoadRejectsIdenticalSecrets(t *testing.T) {
	clearConfigEnv(t)

	secret := "0123456789abcdef0123456789abcdef"
	t.Setenv("JWT_ACCESS_SECRET", secret)
	t.Setenv("JWT_REFRESH_SECRET", secret)

	if _, err := Load(); err == nil {
		t.Fatal("Load() accepted identical JWT secrets")
	}
}

func TestLoadRejectsInvalidTokenTTL(t *testing.T) {
	clearConfigEnv(t)
	t.Setenv("ACCESS_TOKEN_TTL_MINUTES", "0")

	if _, err := Load(); err == nil {
		t.Fatal("Load() accepted a zero access token lifetime")
	}
}
