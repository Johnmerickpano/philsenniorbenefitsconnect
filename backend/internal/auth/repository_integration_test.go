package auth

import (
	"context"
	"errors"
	"fmt"
	"os"
	"testing"
	"time"

	"github.com/Johnmerickpano/philsenniorbenefitsconnect/internal/config"
	"github.com/Johnmerickpano/philsenniorbenefitsconnect/internal/database"
	"github.com/Johnmerickpano/philsenniorbenefitsconnect/internal/database/schema"
	"github.com/gocql/gocql"
)

func TestRefreshTokenRepositoryIntegration(t *testing.T) {
	if os.Getenv("PSBC_INTEGRATION_TEST") != "1" {
		t.Skip("set PSBC_INTEGRATION_TEST=1 to run ScyllaDB integration tests")
	}

	cfg, err := config.Load()
	if err != nil {
		t.Fatalf("load configuration: %v", err)
	}

	session, err := database.NewScyllaSession(cfg)
	if err != nil {
		t.Fatalf("connect to ScyllaDB: %v", err)
	}
	t.Cleanup(func() { session.Close() })

	repository, err := NewRepository(session)
	if err != nil {
		t.Fatalf("create repository: %v", err)
	}

	ctx, cancel := context.WithTimeout(context.Background(), 15*time.Second)
	defer cancel()

	tokenHash := fmt.Sprintf("integration-%s", gocql.TimeUUID())
	userID := gocql.TimeUUID()
	familyID := gocql.TimeUUID()

	t.Cleanup(func() {
		cleanupCtx, cleanupCancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cleanupCancel()

		_ = session.Query(
			`DELETE FROM refresh_tokens_by_user
             WHERE user_id = ? AND token_hash = ?`,
			userID,
			tokenHash,
		).WithContext(cleanupCtx).Exec()

		_ = session.Query(
			`DELETE FROM refresh_tokens_by_hash WHERE token_hash = ?`,
			tokenHash,
		).WithContext(cleanupCtx).Exec()
	})

	token := &schema.RefreshToken{
		TokenHash: tokenHash,
		UserID:    userID,
		FamilyID:  familyID,
		IssuedAt:  time.Now().UTC(),
		ExpiresAt: time.Now().UTC().Add(10 * time.Minute),
	}

	if err := repository.StoreRefreshToken(ctx, token); err != nil {
		t.Fatalf("store refresh token: %v", err)
	}

	stored, err := repository.GetRefreshToken(ctx, tokenHash)
	if err != nil {
		t.Fatalf("get stored refresh token: %v", err)
	}

	if stored.UserID != userID || stored.FamilyID != familyID {
		t.Fatal("stored refresh token has unexpected user or family ID")
	}

	var indexedHash string
	err = session.Query(
		`SELECT token_hash FROM refresh_tokens_by_user
         WHERE user_id = ? AND token_hash = ?`,
		userID,
		tokenHash,
	).WithContext(ctx).Scan(&indexedHash)

	if err != nil {
		t.Fatalf("verify refresh token user index: %v", err)
	}

	if indexedHash != tokenHash {
		t.Fatal("refresh token user index contains an unexpected hash")
	}

	if err := repository.RevokeRefreshToken(ctx, tokenHash); err != nil {
		t.Fatalf("revoke refresh token: %v", err)
	}

	_, err = repository.GetRefreshToken(ctx, tokenHash)
	if !errors.Is(err, ErrInvalidToken) {
		t.Fatalf("expected ErrInvalidToken after revocation, got %v", err)
	}

	err = session.Query(
		`SELECT token_hash FROM refresh_tokens_by_user
         WHERE user_id = ? AND token_hash = ?`,
		userID,
		tokenHash,
	).WithContext(ctx).Scan(&indexedHash)

	if !errors.Is(err, gocql.ErrNotFound) {
		t.Fatalf("expected user index to be removed, got error %v", err)
	}
}
