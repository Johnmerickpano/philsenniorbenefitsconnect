package beneficiary

import (
	"encoding/base64"
	"encoding/json"
	"errors"
	"io"
	"net/http"
	"strconv"
	"strings"
	"time"

	"github.com/Johnmerickpano/philsenniorbenefitsconnect/internal/auth"
	"github.com/Johnmerickpano/philsenniorbenefitsconnect/internal/database/schema"
	"github.com/gocql/gocql"
)

type HTTPHandler struct {
	repository *Repository
}

func NewHTTPHandler(repository *Repository) *HTTPHandler {
	return &HTTPHandler{repository: repository}
}

type createBeneficiaryRequest struct {
	OSCAID       *string `json:"osca_id"`
	FirstName    *string `json:"first_name"`
	MiddleName   *string `json:"middle_name"`
	LastName     *string `json:"last_name"`
	Suffix       *string `json:"suffix"`
	BirthDate    *string `json:"birth_date"`
	Sex          *string `json:"sex"`
	BarangayCode *string `json:"barangay_code"`
	BarangayName *string `json:"barangay_name"`
}

type apiError struct {
	Error string `json:"error"`
}

func writeAPIJSON(w http.ResponseWriter, status int, value any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(value)
}

func decodeAPIJSON(w http.ResponseWriter, r *http.Request, destination any) error {
	r.Body = http.MaxBytesReader(w, r.Body, 1<<20)

	decoder := json.NewDecoder(r.Body)
	decoder.DisallowUnknownFields()

	if err := decoder.Decode(destination); err != nil {
		return err
	}

	var extra any
	if err := decoder.Decode(&extra); !errors.Is(err, io.EOF) {
		if err == nil {
			return errors.New("request must contain only one JSON value")
		}
		return err
	}

	return nil
}

func parseBirthDate(value *string) (*time.Time, error) {
	if value == nil {
		return nil, nil
	}

	parsed, err := time.Parse("2006-01-02", strings.TrimSpace(*value))
	if err != nil {
		return nil, err
	}

	return &parsed, nil
}
func authenticatedUserID(r *http.Request) (gocql.UUID, bool) {
	user, ok := auth.AuthenticatedUserFromContext(r.Context())
	if !ok {
		return gocql.UUID{}, false
	}

	id, err := gocql.ParseUUID(user.UserID)
	if err != nil {
		return gocql.UUID{}, false
	}

	return id, true
}

// Create handles POST /api/v1/beneficiaries.
func (h *HTTPHandler) Create(w http.ResponseWriter, r *http.Request) {
	if h == nil || h.repository == nil {
		writeAPIJSON(w, http.StatusInternalServerError, apiError{Error: "beneficiary service unavailable"})
		return
	}

	createdBy, ok := authenticatedUserID(r)
	if !ok {
		writeAPIJSON(w, http.StatusUnauthorized, apiError{Error: "authenticated user required"})
		return
	}

	var request createBeneficiaryRequest
	if err := decodeAPIJSON(w, r, &request); err != nil {
		writeAPIJSON(w, http.StatusBadRequest, apiError{Error: "invalid JSON request"})
		return
	}

	birthDate, err := parseBirthDate(request.BirthDate)
	if err != nil {
		writeAPIJSON(w, http.StatusBadRequest, apiError{Error: "birth_date must use YYYY-MM-DD"})
		return
	}

	beneficiary := &schema.Beneficiary{
		OSCAID:       request.OSCAID,
		FirstName:    request.FirstName,
		MiddleName:   request.MiddleName,
		LastName:     request.LastName,
		Suffix:       request.Suffix,
		BirthDate:    birthDate,
		Sex:          request.Sex,
		BarangayCode: request.BarangayCode,
		BarangayName: request.BarangayName,
		CreatedBy:    &createdBy,
	}

	if err := h.repository.Create(r.Context(), beneficiary); err != nil {
		switch {
		case errors.Is(err, ErrDuplicateOSCAID),
			errors.Is(err, ErrDuplicateBeneficiaryID):
			writeAPIJSON(w, http.StatusConflict, apiError{Error: "beneficiary record already exists"})
		case errors.Is(err, ErrInvalidBeneficiary):
			writeAPIJSON(w, http.StatusBadRequest, apiError{Error: "invalid beneficiary data"})
		default:
			writeAPIJSON(w, http.StatusInternalServerError, apiError{Error: "could not create beneficiary"})
		}
		return
	}

	writeAPIJSON(w, http.StatusCreated, beneficiary)
}

// GetByID handles GET /api/v1/beneficiaries/{id}.
func (h *HTTPHandler) GetByID(w http.ResponseWriter, r *http.Request) {
	if h == nil || h.repository == nil {
		writeAPIJSON(w, http.StatusInternalServerError, apiError{Error: "beneficiary service unavailable"})
		return
	}

	id, err := gocql.ParseUUID(r.PathValue("id"))
	if err != nil {
		writeAPIJSON(w, http.StatusBadRequest, apiError{Error: "invalid beneficiary ID"})
		return
	}

	beneficiary, err := h.repository.GetByID(r.Context(), id)
	if err != nil {
		if errors.Is(err, ErrNotFound) {
			writeAPIJSON(w, http.StatusNotFound, apiError{Error: "beneficiary not found"})
			return
		}

		writeAPIJSON(w, http.StatusInternalServerError, apiError{Error: "could not retrieve beneficiary"})
		return
	}

	writeAPIJSON(w, http.StatusOK, beneficiary)
}

// parsePagination reads and validates the requested page size and cursor.
func parsePagination(r *http.Request) (int, []byte, error) {
	pageSize := DefaultBeneficiaryPageSize

	if rawPageSize := strings.TrimSpace(r.URL.Query().Get("page_size")); rawPageSize != "" {
		parsed, err := strconv.Atoi(rawPageSize)
		if err != nil || parsed < 1 || parsed > MaxBeneficiaryPageSize {
			return 0, nil, errors.New("invalid page_size")
		}
		pageSize = parsed
	}

	var pagingState []byte
	if rawCursor := strings.TrimSpace(r.URL.Query().Get("cursor")); rawCursor != "" {
		decoded, err := base64.RawURLEncoding.DecodeString(rawCursor)
		if err != nil {
			return 0, nil, errors.New("invalid cursor")
		}
		pagingState = decoded
	}

	return pageSize, pagingState, nil
}

func writeBeneficiaryPage(w http.ResponseWriter, data any, nextState []byte) {
	nextCursor := ""
	if len(nextState) > 0 {
		nextCursor = base64.RawURLEncoding.EncodeToString(nextState)
	}

	writeAPIJSON(w, http.StatusOK, map[string]any{
		"data":        data,
		"next_cursor": nextCursor,
	})
}

// List handles GET /api/v1/beneficiaries.
func (h *HTTPHandler) List(w http.ResponseWriter, r *http.Request) {
	if h == nil || h.repository == nil {
		writeAPIJSON(w, http.StatusInternalServerError, apiError{Error: "beneficiary service unavailable"})
		return
	}

	barangayCode := strings.TrimSpace(r.URL.Query().Get("barangay_code"))
	if barangayCode == "" {
		writeAPIJSON(w, http.StatusBadRequest, apiError{Error: "barangay_code is required"})
		return
	}

	pageSize, pagingState, err := parsePagination(r)
	if err != nil {
		writeAPIJSON(w, http.StatusBadRequest, apiError{Error: err.Error()})
		return
	}

	status := strings.TrimSpace(r.URL.Query().Get("status"))
	if status != "" {
		records, nextState, err := h.repository.ListByStatus(
			r.Context(),
			schema.BeneficiaryStatus(status),
			barangayCode,
			pageSize,
			pagingState,
		)
		if err != nil {
			if errors.Is(err, ErrInvalidBeneficiary) {
				writeAPIJSON(w, http.StatusBadRequest, apiError{Error: "invalid beneficiary status"})
				return
			}
			writeAPIJSON(w, http.StatusInternalServerError, apiError{Error: "could not list beneficiaries"})
			return
		}

		writeBeneficiaryPage(w, records, nextState)
		return
	}

	records, nextState, err := h.repository.ListByBarangay(
		r.Context(),
		barangayCode,
		pageSize,
		pagingState,
	)
	if err != nil {
		writeAPIJSON(w, http.StatusInternalServerError, apiError{Error: "could not list beneficiaries"})
		return
	}

	writeBeneficiaryPage(w, records, nextState)
}

// Search handles GET /api/v1/beneficiaries/search.
func (h *HTTPHandler) Search(w http.ResponseWriter, r *http.Request) {
	if h == nil || h.repository == nil {
		writeAPIJSON(w, http.StatusInternalServerError, apiError{Error: "beneficiary service unavailable"})
		return
	}

	barangayCode := strings.TrimSpace(r.URL.Query().Get("barangay_code"))
	searchTerm := strings.TrimSpace(r.URL.Query().Get("q"))
	if barangayCode == "" || searchTerm == "" {
		writeAPIJSON(w, http.StatusBadRequest, apiError{Error: "barangay_code and q are required"})
		return
	}

	pageSize, pagingState, err := parsePagination(r)
	if err != nil {
		writeAPIJSON(w, http.StatusBadRequest, apiError{Error: err.Error()})
		return
	}

	records, nextState, err := h.repository.SearchByName(
		r.Context(),
		barangayCode,
		searchTerm,
		pageSize,
		pagingState,
	)
	if err != nil {
		if errors.Is(err, ErrInvalidBeneficiary) {
			writeAPIJSON(w, http.StatusBadRequest, apiError{Error: "search term must contain at least three letters or digits"})
			return
		}
		writeAPIJSON(w, http.StatusInternalServerError, apiError{Error: "could not search beneficiaries"})
		return
	}

	writeBeneficiaryPage(w, records, nextState)
}

// updateBeneficiaryRequest represents a complete profile replacement.
// Optional fields omitted or set to null are cleared.
type updateBeneficiaryRequest struct {
	OSCAID          *string `json:"osca_id"`
	FirstName       *string `json:"first_name"`
	MiddleName      *string `json:"middle_name"`
	LastName        *string `json:"last_name"`
	Suffix          *string `json:"suffix"`
	BirthDate       *string `json:"birth_date"`
	Sex             *string `json:"sex"`
	BarangayCode    *string `json:"barangay_code"`
	BarangayName    *string `json:"barangay_name"`
	ExpectedVersion int64   `json:"expected_version"`
}

// UpdateProfile handles PUT /api/v1/beneficiaries/{id}.
func (h *HTTPHandler) UpdateProfile(w http.ResponseWriter, r *http.Request) {
	if h == nil || h.repository == nil {
		writeAPIJSON(w, http.StatusInternalServerError, apiError{Error: "beneficiary service unavailable"})
		return
	}

	updatedBy, ok := authenticatedUserID(r)
	if !ok {
		writeAPIJSON(w, http.StatusUnauthorized, apiError{Error: "authenticated user required"})
		return
	}

	id, err := gocql.ParseUUID(r.PathValue("id"))
	if err != nil {
		writeAPIJSON(w, http.StatusBadRequest, apiError{Error: "invalid beneficiary ID"})
		return
	}

	var request updateBeneficiaryRequest
	if err := decodeAPIJSON(w, r, &request); err != nil {
		writeAPIJSON(w, http.StatusBadRequest, apiError{Error: "invalid JSON request"})
		return
	}

	if request.ExpectedVersion < 1 {
		writeAPIJSON(w, http.StatusBadRequest, apiError{Error: "expected_version must be a positive integer"})
		return
	}

	birthDate, err := parseBirthDate(request.BirthDate)
	if err != nil {
		writeAPIJSON(w, http.StatusBadRequest, apiError{Error: "birth_date must use YYYY-MM-DD"})
		return
	}

	profile := BeneficiaryProfile{
		OSCAID:       request.OSCAID,
		FirstName:    request.FirstName,
		MiddleName:   request.MiddleName,
		LastName:     request.LastName,
		Suffix:       request.Suffix,
		BirthDate:    birthDate,
		Sex:          request.Sex,
		BarangayCode: request.BarangayCode,
		BarangayName: request.BarangayName,
	}

	updated, err := h.repository.UpdateProfile(
		r.Context(),
		id,
		profile,
		request.ExpectedVersion,
		updatedBy,
	)
	if err != nil {
		switch {
		case errors.Is(err, ErrNotFound):
			writeAPIJSON(w, http.StatusNotFound, apiError{Error: "beneficiary not found"})
		case errors.Is(err, ErrVersionConflict):
			writeAPIJSON(w, http.StatusConflict, apiError{Error: "beneficiary was updated by another user; retrieve the latest record and retry"})
		case errors.Is(err, ErrInvalidBeneficiary):
			writeAPIJSON(w, http.StatusBadRequest, apiError{Error: "invalid beneficiary profile"})
		default:
			writeAPIJSON(w, http.StatusInternalServerError, apiError{Error: "could not update beneficiary"})
		}
		return
	}

	writeAPIJSON(w, http.StatusOK, updated)
}
