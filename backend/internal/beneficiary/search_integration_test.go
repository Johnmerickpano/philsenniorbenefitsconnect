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

func TestSearchByNamePaginationIntegration(t *testing.T) {
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

	barangayCode := "IT-SEARCH-" + gocql.TimeUUID().String()
	otherBarangayCode := "IT-SEARCH-OTHER-" + gocql.TimeUUID().String()

	names := []struct {
		first string
		last  string
	}{
		{"Maria", "Alpha"},
		{"Maribel", "Bravo"},
		{"Marcos", "Charlie"},
		{"Martha", "Delta"},
		{"Mariana", "Echo"},
	}

	records := make([]*schema.Beneficiary, 0, len(names)+1)

	for _, name := range names {
		firstName := name.first
		lastName := name.last
		code := barangayCode

		records = append(records, &schema.Beneficiary{
			BeneficiaryID: gocql.TimeUUID(),
			FirstName:     &firstName,
			LastName:      &lastName,
			BarangayCode:  &code,
			RecordStatus:  schema.BeneficiaryPendingVerification,
		})
	}

	// This record must not appear in results for the test barangay.
	otherFirstName := "Maria"
	otherLastName := "Outside"
	otherCode := otherBarangayCode

	records = append(records, &schema.Beneficiary{
		BeneficiaryID: gocql.TimeUUID(),
		FirstName:     &otherFirstName,
		LastName:      &otherLastName,
		BarangayCode:  &otherCode,
		RecordStatus:  schema.BeneficiaryPendingVerification,
	})

	defer func() {
		cleanupCtx, cleanupCancel := context.WithTimeout(
			context.Background(),
			10*time.Second,
		)
		defer cleanupCancel()

		for _, beneficiary := range records {
			sortName := deriveSortName(beneficiary)
			code := optionalStringValue(beneficiary.BarangayCode)

			_ = session.Query(
				`DELETE FROM beneficiaries_by_barangay
WHERE barangay_code = ? AND sort_name = ? AND beneficiary_id = ?`,
				code, sortName, beneficiary.BeneficiaryID,
			).WithContext(cleanupCtx).Exec()

			_ = session.Query(
				`DELETE FROM beneficiaries_by_status
WHERE record_status = ? AND barangay_code = ? AND sort_name = ? AND beneficiary_id = ?`,
				string(beneficiary.RecordStatus),
				code,
				sortName,
				beneficiary.BeneficiaryID,
			).WithContext(cleanupCtx).Exec()

			for _, prefix := range nameSearchPrefixes(beneficiary) {
				_ = session.Query(
					`DELETE FROM beneficiary_search
WHERE barangay_code = ? AND name_prefix = ? AND sort_name = ? AND beneficiary_id = ?`,
					code,
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
			t.Fatalf("create search fixture %s: %v",
				optionalStringValue(beneficiary.FirstName), err)
		}
	}

	t.Run("filters matching names and continues through pages", func(t *testing.T) {
		var pagingState []byte
		found := make(map[gocql.UUID]bool)
		pageCount := 0

		for {
			results, nextState, err := repository.SearchByName(
				ctx,
				barangayCode,
				"maria",
				2,
				pagingState,
			)
			if err != nil {
				t.Fatalf("SearchByName() error = %v", err)
			}

			pageCount++

			for _, result := range results {
				if result.BarangayCode != barangayCode {
					t.Errorf("result barangay = %q, want %q",
						result.BarangayCode, barangayCode)
				}

				if result.NamePrefix != "mar" {
					t.Errorf("result prefix = %q, want %q",
						result.NamePrefix, "mar")
				}

				if !matchesNameSearch(result.DisplayName, "maria") {
					t.Errorf("unexpected non-matching result: %v",
						result.DisplayName)
				}

				if found[result.BeneficiaryID] {
					t.Errorf("duplicate result ID: %v", result.BeneficiaryID)
				}

				found[result.BeneficiaryID] = true
			}

			if len(nextState) == 0 {
				break
			}

			if pageCount > 10 {
				t.Fatal("search pagination did not terminate")
			}

			pagingState = nextState
		}

		if pageCount < 2 {
			t.Errorf("page count = %d, want at least 2", pageCount)
		}

		expected := map[gocql.UUID]bool{
			records[0].BeneficiaryID: true, // Maria
			records[4].BeneficiaryID: true, // Mariana
		}

		if len(found) != len(expected) {
			t.Fatalf("found %d matching beneficiaries, want %d",
				len(found), len(expected))
		}

		for id := range expected {
			if !found[id] {
				t.Errorf("expected matching beneficiary %v was not returned", id)
			}
		}

		if found[records[5].BeneficiaryID] {
			t.Error("beneficiary from another barangay was returned")
		}
	})
}
