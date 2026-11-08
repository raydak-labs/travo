package uci

import (
	"strings"
	"unicode"
)

// SplitListValue splits a list option as UCI.GetSections/GetAll return it. The real
// reader joins elements with commas, the mock with spaces; accept both.
func SplitListValue(value string) []string {
	return strings.FieldsFunc(value, func(r rune) bool { return r == ',' || unicode.IsSpace(r) })
}
