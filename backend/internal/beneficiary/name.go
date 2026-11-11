package beneficiary

import (
	"strings"
	"unicode"

	"golang.org/x/text/unicode/norm"

	"github.com/Johnmerickpano/philsenniorbenefitsconnect/internal/database/schema"
)

func normalizeSearchText(value string) string {
	decomposed := norm.NFD.String(value)

	var normalized strings.Builder
	for _, r := range decomposed {
		if unicode.Is(unicode.Mn, r) {
			continue
		}
		normalized.WriteRune(unicode.ToLower(r))
	}

	return strings.Join(strings.Fields(normalized.String()), " ")
}

// Sort order: last name, first name, middle name. Suffix is excluded.
func deriveSortName(b *schema.Beneficiary) string {
	if b == nil {
		return ""
	}

	parts := []string{
		optionalStringValue(b.LastName),
		optionalStringValue(b.FirstName),
		optionalStringValue(b.MiddleName),
	}

	normalized := make([]string, 0, len(parts))
	for _, part := range parts {
		if part = normalizeSearchText(part); part != "" {
			normalized = append(normalized, part)
		}
	}

	return strings.Join(normalized, " ")
}

// Display order: first name, middle name, last name, suffix.
func displayName(b *schema.Beneficiary) *string {
	if b == nil {
		return nil
	}

	parts := []string{
		optionalStringValue(b.FirstName),
		optionalStringValue(b.MiddleName),
		optionalStringValue(b.LastName),
		optionalStringValue(b.Suffix),
	}

	clean := make([]string, 0, len(parts))
	for _, part := range parts {
		if part = strings.TrimSpace(part); part != "" {
			clean = append(clean, part)
		}
	}

	if len(clean) == 0 {
		return nil
	}

	value := strings.Join(clean, " ")
	return &value
}

// Returns unique prefixes from first, middle, and last name fields.
// Each prefix contains up to three Unicode letters or digits.
func nameSearchPrefixes(b *schema.Beneficiary) []string {
	if b == nil {
		return nil
	}

	parts := []*string{b.FirstName, b.MiddleName, b.LastName}
	seen := make(map[string]struct{}, len(parts))
	prefixes := make([]string, 0, len(parts))

	for _, part := range parts {
		if part == nil {
			continue
		}

		normalized := normalizeSearchText(*part)
		var prefix strings.Builder
		count := 0

		for _, r := range normalized {
			if !unicode.IsLetter(r) && !unicode.IsNumber(r) {
				continue
			}
			prefix.WriteRune(r)
			count++
			if count == 3 {
				break
			}
		}

		value := prefix.String()
		if value == "" {
			continue
		}
		if _, exists := seen[value]; exists {
			continue
		}

		seen[value] = struct{}{}
		prefixes = append(prefixes, value)
	}

	return prefixes
}

func optionalStringValue(value *string) string {
	if value == nil {
		return ""
	}
	return *value
}

// searchNamePrefix returns the first three normalized letters or digits.
// An empty result means the search term is too short.
func searchNamePrefix(value string) string {
	normalized := normalizeSearchText(value)

	var prefix strings.Builder
	count := 0

	for _, r := range normalized {
		if !unicode.IsLetter(r) && !unicode.IsNumber(r) {
			continue
		}

		prefix.WriteRune(r)
		count++

		if count == 3 {
			break
		}
	}

	if count < 3 {
		return ""
	}

	return prefix.String()
}

// normalizeNameSearchKey removes accents, spaces, and punctuation while
// retaining Unicode letters and digits for component-prefix matching.
func normalizeNameSearchKey(value string) string {
	normalized := normalizeSearchText(value)

	var key strings.Builder
	for _, r := range normalized {
		if unicode.IsLetter(r) || unicode.IsNumber(r) {
			key.WriteRune(r)
		}
	}

	return key.String()
}
