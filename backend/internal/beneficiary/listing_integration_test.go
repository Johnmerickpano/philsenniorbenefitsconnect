package beneficiary

import (
	"context"
	"os"
	"testing"
	"time"

	"github.com/Johnmerickpano/philsenniorbenefitsconnect/internal/config"
	"github.com/Johnmerickpano/philsenniorbenefitsconnect/internal/database"
	"github.com/Johnmerickpano/philsenniorbenefitsconnect/internal/database/schema"
	"github.com/gocql/gocql"
)

func TestListByBarangayPaginationIntegration(t *testing.T) {
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

	barangayCode := "IT-PAGE-" + gocql.TimeUUID().String()
	firstName := "Page"

	lastNames := []string{"Alpha", "Bravo", "Charlie"}
	beneficiaries := make([]*schema.Beneficiary, 0, len(lastNames))

	for _, lastNameValue := range lastNames {
		lastName := lastNameValue
		beneficiaries = append(beneficiaries, &schema.Beneficiary{
			BeneficiaryID: gocql.TimeUUID(),
			FirstName:     &firstName,
			LastName:      &lastName,
			BarangayCode:  &barangayCode,
			RecordStatus:  schema.BeneficiaryPendingVerification,
		})
	}

	// Remove all records and projections created by this test.
	defer func() {
		cleanupCtx, cleanupCancel := context.WithTimeout(
			context.Background(),
			10*time.Second,
		)
		defer cleanupCancel()

		for _, beneficiary := range beneficiaries {
			sortName := deriveSortName(beneficiary)

			_ = session.Query(
				`DELETE FROM beneficiaries_by_barangay
WHERE barangay_code = ? AND sort_name = ? AND beneficiary_id = ?`,
				barangayCode, sortName, beneficiary.BeneficiaryID,
			).WithContext(cleanupCtx).Exec()

			_ = session.Query(
				`DELETE FROM beneficiaries_by_status
WHERE record_status = ? AND barangay_code = ? AND sort_name = ? AND beneficiary_id = ?`,
				string(beneficiary.RecordStatus),
				barangayCode,
				sortName,
				beneficiary.BeneficiaryID,
			).WithContext(cleanupCtx).Exec()

			for _, prefix := range nameSearchPrefixes(beneficiary) {
				_ = session.Query(
					`DELETE FROM beneficiary_search
WHERE barangay_code = ? AND name_prefix = ? AND sort_name = ? AND beneficiary_id = ?`,
					barangayCode,
					prefix,
					sortName,
					beneficiary.BeneficiaryID,
				).WithContext(cleanupCtx).Exec()
			}

			_ = session.Query(
				`DELETE FROM beneficiaries_by_id WHERE beneficiary_id = ?`,
				beneficiary.BeneficiaryID,
			).WithContext(cleanupCtx).Exec()
		}
	}()

	for _, beneficiary := range beneficiaries {
		if err := repository.Create(ctx, beneficiary); err != nil {
			t.Fatalf("Create() for %q: %v", optionalStringValue(beneficiary.LastName), err)
		}
	}

	t.Run("first page returns two ordered records and continuation state", func(t *testing.T) {
		firstPage, nextState, err := repository.ListByBarangay(
			ctx,
			barangayCode,
			2,
			nil,
		)
		if err != nil {
			t.Fatalf("ListByBarangay() first page error = %v", err)
		}

		if len(firstPage) != 2 {
			t.Fatalf("first page length = %d, want 2", len(firstPage))
		}

		if len(nextState) == 0 {
			t.Fatal("expected a continuation paging state")
		}

		for i, item := range firstPage {
			if item.BeneficiaryID != beneficiaries[i].BeneficiaryID {
				t.Errorf(
					"first page item %d ID = %v, want %v",
					i,
					item.BeneficiaryID,
					beneficiaries[i].BeneficiaryID,
				)
			}
		}
	})

	t.Run("all pages return records without duplicates or omissions", func(t *testing.T) {
		firstPage, nextState, err := repository.ListByBarangay(
			ctx,
			barangayCode,
			2,
			nil,
		)
		if err != nil {
			t.Fatalf("ListByBarangay() first page error = %v", err)
		}

		secondPage, finalState, err := repository.ListByBarangay(
			ctx,
			barangayCode,
			2,
			nextState,
		)
		if err != nil {
			t.Fatalf("ListByBarangay() second page error = %v", err)
		}

		if len(secondPage) != 1 {
			t.Fatalf("second page length = %d, want 1", len(secondPage))
		}

		if len(finalState) != 0 {
			t.Errorf("final paging state has %d bytes, want 0", len(finalState))
		}

		all := append(firstPage, secondPage...)

		if len(all) != len(beneficiaries) {
			t.Fatalf("total records = %d, want %d", len(all), len(beneficiaries))
		}

		seen := make(map[gocql.UUID]bool)

		for i, item := range all {
			if seen[item.BeneficiaryID] {
				t.Errorf("duplicate beneficiary ID: %v", item.BeneficiaryID)
			}
			seen[item.BeneficiaryID] = true

			if item.BeneficiaryID != beneficiaries[i].BeneficiaryID {
				t.Errorf(
					"record %d ID = %v, want %v",
					i,
					item.BeneficiaryID,
					beneficiaries[i].BeneficiaryID,
				)
			}

			if item.BarangayCode != barangayCode {
				t.Errorf(
					"record %d barangay code = %q, want %q",
					i,
					item.BarangayCode,
					barangayCode,
				)
			}
		}
	})
}

func TestListByStatusPaginationIntegration(t *testing.T) {
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

	barangayCode := "IT-STATUS-" + gocql.TimeUUID().String()
	firstName := "Status"

	records := []*schema.Beneficiary{
		{
			BeneficiaryID: gocql.TimeUUID(),
			FirstName:     &firstName,
			LastName:      stringPointer("Alpha"),
			BarangayCode:  &barangayCode,
			RecordStatus:  schema.BeneficiaryVerified,
		},
		{
			BeneficiaryID: gocql.TimeUUID(),
			FirstName:     &firstName,
			LastName:      stringPointer("Bravo"),
			BarangayCode:  &barangayCode,
			RecordStatus:  schema.BeneficiaryVerified,
		},
		{
			BeneficiaryID: gocql.TimeUUID(),
			FirstName:     &firstName,
			LastName:      stringPointer("Charlie"),
			BarangayCode:  &barangayCode,
			RecordStatus:  schema.BeneficiaryVerified,
		},
		{
			BeneficiaryID: gocql.TimeUUID(),
			FirstName:     &firstName,
			LastName:      stringPointer("Delta"),
			BarangayCode:  &barangayCode,
			RecordStatus:  schema.BeneficiaryInactive,
		},
	}

	expected := records[:3]

	defer func() {
		cleanupCtx, cleanupCancel := context.WithTimeout(
			context.Background(),
			10*time.Second,
		)
		defer cleanupCancel()

		for _, beneficiary := range records {
			sortName := deriveSortName(beneficiary)

			_ = session.Query(
				`DELETE FROM beneficiaries_by_barangay
WHERE barangay_code = ? AND sort_name = ? AND beneficiary_id = ?`,
				barangayCode, sortName, beneficiary.BeneficiaryID,
			).WithContext(cleanupCtx).Exec()

			_ = session.Query(
				`DELETE FROM beneficiaries_by_status
WHERE record_status = ? AND barangay_code = ? AND sort_name = ? AND beneficiary_id = ?`,
				string(beneficiary.RecordStatus),
				barangayCode,
				sortName,
				beneficiary.BeneficiaryID,
			).WithContext(cleanupCtx).Exec()

			for _, prefix := range nameSearchPrefixes(beneficiary) {
				_ = session.Query(
					`DELETE FROM beneficiary_search
WHERE barangay_code = ? AND name_prefix = ? AND sort_name = ? AND beneficiary_id = ?`,
					barangayCode,
					prefix,
					sortName,
					beneficiary.BeneficiaryID,
				).WithContext(cleanupCtx).Exec()
			}

			_ = session.Query(
				`DELETE FROM beneficiaries_by_id WHERE beneficiary_id = ?`,
				beneficiary.BeneficiaryID,
			).WithContext(cleanupCtx).Exec()
		}
	}()

	for _, beneficiary := range records {
		if err := repository.Create(ctx, beneficiary); err != nil {
			t.Fatalf(
				"Create() for %q: %v",
				optionalStringValue(beneficiary.LastName),
				err,
			)
		}
	}

	t.Run("filters by status and returns ordered first page", func(t *testing.T) {
		firstPage, nextState, err := repository.ListByStatus(
			ctx,
			schema.BeneficiaryVerified,
			barangayCode,
			2,
			nil,
		)
		if err != nil {
			t.Fatalf("ListByStatus() first page error = %v", err)
		}

		if len(firstPage) != 2 {
			t.Fatalf("first page length = %d, want 2", len(firstPage))
		}

		if len(nextState) == 0 {
			t.Fatal("expected a continuation paging state")
		}

		for i, item := range firstPage {
			if item.BeneficiaryID != expected[i].BeneficiaryID {
				t.Errorf(
					"first page item %d ID = %v, want %v",
					i,
					item.BeneficiaryID,
					expected[i].BeneficiaryID,
				)
			}

			if item.RecordStatus != schema.BeneficiaryVerified {
				t.Errorf(
					"first page item %d status = %q, want %q",
					i,
					item.RecordStatus,
					schema.BeneficiaryVerified,
				)
			}
		}
	})

	t.Run("all pages contain only matching records without duplicates", func(t *testing.T) {
		firstPage, nextState, err := repository.ListByStatus(
			ctx,
			schema.BeneficiaryVerified,
			barangayCode,
			2,
			nil,
		)
		if err != nil {
			t.Fatalf("ListByStatus() first page error = %v", err)
		}

		secondPage, finalState, err := repository.ListByStatus(
			ctx,
			schema.BeneficiaryVerified,
			barangayCode,
			2,
			nextState,
		)
		if err != nil {
			t.Fatalf("ListByStatus() second page error = %v", err)
		}

		if len(secondPage) != 1 {
			t.Fatalf("second page length = %d, want 1", len(secondPage))
		}

		if len(finalState) != 0 {
			t.Errorf("final paging state has %d bytes, want 0", len(finalState))
		}

		all := append(firstPage, secondPage...)

		if len(all) != len(expected) {
			t.Fatalf("total records = %d, want %d", len(all), len(expected))
		}

		seen := make(map[gocql.UUID]bool)

		for i, item := range all {
			if seen[item.BeneficiaryID] {
				t.Errorf("duplicate beneficiary ID: %v", item.BeneficiaryID)
			}
			seen[item.BeneficiaryID] = true

			if item.BeneficiaryID != expected[i].BeneficiaryID {
				t.Errorf(
					"record %d ID = %v, want %v",
					i,
					item.BeneficiaryID,
					expected[i].BeneficiaryID,
				)
			}

			if item.RecordStatus != schema.BeneficiaryVerified {
				t.Errorf(
					"record %d status = %q, want %q",
					i,
					item.RecordStatus,
					schema.BeneficiaryVerified,
				)
			}

			if item.BarangayCode != barangayCode {
				t.Errorf(
					"record %d barangay code = %q, want %q",
					i,
					item.BarangayCode,
					barangayCode,
				)
			}
		}
	})
}
