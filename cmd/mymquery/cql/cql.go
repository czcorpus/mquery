// Package cql contains small helpers for building CQL queries
// on the client side (e.g. "show concordance for this frequency item").
package cql

import (
	"fmt"
	"regexp"
	"strings"
	"unicode"
)

// EscapeValue turns a literal string into a CQL regexp value
// (without the surrounding quotes)
func EscapeValue(s string) string {
	return strings.ReplaceAll(regexp.QuoteMeta(s), `"`, `\"`)
}

// AttrCond creates an `attr="value"` condition matching the literal value.
func AttrCond(attr, value string, ignoreCase bool) string {
	v := EscapeValue(value)
	if ignoreCase {
		v = "(?i)" + v
	}
	return attr + `="` + v + `"`
}

// SplitPositions splits a query consisting only of a sequence of bracketed
// position expressions (e.g. `[lemma="a"] [tag="N.*"] []`) and returns
// their inner contents. For anything more complex (repetitions, `within`,
// alternatives, labels, structures, ...) ok is false.
func SplitPositions(q string) (positions []string, ok bool) {
	i := 0
	for {
		for i < len(q) && unicode.IsSpace(rune(q[i])) {
			i++
		}
		if i == len(q) {
			return positions, len(positions) > 0
		}
		if q[i] != '[' {
			return nil, false
		}
		end := findClosingBracket(q, i)
		if end < 0 {
			return nil, false
		}
		positions = append(positions, strings.TrimSpace(q[i+1:end]))
		i = end + 1
		// position must be followed by a space, another position or the end
		if i < len(q) && q[i] != '[' && !unicode.IsSpace(rune(q[i])) {
			return nil, false
		}
	}
}

// findClosingBracket returns the index of the `]` matching
// the `[` at `start` (quoted strings are skipped) or -1
func findClosingBracket(q string, start int) int {
	depth := 0
	inQuote := false
	for i := start; i < len(q); i++ {
		c := q[i]
		switch {
		case inQuote && c == '\\':
			i++ // skip escaped char
		case c == '"':
			inQuote = !inQuote
		case inQuote:
		case c == '[':
			depth++
		case c == ']':
			depth--
			if depth == 0 {
				return i
			}
		}
	}
	return -1
}

// RestrictByValues creates a query matching the same positions as `query`
// with an additional condition `attr=values[i]` for each position. If the
// query cannot be merged (it is too complex or the number of its positions
// differs from the number of values), the result contains only the value
// conditions and `merged` is false.
func RestrictByValues(query, attr string, values []string, ignoreCase bool) (ans string, merged bool) {
	positions, ok := SplitPositions(query)
	merged = ok && len(positions) == len(values)
	parts := make([]string, len(values))
	for i, v := range values {
		cond := AttrCond(attr, v, ignoreCase)
		if merged && positions[i] != "" {
			cond = "(" + positions[i] + ") & " + cond
		}
		parts[i] = "[" + cond + "]"
	}
	return strings.Join(parts, " "), merged
}

// WithinStructAttr restricts a query to structures with the given attribute
// value, e.g. `(q) within <doc author="X"/>`. The structAttr must be in
// the `struct.attr` form.
func WithinStructAttr(query, structAttr, value string) (string, error) {
	st, attr, ok := strings.Cut(structAttr, ".")
	if !ok || st == "" || attr == "" {
		return "", fmt.Errorf("invalid structural attribute %q (expected struct.attr)", structAttr)
	}
	return fmt.Sprintf(`(%s) within <%s %s="%s"/>`, query, st, attr, EscapeValue(value)), nil
}
