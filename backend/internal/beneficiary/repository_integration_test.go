package beneficiary

import (
	"context"
	"errors"
	"os"
	"testing"
	"time"

	"github.com/Johnmerickpano/philsenniorbenefitsconnect/internal/config"
	"github.com/Johnmerickpano/philsenniorbenefitsconnect/internal/database"
	"github.com/Johnmerickpano/philsenniorbenefitsconnect/internal/database/schema"
	"github.com/gocql/gocql"
)

func TestBeneficiaryRepositoryIntegration(t *testing.T) {
	if os.Getenv("PSBC_INTEGRATION_TEST") != "1" {
		t.Skip("set PSBC_INTEGRATION_TEST=1 to run ScyllaDB integration tests")
	}

	cfg, err := config.Load()
	if err != nil {
		t.Fatalf("load config: %v", err)
	}

	session, err := database.NewScyllaSession(cfg)
	if err != nil {
		t.Fatalf("connect to ScyllaDB: %v", err)
	}
	defer session.Close()

	repository, err := NewRepository(session)
	if err != nil {
		t.Fatalf("create beneficiary repository: %v", err)
	}

	ctx, cancel := context.WithTimeout(context.Background(), 20*time.Second)
	defer cancel()

	beneficiaryID := gocql.TimeUUID()
	oscaID := "IT-" + gocql.TimeUUID().String()
	firstName := "Integration"
	lastName := "Test"
	barangayCode := "IT-BRG-" + gocql.TimeUUID().String()
	beneficiary := &schema.Beneficiary{
		BeneficiaryID: beneficiaryID,
		OSCAID:        &oscaID,
		FirstName:     &firstName,
		LastName:      &lastName,
		BarangayCode:  &barangayCode,
		RecordStatus:  schema.BeneficiaryPendingVerification,
	}

	// Remove only this test's records, including after a failed assertion.
	defer func() {
		cleanupCtx, cleanupCancel := context.WithTimeout(
			context.Background(),
			5*time.Second,
		)
		defer cleanupCancel()

		sortName := deriveSortName(beneficiary)

		_ = session.Query(
			`DELETE FROM beneficiaries_by_barangay
WHERE barangay_code = ? AND sort_name = ? AND beneficiary_id = ?`,
			barangayCode, sortName, beneficiaryID,
		).WithContext(cleanupCtx).Exec()

		_ = session.Query(
			`DELETE FROM beneficiaries_by_status
WHERE record_status = ? AND barangay_code = ? AND sort_name = ? AND beneficiary_id = ?`,
			string(beneficiary.RecordStatus), barangayCode, sortName, beneficiaryID,
		).WithContext(cleanupCtx).Exec()

		for _, prefix := range nameSearchPrefixes(beneficiary) {
			_ = session.Query(
				`DELETE FROM beneficiary_search
WHERE barangay_code = ? AND name_prefix = ? AND sort_name = ? AND beneficiary_id = ?`,
				barangayCode, prefix, sortName, beneficiaryID,
			).WithContext(cleanupCtx).Exec()
		}

		_ = session.Query(
			`DELETE FROM beneficiaries_by_id WHERE beneficiary_id = ?`,
			beneficiaryID,
		).WithContext(cleanupCtx).Exec()

		_ = session.Query(
			`DELETE FROM beneficiaries_by_osca_id WHERE osca_id = ?`,
			oscaID,
		).WithContext(cleanupCtx).Exec()
	}()

	t.Run("create and retrieve by beneficiary ID", func(t *testing.T) {
		if err := repository.Create(ctx, beneficiary); err != nil {
			t.Fatalf("Create() error = %v", err)
		}

		if beneficiary.RowVersion != 1 {
			t.Errorf("RowVersion = %d, want 1", beneficiary.RowVersion)
		}

		if beneficiary.CreatedAt == nil {
			t.Error("CreatedAt was not initialized")
		}

		if beneficiary.UpdatedAt == nil {
			t.Error("UpdatedAt was not initialized")
		}

		got, err := repository.GetByID(ctx, beneficiaryID)
		if err != nil {
			t.Fatalf("GetByID() error = %v", err)
		}

		if got.BeneficiaryID != beneficiaryID {
			t.Errorf("BeneficiaryID = %v, want %v", got.BeneficiaryID, beneficiaryID)
		}

		if got.OSCAID == nil || *got.OSCAID != oscaID {
			t.Errorf("OSCAID = %v, want %q", got.OSCAID, oscaID)
		}

		if got.FirstName == nil || *got.FirstName != firstName {
			t.Errorf("FirstName = %v, want %q", got.FirstName, firstName)
		}

		if got.LastName == nil || *got.LastName != lastName {
			t.Errorf("LastName = %v, want %q", got.LastName, lastName)
		}

		if got.RecordStatus != schema.BeneficiaryPendingVerification {
			t.Errorf("RecordStatus = %q, want %q",
				got.RecordStatus,
				schema.BeneficiaryPendingVerification,
			)
		}

		if got.RowVersion != 1 {
			t.Errorf("retrieved RowVersion = %d, want 1", got.RowVersion)
		}
	})

	t.Run("retrieve by OSCA ID", func(t *testing.T) {
		got, err := repository.GetByOSCAID(ctx, oscaID)
		if err != nil {
			t.Fatalf("GetByOSCAID() error = %v", err)
		}

		if got.BeneficiaryID != beneficiaryID {
			t.Errorf("BeneficiaryID = %v, want %v", got.BeneficiaryID, beneficiaryID)
		}
	})

	t.Run("write barangay, status, and search projections", func(t *testing.T) {
		sortName := deriveSortName(beneficiary)

		var gotID gocql.UUID

		err := session.Query(
			`SELECT beneficiary_id FROM beneficiaries_by_barangay
WHERE barangay_code = ? AND sort_name = ?`,
			barangayCode, sortName,
		).WithContext(ctx).Scan(&gotID)
		if err != nil {
			t.Fatalf("read barangay projection: %v", err)
		}
		if gotID != beneficiaryID {
			t.Errorf("barangay projection ID = %v, want %v", gotID, beneficiaryID)
		}

		err = session.Query(
			`SELECT beneficiary_id FROM beneficiaries_by_status
WHERE record_status = ? AND barangay_code = ? AND sort_name = ?`,
			string(beneficiary.RecordStatus), barangayCode, sortName,
		).WithContext(ctx).Scan(&gotID)
		if err != nil {
			t.Fatalf("read status projection: %v", err)
		}
		if gotID != beneficiaryID {
			t.Errorf("status projection ID = %v, want %v", gotID, beneficiaryID)
		}

		prefixes := nameSearchPrefixes(beneficiary)
		if len(prefixes) == 0 {
			t.Fatal("expected at least one name-search prefix")
		}

		for _, prefix := range prefixes {
			err := session.Query(
				`SELECT beneficiary_id FROM beneficiary_search
WHERE barangay_code = ? AND name_prefix = ? AND sort_name = ?`,
				barangayCode, prefix, sortName,
			).WithContext(ctx).Scan(&gotID)
			if err != nil {
				t.Errorf("read search projection for prefix %q: %v", prefix, err)
				continue
			}
			if gotID != beneficiaryID {
				t.Errorf("search projection for %q has ID %v, want %v",
					prefix, gotID, beneficiaryID)
			}
		}
	})

	t.Run("reject duplicate OSCA ID", func(t *testing.T) {
		duplicate := &schema.Beneficiary{
			BeneficiaryID: gocql.TimeUUID(),
			OSCAID:        &oscaID,
			RecordStatus:  schema.BeneficiaryPendingVerification,
		}

		err := repository.Create(ctx, duplicate)
		if !errors.Is(err, ErrDuplicateOSCAID) {
			t.Fatalf("Create() error = %v, want %v", err, ErrDuplicateOSCAID)
		}
	})

	t.Run("return not found for unknown beneficiary", func(t *testing.T) {
		_, err := repository.GetByID(ctx, gocql.TimeUUID())
		if !errors.Is(err, ErrNotFound) {
			t.Fatalf("GetByID() error = %v, want %v", err, ErrNotFound)
		}
	})

	t.Run("return not found for unknown OSCA ID", func(t *testing.T) {
		_, err := repository.GetByOSCAID(ctx, "IT-MISSING-"+gocql.TimeUUID().String())
		if !errors.Is(err, ErrNotFound) {
			t.Fatalf("GetByOSCAID() error = %v, want %v", err, ErrNotFound)
		}
	})
}
