package beneficiary

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

func newHTTPTestHandler(t *testing.T) *HTTPHandler {
	t.Helper()
	return NewHTTPHandler(newTestRepository(t))
}

func TestHTTPHandlerUnavailable(t *testing.T) {
	var handler *HTTPHandler

	tests := []struct {
		name string
		call func(http.ResponseWriter, *http.Request)
	}{
		{"create", handler.Create},
		{"get", handler.GetByID},
		{"list", handler.List},
		{"search", handler.Search},
		{"update", handler.UpdateProfile},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			request := httptest.NewRequest(http.MethodGet, "/", nil)
			response := httptest.NewRecorder()

			tt.call(response, request)

			if response.Code != http.StatusInternalServerError {
				t.Fatalf("status = %d, want %d", response.Code, http.StatusInternalServerError)
			}
		})
	}
}

func TestHTTPCreateRejectsUnauthenticatedRequest(t *testing.T) {
	handler := newHTTPTestHandler(t)

	request := httptest.NewRequest(
		http.MethodPost,
		"/api/v1/beneficiaries",
		strings.NewReader(`{}`),
	)
	response := httptest.NewRecorder()

	handler.Create(response, request)

	if response.Code != http.StatusUnauthorized {
		t.Fatalf("status = %d, want %d", response.Code, http.StatusUnauthorized)
	}
}

func TestHTTPUpdateRejectsUnauthenticatedRequest(t *testing.T) {
	handler := newHTTPTestHandler(t)

	request := httptest.NewRequest(
		http.MethodPut,
		"/api/v1/beneficiaries/invalid-id",
		strings.NewReader(`{"expected_version":1}`),
	)
	request.SetPathValue("id", "invalid-id")

	response := httptest.NewRecorder()
	handler.UpdateProfile(response, request)

	if response.Code != http.StatusUnauthorized {
		t.Fatalf("status = %d, want %d", response.Code, http.StatusUnauthorized)
	}
}

func TestHTTPGetByIDRejectsInvalidUUID(t *testing.T) {
	handler := newHTTPTestHandler(t)

	request := httptest.NewRequest(http.MethodGet, "/", nil)
	request.SetPathValue("id", "invalid-id")

	response := httptest.NewRecorder()
	handler.GetByID(response, request)

	if response.Code != http.StatusBadRequest {
		t.Fatalf("status = %d, want %d", response.Code, http.StatusBadRequest)
	}
}

func TestHTTPListValidatesQueryParameters(t *testing.T) {
	tests := []struct {
		name  string
		query string
	}{
		{"missing barangay", ""},
		{"invalid page size", "?barangay_code=TEST&page_size=0"},
		{"page size too large", "?barangay_code=TEST&page_size=999"},
		{"non numeric page size", "?barangay_code=TEST&page_size=abc"},
		{"invalid cursor", "?barangay_code=TEST&cursor=invalid*cursor"},
	}

	handler := newHTTPTestHandler(t)

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			request := httptest.NewRequest(
				http.MethodGet,
				"/api/v1/beneficiaries"+tt.query,
				nil,
			)
			response := httptest.NewRecorder()

			handler.List(response, request)

			if response.Code != http.StatusBadRequest {
				t.Fatalf("status = %d, want %d; body=%s",
					response.Code, http.StatusBadRequest, response.Body.String())
			}
		})
	}
}

func TestHTTPSearchValidatesQueryParameters(t *testing.T) {
	tests := []struct {
		name  string
		query string
	}{
		{"missing barangay", "?q=Maria"},
		{"missing search term", "?barangay_code=TEST"},
		{"invalid page size", "?barangay_code=TEST&q=Maria&page_size=0"},
		{"invalid cursor", "?barangay_code=TEST&q=Maria&cursor=invalid*cursor"},
	}

	handler := newHTTPTestHandler(t)

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			request := httptest.NewRequest(
				http.MethodGet,
				"/api/v1/beneficiaries/search"+tt.query,
				nil,
			)
			response := httptest.NewRecorder()

			handler.Search(response, request)

			if response.Code != http.StatusBadRequest {
				t.Fatalf("status = %d, want %d; body=%s",
					response.Code, http.StatusBadRequest, response.Body.String())
			}
		})
	}
}

func TestHTTPCreateChecksAuthenticationBeforeDecoding(t *testing.T) {
	handler := newHTTPTestHandler(t)

	tests := []struct {
		name string
		body string
	}{
		{"malformed JSON", `{"first_name":`},
		{"unknown field", `{"unexpected":"value"}`},
		{"multiple JSON values", `{} {}`},
		{"invalid birth date", `{"birth_date":"12-31-2000"}`},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			request := httptest.NewRequest(
				http.MethodPost,
				"/api/v1/beneficiaries",
				strings.NewReader(tt.body),
			)
			response := httptest.NewRecorder()

			handler.Create(response, request)

			// Authentication is checked before JSON parsing.
			if response.Code != http.StatusUnauthorized {
				t.Fatalf("status = %d, want %d",
					response.Code, http.StatusUnauthorized)
			}
		})
	}
}

func TestHTTPUpdateChecksAuthenticationBeforeValidation(t *testing.T) {
	handler := newHTTPTestHandler(t)

	request := httptest.NewRequest(
		http.MethodPut,
		"/api/v1/beneficiaries/invalid-id",
		strings.NewReader(`not-json`),
	)
	request.SetPathValue("id", "invalid-id")

	response := httptest.NewRecorder()
	handler.UpdateProfile(response, request)

	// Authentication is checked before ID parsing.
	if response.Code != http.StatusUnauthorized {
		t.Fatalf("status = %d, want %d",
			response.Code, http.StatusUnauthorized)
	}
}

// TestDecodeAPIJSON validates request-body decoding independently of authentication.
func TestDecodeAPIJSON(t *testing.T) {
	oversizedBody := `{"value":"` + strings.Repeat("a", 1<<20) + `"}`

	tests := []struct {
		name    string
		body    string
		wantErr bool
	}{
		{"valid JSON", `{"value":"hello"}`, false},
		{"malformed JSON", `{"value":`, true},
		{"unknown field", `{"unexpected":"value"}`, true},
		{"multiple JSON values", `{"value":"one"} {"value":"two"}`, true},
		{"oversized body", oversizedBody, true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			request := httptest.NewRequest(
				http.MethodPost,
				"/",
				strings.NewReader(tt.body),
			)
			response := httptest.NewRecorder()

			destination := struct {
				Value string `json:"value"`
			}{}

			err := decodeAPIJSON(response, request, &destination)

			if (err != nil) != tt.wantErr {
				t.Fatalf("decodeAPIJSON() error = %v, wantErr %v", err, tt.wantErr)
			}

			if err == nil && destination.Value != "hello" {
				t.Fatalf("decoded value = %q, want %q", destination.Value, "hello")
			}
		})
	}
}

// TestParseBirthDate validates the shared birth-date parser.
func TestParseBirthDate(t *testing.T) {
	tests := []struct {
		name    string
		value   *string
		want    string
		wantErr bool
	}{
		{"nil date", nil, "", false},
		{"valid date", stringPointer("2000-12-31"), "2000-12-31", false},
		{"whitespace around date", stringPointer(" 2000-12-31 "), "2000-12-31", false},
		{"empty date", stringPointer(""), "", true},
		{"wrong format", stringPointer("12-31-2000"), "", true},
		{"impossible date", stringPointer("2000-02-30"), "", true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			parsed, err := parseBirthDate(tt.value)

			if (err != nil) != tt.wantErr {
				t.Fatalf("parseBirthDate() error = %v, wantErr %v", err, tt.wantErr)
			}

			if err != nil {
				return
			}

			if tt.want == "" {
				if parsed != nil {
					t.Fatalf("parsed date = %v, want nil", parsed)
				}
				return
			}

			if parsed == nil {
				t.Fatal("parsed date is nil")
			}

			if got := parsed.Format("2006-01-02"); got != tt.want {
				t.Fatalf("parsed date = %q, want %q", got, tt.want)
			}
		})
	}
}
