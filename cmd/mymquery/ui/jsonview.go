package ui

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"math"
	"strconv"
	"strings"
	"unicode"

	"github.com/charmbracelet/lipgloss"
	"github.com/charmbracelet/x/ansi"
)

// jsonHiddenKeys are keys of no interest for users (e.g. MQuery's
// result type identifiers)
var jsonHiddenKeys = map[string]bool{"resultType": true}

// jsonNode is a parsed JSON value which keeps the order of object keys
type jsonNode struct {
	// keys and values of an object (isObject)
	isObject bool
	keys     []string
	values   []*jsonNode
	// items of an array (isArray)
	isArray bool
	items   []*jsonNode
	// scalar is a json.Number, string, bool or nil
	scalar any
}

func (n *jsonNode) isScalar() bool {
	return !n.isObject && !n.isArray
}

// parseOrderedJSON parses a single JSON value
func parseOrderedJSON(data string) (*jsonNode, error) {
	dec := json.NewDecoder(strings.NewReader(data))
	dec.UseNumber()
	n, err := decodeJSONNode(dec)
	if err != nil {
		return nil, err
	}
	if _, err := dec.Token(); err != io.EOF {
		return nil, fmt.Errorf("unexpected data after a JSON value")
	}
	return n, nil
}

func decodeJSONNode(dec *json.Decoder) (*jsonNode, error) {
	tok, err := dec.Token()
	if err != nil {
		return nil, err
	}
	switch tok {
	case json.Delim('{'):
		n := &jsonNode{isObject: true}
		for dec.More() {
			key, err := dec.Token()
			if err != nil {
				return nil, err
			}
			val, err := decodeJSONNode(dec)
			if err != nil {
				return nil, err
			}
			n.keys = append(n.keys, fmt.Sprint(key))
			n.values = append(n.values, val)
		}
		_, err := dec.Token() // }
		return n, err
	case json.Delim('['):
		n := &jsonNode{isArray: true}
		for dec.More() {
			item, err := decodeJSONNode(dec)
			if err != nil {
				return nil, err
			}
			n.items = append(n.items, item)
		}
		_, err := dec.Token() // ]
		return n, err
	}
	return &jsonNode{scalar: tok}, nil
}

// visibleFields returns indices of the object's fields to be shown
func (n *jsonNode) visibleFields() []int {
	var ans []int
	for i, k := range n.keys {
		if !jsonHiddenKeys[k] {
			ans = append(ans, i)
		}
	}
	return ans
}

// renderJSON renders a JSON value in a readable form: objects as "key value"
// lines (nested values below their keys), arrays of records as tables
func renderJSON(n *jsonNode, width int) string {
	return strings.Join(renderJSONLines(n, max(20, width)), "\n")
}

func renderJSONLines(n *jsonNode, width int) []string {
	switch {
	case n.isObject:
		return renderJSONObject(n, width)
	case n.isArray:
		return renderJSONArray(n, width)
	}
	return strings.Split(ansi.Wordwrap(formatJSONScalar(n.scalar), width, " "), "\n")
}

func renderJSONObject(n *jsonNode, width int) []string {
	fields := n.visibleFields()
	if len(fields) == 0 {
		return []string{mutedStyle.Render("(empty)")}
	}
	// scalar fields first (aligned "key value" lines), then nested values
	keyW := 0
	for _, i := range fields {
		if n.values[i].isScalar() {
			keyW = max(keyW, ansi.StringWidth(humanizeJSONKey(n.keys[i])))
		}
	}
	keyW = min(keyW, width/3)
	var lines []string
	for _, i := range fields {
		v := n.values[i]
		if !v.isScalar() {
			continue
		}
		valW := max(10, width-keyW-2)
		val := strings.Split(ansi.Wordwrap(formatJSONScalar(v.scalar), valW, " "), "\n")
		key := mutedStyle.Render(padRight(ansi.Truncate(humanizeJSONKey(n.keys[i]), keyW, "…"), keyW))
		lines = append(lines, key+"  "+val[0])
		for _, l := range val[1:] {
			lines = append(lines, strings.Repeat(" ", keyW+2)+l)
		}
	}
	for _, i := range fields {
		v := n.values[i]
		if v.isScalar() {
			continue
		}
		if len(lines) > 0 {
			lines = append(lines, "")
		}
		lines = append(lines, accentStyle.Bold(true).Render(humanizeJSONKey(n.keys[i])))
		for _, l := range renderJSONLines(v, width-2) {
			lines = append(lines, "  "+l)
		}
	}
	return lines
}

func renderJSONArray(n *jsonNode, width int) []string {
	if len(n.items) == 0 {
		return []string{mutedStyle.Render("(empty)")}
	}
	if cols, ok := recordColumns(n.items); ok {
		return strings.Split(renderJSONTable(n.items, cols, width), "\n")
	}
	allScalars := true
	for _, it := range n.items {
		allScalars = allScalars && it.isScalar()
	}
	if allScalars {
		vals := make([]string, len(n.items))
		for i, it := range n.items {
			vals[i] = formatJSONScalar(it.scalar)
		}
		return strings.Split(ansi.Wordwrap(strings.Join(vals, ", "), width, " "), "\n")
	}
	// mixed items: a bulleted list
	var lines []string
	for _, it := range n.items {
		for j, l := range renderJSONLines(it, width-2) {
			if j == 0 {
				lines = append(lines, mutedStyle.Render("• ")+l)
			} else {
				lines = append(lines, "  "+l)
			}
		}
	}
	return lines
}

// recordColumns returns columns of a table if all the items are objects
// with scalar values only (the keys in order of appearance)
func recordColumns(items []*jsonNode) ([]string, bool) {
	var cols []string
	seen := make(map[string]bool)
	for _, it := range items {
		if !it.isObject {
			return nil, false
		}
		for _, i := range it.visibleFields() {
			if !it.values[i].isScalar() {
				return nil, false
			}
			if k := it.keys[i]; !seen[k] {
				seen[k] = true
				cols = append(cols, k)
			}
		}
	}
	return cols, len(cols) > 0
}

func renderJSONTable(items []*jsonNode, cols []string, width int) string {
	header := make([]string, len(cols))
	numeric := make([]bool, len(cols))
	for c, k := range cols {
		header[c] = humanizeJSONKey(k)
		numeric[c] = true
	}
	rows := make([][]string, len(items))
	for r, it := range items {
		rows[r] = make([]string, len(cols))
		for i, k := range it.keys {
			c := indexOf(cols, k)
			if c < 0 {
				continue
			}
			rows[r][c] = formatJSONScalar(it.values[i].scalar)
			if _, isNum := it.values[i].scalar.(json.Number); !isNum {
				numeric[c] = false
			}
		}
	}
	aligns := make([]lipgloss.Position, len(cols))
	for c := range cols {
		if numeric[c] {
			aligns[c] = lipgloss.Right
		}
	}
	return renderTable(header, rows, aligns, width)
}

func indexOf(items []string, s string) int {
	for i, v := range items {
		if v == s {
			return i
		}
	}
	return -1
}

// formatJSONScalar formats a scalar value for reading (numbers with
// thousands separators, floats rounded)
func formatJSONScalar(v any) string {
	switch v := v.(type) {
	case nil:
		return "—"
	case json.Number:
		if i, err := v.Int64(); err == nil {
			return formatInt(int(i))
		}
		f, err := v.Float64()
		if err != nil {
			return v.String()
		}
		if math.Abs(f) >= 1 {
			return formatFloat(f)
		}
		return strconv.FormatFloat(f, 'g', 4, 64)
	case string:
		return v
	}
	return fmt.Sprint(v)
}

// formatFloat formats a number with 2 decimal places and thousands separators
func formatFloat(f float64) string {
	s := strconv.FormatFloat(math.Abs(f), 'f', 2, 64)
	intPart, frac, _ := strings.Cut(s, ".")
	n, _ := strconv.Atoi(intPart)
	ans := formatInt(n) + "." + frac
	if f < 0 {
		ans = "-" + ans
	}
	return ans
}

// humanizeJSONKey makes a key readable: "corpusSize" => "corpus size",
// "text-type" => "text type"
func humanizeJSONKey(k string) string {
	var b strings.Builder
	runes := []rune(k)
	for i, r := range runes {
		switch {
		case r == '-' || r == '_':
			b.WriteRune(' ')
		case unicode.IsUpper(r) && i > 0 && unicode.IsLower(runes[i-1]):
			b.WriteRune(' ')
			b.WriteRune(unicode.ToLower(r))
		default:
			b.WriteRune(r)
		}
	}
	return b.String()
}

// prettyJSON returns indented JSON (or the original text if it is not JSON)
func prettyJSON(text string) string {
	var buf bytes.Buffer
	if json.Indent(&buf, []byte(text), "", "  ") == nil {
		return buf.String()
	}
	return text
}
