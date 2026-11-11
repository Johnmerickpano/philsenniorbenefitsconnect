package beneficiary

import (
	"reflect"
	"testing"

	"github.com/Johnmerickpano/philsenniorbenefitsconnect/internal/database/schema"
)

func stringPointer(value string) *string {
	return &value
}

func TestNormalizeSearchText(t *testing.T) {
	tests := []struct {
		name  string
		input string
		want  string
	}{
		{"accented name", "Jos\u00e9", "jose"},
		{"uppercase and whitespace", "  MAR\u00cdA   DEL   CARMEN  ", "maria del carmen"},
		{"decomposed accent", "Cafe\u0301", "cafe"},
		{"empty string", "", ""},
		{"non-Latin text", "\u5c71\u7530", "\u5c71\u7530"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := normalizeSearchText(tt.input); got != tt.want {
				t.Errorf("normalizeSearchText(%q) = %q, want %q", tt.input, got, tt.want)
			}
		})
	}
}

func TestDeriveSortName(t *testing.T) {
	tests := []struct {
		name  string
		input *schema.Beneficiary
		want  string
	}{
		{
			name: "last first middle",
			input: &schema.Beneficiary{
				FirstName:  stringPointer("Jos\u00e9"),
				MiddleName: stringPointer("Dela Cruz"),
				LastName:   stringPointer("Santos"),
				Suffix:     stringPointer("Jr."),
			},
			want: "santos jose dela cruz",
		},
		{
			name: "optional names",
			input: &schema.Beneficiary{
				FirstName: stringPointer("Ana"),
			},
			want: "ana",
		},
		{
			name:  "nil beneficiary",
			input: nil,
			want:  "",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := deriveSortName(tt.input); got != tt.want {
				t.Errorf("deriveSortName() = %q, want %q", got, tt.want)
			}
		})
	}
}

func TestDisplayName(t *testing.T) {
	tests := []struct {
		name  string
		input *schema.Beneficiary
		want  *string
	}{
		{
			name: "full display name",
			input: &schema.Beneficiary{
				FirstName:  stringPointer("Jos\u00e9"),
				MiddleName: stringPointer("Dela Cruz"),
				LastName:   stringPointer("Santos"),
				Suffix:     stringPointer("Jr."),
			},
			want: stringPointer("Jos\u00e9 Dela Cruz Santos Jr."),
		},
		{
			name: "missing optional names",
			input: &schema.Beneficiary{
				FirstName: stringPointer("Ana"),
			},
			want: stringPointer("Ana"),
		},
		{
			name:  "no beneficiary",
			input: nil,
			want:  nil,
		},
		{
			name:  "no name fields",
			input: &schema.Beneficiary{},
			want:  nil,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := displayName(tt.input)
			if !reflect.DeepEqual(got, tt.want) {
				t.Errorf("displayName() = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestNameSearchPrefixes(t *testing.T) {
	tests := []struct {
		name  string
		input *schema.Beneficiary
		want  []string
	}{
		{
			name: "three name fields",
			input: &schema.Beneficiary{
				FirstName:  stringPointer("\u00c9lodie"),
				MiddleName: stringPointer("O'Neil"),
				LastName:   stringPointer("Santos"),
			},
			want: []string{"elo", "one", "san"},
		},
		{
			name: "duplicate prefixes",
			input: &schema.Beneficiary{
				FirstName:  stringPointer("Ana"),
				MiddleName: stringPointer("Anabel"),
				LastName:   stringPointer("Anacleto"),
			},
			want: []string{"ana"},
		},
		{
			name: "short name",
			input: &schema.Beneficiary{
				FirstName: stringPointer("Li"),
			},
			want: []string{"li"},
		},
		{
			name:  "nil beneficiary",
			input: nil,
			want:  nil,
		},
		{
			name:  "no names",
			input: &schema.Beneficiary{},
			want:  []string{},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := nameSearchPrefixes(tt.input)
			if !reflect.DeepEqual(got, tt.want) {
				t.Errorf("nameSearchPrefixes() = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestSearchNamePrefix(t *testing.T) {
	tests := []struct {
		name  string
		input string
		want  string
	}{
		{name: "normal name", input: "Santos", want: "san"},
		{name: "accented name", input: "\u00c9lodie", want: "elo"},
		{name: "uppercase name", input: "MARIA", want: "mar"},
		{name: "spaces and punctuation", input: "  O'Neil-Santos ", want: "one"},
		{name: "digits and letters", input: "AB-123", want: "ab1"},
		{name: "non-Latin name", input: "\u5c71\u7530\u592a\u90ce", want: "\u5c71\u7530\u592a"},
		{name: "short input", input: "Li", want: ""},
		{name: "punctuation only", input: "---", want: ""},
		{name: "empty input", input: "", want: ""},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := searchNamePrefix(tt.input); got != tt.want {
				t.Errorf("searchNamePrefix(%q) = %q, want %q", tt.input, got, tt.want)
			}
		})
	}
}

// TestNormalizeNameSearchKey verifies accent and punctuation normalization.
func TestNormalizeNameSearchKey(t *testing.T) {
	tests := []struct {
		name  string
		input string
		want  string
	}{
		{"accented name", "Jos\u00e9", "jose"},
		{"uppercase name", "MARIA", "maria"},
		{"punctuation", "O'Neil-Santos", "oneilsantos"},
		{"spaces", "Mary Jane", "maryjane"},
		{"digits", "AB-123", "ab123"},
		{"non-Latin", "\u5c71\u7530\u592a\u90ce", "\u5c71\u7530\u592a\u90ce"},
		{"empty", "", ""},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := normalizeNameSearchKey(tt.input); got != tt.want {
				t.Errorf("normalizeNameSearchKey(%q) = %q, want %q", tt.input, got, tt.want)
			}
		})
	}
}

// TestMatchesNameSearch verifies component-prefix matching.
func TestMatchesNameSearch(t *testing.T) {
	display := "Jos\u00e9 Maria Santos"

	tests := []struct {
		name        string
		displayName *string
		searchKey   string
		want        bool
	}{
		{"first name prefix", stringPointer(display), "jose", true},
		{"middle name prefix", stringPointer(display), "mar", true},
		{"last name prefix", stringPointer(display), "san", true},
		{"case insensitive", stringPointer(display), "JOSE", true},
		{"accent insensitive", stringPointer(display), "jose", true},
		{"not a prefix", stringPointer(display), "aria", false},
		{"empty search", stringPointer(display), "", false},
		{"nil display name", nil, "jose", false},
		{"punctuation in name", stringPointer("O'Neil Santos"), "one", true},
		{"hyphenated name", stringPointer("Dela-Cruz Santos"), "delacruz", true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := matchesNameSearch(tt.displayName, tt.searchKey); got != tt.want {
				t.Errorf("matchesNameSearch(%v, %q) = %v, want %v",
					tt.displayName, tt.searchKey, got, tt.want)
			}
		})
	}
}
