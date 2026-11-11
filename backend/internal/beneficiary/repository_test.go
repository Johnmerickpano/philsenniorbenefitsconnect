package beneficiary

import (
	"context"
	"errors"
	"testing"

	"github.com/Johnmerickpano/philsenniorbenefitsconnect/internal/database/schema"
	"github.com/gocql/gocql"
)

func newTestRepository(t *testing.T) *Repository {
	t.Helper()

	repository, err := NewRepository(&gocql.Session{})
	if err != nil {
		t.Fatalf("NewRepository() error = %v", err)
	}

	return repository
}

func TestNewRepository(t *testing.T) {
	if _, err := NewRepository(nil); err == nil {
		t.Fatal("NewRepository(nil) expected an error")
	}

	repository := newTestRepository(t)
	if repository == nil {
		t.Fatal("NewRepository() returned nil repository")
	}
}

func TestRepositoryRejectsUninitializedReceiver(t *testing.T) {
	ctx := context.Background()
	var repository *Repository

	if err := repository.Create(ctx, nil); err == nil {
		t.Fatal("Create() expected an initialization error")
	}

	if _, err := repository.GetByID(ctx, gocql.TimeUUID()); err == nil {
		t.Fatal("GetByID() expected an initialization error")
	}

	if _, err := repository.GetByOSCAID(ctx, "OSCA-TEST"); err == nil {
		t.Fatal("GetByOSCAID() expected an initialization error")
	}
}

func TestCreateRejectsNilBeneficiary(t *testing.T) {
	repository := newTestRepository(t)

	err := repository.Create(context.Background(), nil)
	if !errors.Is(err, ErrInvalidBeneficiary) {
		t.Fatalf("Create(nil) error = %v, want %v", err, ErrInvalidBeneficiary)
	}
}

func TestCreateRejectsInvalidStatus(t *testing.T) {
	repository := newTestRepository(t)

	beneficiary := &schema.Beneficiary{
		RecordStatus: schema.BeneficiaryStatus("INVALID_STATUS"),
	}

	err := repository.Create(context.Background(), beneficiary)
	if !errors.Is(err, ErrInvalidBeneficiary) {
		t.Fatalf("Create() error = %v, want %v", err, ErrInvalidBeneficiary)
	}
}

func TestCreateRejectsNegativeRowVersion(t *testing.T) {
	repository := newTestRepository(t)

	beneficiary := &schema.Beneficiary{
		RecordStatus: schema.BeneficiaryPendingVerification,
		RowVersion:   -1,
	}

	err := repository.Create(context.Background(), beneficiary)
	if !errors.Is(err, ErrInvalidBeneficiary) {
		t.Fatalf("Create() error = %v, want %v", err, ErrInvalidBeneficiary)
	}
}

func TestValidStatus(t *testing.T) {
	validStatuses := []schema.BeneficiaryStatus{
		schema.BeneficiaryPendingVerification,
		schema.BeneficiaryVerified,
		schema.BeneficiaryActive,
		schema.BeneficiaryInactive,
		schema.BeneficiaryForReview,
		schema.BeneficiaryArchived,
	}

	for _, status := range validStatuses {
		if !validStatus(status) {
			t.Errorf("validStatus(%q) = false, want true", status)
		}
	}

	invalidStatuses := []schema.BeneficiaryStatus{
		"",
		"UNKNOWN",
		"APPROVED",
	}

	for _, status := range invalidStatuses {
		if validStatus(status) {
			t.Errorf("validStatus(%q) = true, want false", status)
		}
	}
}

func TestNormalizeOptionalString(t *testing.T) {
	value := "  OSCA-12345  "
	normalized := normalizeOptionalString(&value)

	if normalized == nil || *normalized != "OSCA-12345" {
		t.Fatalf("normalizeOptionalString() = %v, want OSCA-12345", normalized)
	}

	blank := "   "
	if normalizeOptionalString(&blank) != nil {
		t.Fatal("normalizeOptionalString() should return nil for whitespace")
	}

	if normalizeOptionalString(nil) != nil {
		t.Fatal("normalizeOptionalString(nil) should return nil")
	}
}

func TestGetByIDRejectsZeroUUID(t *testing.T) {
	repository := newTestRepository(t)

	_, err := repository.GetByID(context.Background(), gocql.UUID{})
	if !errors.Is(err, ErrNotFound) {
		t.Fatalf("GetByID(zero UUID) error = %v, want %v", err, ErrNotFound)
	}
}

func TestGetByOSCAIDRejectsBlankID(t *testing.T) {
	repository := newTestRepository(t)

	_, err := repository.GetByOSCAID(context.Background(), "   ")
	if !errors.Is(err, ErrNotFound) {
		t.Fatalf("GetByOSCAID(blank) error = %v, want %v", err, ErrNotFound)
	}
}
