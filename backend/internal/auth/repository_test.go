package auth

import (
	"context"
	"errors"
	"testing"

	"github.com/gocql/gocql"
)

func TestNewRepositoryRequiresSession(t *testing.T) {
	repository, err := NewRepository(nil)

	if err == nil {
		t.Fatal("NewRepository(nil) expected an error")
	}

	if repository != nil {
		t.Fatal("NewRepository(nil) expected a nil repository")
	}
}

func TestNewRepositoryAcceptsSession(t *testing.T) {
	session := &gocql.Session{}

	repository, err := NewRepository(session)
	if err != nil {
		t.Fatalf("NewRepository() error: %v", err)
	}

	if repository == nil {
		t.Fatal("NewRepository() returned a nil repository")
	}

	if repository.session != session {
		t.Fatal("repository does not contain the supplied session")
	}
}

func TestUninitializedRepositoryRejectsOperations(t *testing.T) {
	ctx := context.Background()
	var repository *Repository

	if err := repository.CreateUser(ctx, nil); err == nil {
		t.Fatal("CreateUser() expected an initialization error")
	}

	if _, err := repository.GetUserByUsername(ctx, "testuser"); err == nil {
		t.Fatal("GetUserByUsername() expected an initialization error")
	}

	if _, err := repository.GetUserByID(ctx, gocql.UUID{}); err == nil {
		t.Fatal("GetUserByID() expected an initialization error")
	}
}

func TestRepositoryRejectsInvalidUpdateInputs(t *testing.T) {
	ctx := context.Background()
	repository := &Repository{session: &gocql.Session{}}

	validID := gocql.TimeUUID()

	tests := []struct {
		name string
		run  func() error
	}{
		{
			name: "login failure with zero user ID",
			run: func() error {
				return repository.UpdateLoginFailure(ctx, gocql.UUID{}, 1, nil)
			},
		},
		{
			name: "login failure with negative count",
			run: func() error {
				return repository.UpdateLoginFailure(ctx, validID, -1, nil)
			},
		},
		{
			name: "reset with zero user ID",
			run: func() error {
				return repository.ResetLoginFailures(ctx, gocql.UUID{})
			},
		},
		{
			name: "password update with zero user ID",
			run: func() error {
				return repository.UpdatePassword(ctx, gocql.UUID{}, "valid-hash")
			},
		},
		{
			name: "password update with empty hash",
			run: func() error {
				return repository.UpdatePassword(ctx, validID, " ")
			},
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			err := test.run()
			if !errors.Is(err, ErrInvalidUser) {
				t.Fatalf("expected ErrInvalidUser, got %v", err)
			}
		})
	}
}
