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

func TestUpdateProfileIntegration(t *testing.T) {
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

	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	oldBarangay := "IT-PROFILE-OLD-" + gocql.TimeUUID().String()
	newBarangay := "IT-PROFILE-NEW-" + gocql.TimeUUID().String()

	oldOSCAID := "IT-PROFILE-OLD-" + gocql.TimeUUID().String()
	newOSCAID := "IT-PROFILE-NEW-" + gocql.TimeUUID().String()
	otherOSCAID := "IT-PROFILE-OTHER-" + gocql.TimeUUID().String()

	firstName := "Original"
	lastName := "Profile"
	newFirstName := "Updated"
	newLastName := "Beneficiary"
	barangayName := "Integration Barangay"

	first := &schema.Beneficiary{
		BeneficiaryID: gocql.TimeUUID(),
		OSCAID:        &oldOSCAID,
		FirstName:     &firstName,
		LastName:      &lastName,
		BarangayCode:  &oldBarangay,
		RecordStatus:  schema.BeneficiaryVerified,
	}

	second := &schema.Beneficiary{
		BeneficiaryID: gocql.TimeUUID(),
		OSCAID:        &otherOSCAID,
		FirstName:     stringPointer("Other"),
		LastName:      stringPointer("Owner"),
		BarangayCode:  &oldBarangay,
		RecordStatus:  schema.BeneficiaryPendingVerification,
	}

	var snapshots []*schema.Beneficiary

	defer func() {
		cleanupCtx, cleanupCancel := context.WithTimeout(
			context.Background(),
			10*time.Second,
		)
		defer cleanupCancel()

		cleaned := make(map[gocql.UUID]bool)

		for _, snapshot := range snapshots {
			if snapshot == nil {
				continue
			}

			sortName := optionalStringValue(snapshot.SortName)
			if sortName == "" {
				sortName = deriveSortName(snapshot)
			}

			if snapshot.BarangayCode != nil {
				barangay := *snapshot.BarangayCode

				_ = session.Query(
					`DELETE FROM beneficiaries_by_barangay
WHERE barangay_code = ? AND sort_name = ? AND beneficiary_id = ?`,
					barangay, sortName, snapshot.BeneficiaryID,
				).WithContext(cleanupCtx).Exec()

				_ = session.Query(
					`DELETE FROM beneficiaries_by_status
WHERE record_status = ? AND barangay_code = ? AND sort_name = ? AND beneficiary_id = ?`,
					string(snapshot.RecordStatus),
					barangay,
					sortName,
					snapshot.BeneficiaryID,
				).WithContext(cleanupCtx).Exec()

				for _, prefix := range nameSearchPrefixes(snapshot) {
					_ = session.Query(
						`DELETE FROM beneficiary_search
WHERE barangay_code = ? AND name_prefix = ? AND sort_name = ? AND beneficiary_id = ?`,
						barangay,
						prefix,
						sortName,
						snapshot.BeneficiaryID,
					).WithContext(cleanupCtx).Exec()
				}
			}

			if snapshot.OSCAID != nil {
				_ = session.Query(
					`DELETE FROM beneficiaries_by_osca_id WHERE osca_id = ?`,
					*snapshot.OSCAID,
				).WithContext(cleanupCtx).Exec()
			}

			if !cleaned[snapshot.BeneficiaryID] {
				_ = session.Query(
					`DELETE FROM beneficiaries_by_id WHERE beneficiary_id = ?`,
					snapshot.BeneficiaryID,
				).WithContext(cleanupCtx).Exec()

				cleaned[snapshot.BeneficiaryID] = true
			}
		}
	}()

	if err := repository.Create(ctx, first); err != nil {
		t.Fatalf("create first beneficiary: %v", err)
	}
	originalSnapshot := *first
	snapshots = append(snapshots, &originalSnapshot)

	if err := repository.Create(ctx, second); err != nil {
		t.Fatalf("create second beneficiary: %v", err)
	}
	secondSnapshot := *second
	snapshots = append(snapshots, &secondSnapshot)

	updatedBy := gocql.TimeUUID()

	profile := BeneficiaryProfile{
		OSCAID:       &newOSCAID,
		FirstName:    &newFirstName,
		LastName:     &newLastName,
		BarangayCode: &newBarangay,
		BarangayName: &barangayName,
	}

	t.Run("replaces profile and refreshes projections", func(t *testing.T) {
		updated, err := repository.UpdateProfile(
			ctx,
			first.BeneficiaryID,
			profile,
			1,
			updatedBy,
		)
		if err != nil {
			t.Fatalf("UpdateProfile() error = %v", err)
		}

		updatedSnapshot := *updated
		snapshots = append(snapshots, &updatedSnapshot)

		if updated.RowVersion != 2 {
			t.Errorf("RowVersion = %d, want 2", updated.RowVersion)
		}

		if updated.OSCAID == nil || *updated.OSCAID != newOSCAID {
			t.Errorf("OSCAID = %v, want %q", updated.OSCAID, newOSCAID)
		}

		if updated.FirstName == nil || *updated.FirstName != newFirstName {
			t.Errorf("FirstName = %v, want %q", updated.FirstName, newFirstName)
		}

		if updated.LastName == nil || *updated.LastName != newLastName {
			t.Errorf("LastName = %v, want %q", updated.LastName, newLastName)
		}

		if updated.BarangayCode == nil || *updated.BarangayCode != newBarangay {
			t.Errorf("BarangayCode = %v, want %q", updated.BarangayCode, newBarangay)
		}

		if updated.RecordStatus != schema.BeneficiaryVerified {
			t.Errorf("RecordStatus = %q, want %q",
				updated.RecordStatus,
				schema.BeneficiaryVerified,
			)
		}

		if updated.UpdatedBy == nil || *updated.UpdatedBy != updatedBy {
			t.Errorf("UpdatedBy = %v, want %v", updated.UpdatedBy, updatedBy)
		}

		got, err := repository.GetByID(ctx, first.BeneficiaryID)
		if err != nil {
			t.Fatalf("GetByID() after update: %v", err)
		}

		if got.RowVersion != 2 {
			t.Errorf("stored RowVersion = %d, want 2", got.RowVersion)
		}

		if got.RecordStatus != schema.BeneficiaryVerified {
			t.Errorf("stored RecordStatus = %q, want VERIFIED", got.RecordStatus)
		}

		if _, err := repository.GetByOSCAID(ctx, oldOSCAID); !errors.Is(err, ErrNotFound) {
			t.Errorf("old OSCA lookup error = %v, want ErrNotFound", err)
		}

		byNewOSCA, err := repository.GetByOSCAID(ctx, newOSCAID)
		if err != nil {
			t.Fatalf("lookup new OSCA ID: %v", err)
		}

		if byNewOSCA.BeneficiaryID != first.BeneficiaryID {
			t.Errorf("new OSCA owner = %v, want %v",
				byNewOSCA.BeneficiaryID,
				first.BeneficiaryID,
			)
		}

		oldSortName := deriveSortName(&originalSnapshot)

		var oldID gocql.UUID
		err = session.Query(
			`SELECT beneficiary_id FROM beneficiaries_by_barangay
WHERE barangay_code = ? AND sort_name = ? AND beneficiary_id = ?`,
			oldBarangay,
			oldSortName,
			first.BeneficiaryID,
		).WithContext(ctx).Scan(&oldID)

		if !errors.Is(err, gocql.ErrNotFound) {
			t.Errorf("old barangay projection error = %v, want ErrNotFound", err)
		}

		newSortName := deriveSortName(updated)

		err = session.Query(
			`SELECT beneficiary_id FROM beneficiaries_by_barangay
WHERE barangay_code = ? AND sort_name = ? AND beneficiary_id = ?`,
			newBarangay,
			newSortName,
			first.BeneficiaryID,
		).WithContext(ctx).Scan(&oldID)

		if err != nil {
			t.Fatalf("read new barangay projection: %v", err)
		}

		if oldID != first.BeneficiaryID {
			t.Errorf("new barangay projection ID = %v, want %v",
				oldID,
				first.BeneficiaryID,
			)
		}

		for _, prefix := range nameSearchPrefixes(updated) {
			err = session.Query(
				`SELECT beneficiary_id FROM beneficiary_search
WHERE barangay_code = ? AND name_prefix = ? AND sort_name = ? AND beneficiary_id = ?`,
				newBarangay,
				prefix,
				newSortName,
				first.BeneficiaryID,
			).WithContext(ctx).Scan(&oldID)

			if err != nil {
				t.Errorf("read new search prefix %q: %v", prefix, err)
				continue
			}

			if oldID != first.BeneficiaryID {
				t.Errorf("search prefix %q returned ID %v, want %v",
					prefix,
					oldID,
					first.BeneficiaryID,
				)
			}
		}
	})

	t.Run("rejects stale version", func(t *testing.T) {
		_, err := repository.UpdateProfile(
			ctx,
			first.BeneficiaryID,
			profile,
			1,
			updatedBy,
		)

		if !errors.Is(err, ErrVersionConflict) {
			t.Fatalf("UpdateProfile() error = %v, want %v", err, ErrVersionConflict)
		}
	})

	t.Run("rejects OSCA ID owned by another beneficiary", func(t *testing.T) {
		duplicateProfile := BeneficiaryProfile{
			OSCAID:       &newOSCAID,
			FirstName:    stringPointer("Other"),
			LastName:     stringPointer("Owner"),
			BarangayCode: &oldBarangay,
		}

		_, err := repository.UpdateProfile(
			ctx,
			second.BeneficiaryID,
			duplicateProfile,
			1,
			updatedBy,
		)

		if !errors.Is(err, ErrDuplicateOSCAID) {
			t.Fatalf("UpdateProfile() error = %v, want %v", err, ErrDuplicateOSCAID)
		}

		got, err := repository.GetByID(ctx, second.BeneficiaryID)
		if err != nil {
			t.Fatalf("read second beneficiary after rejected update: %v", err)
		}

		if got.OSCAID == nil || *got.OSCAID != otherOSCAID {
			t.Errorf("second beneficiary OSCAID = %v, want %q", got.OSCAID, otherOSCAID)
		}

		if got.RowVersion != 1 {
			t.Errorf("second beneficiary RowVersion = %d, want 1", got.RowVersion)
		}
	})
}
