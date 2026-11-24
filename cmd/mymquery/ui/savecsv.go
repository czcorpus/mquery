package ui

import (
	"bytes"
	"crypto/sha256"
	"encoding/csv"
	"encoding/hex"
	"errors"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"strconv"
	"strings"

	"github.com/charmbracelet/bubbles/cursor"
	"github.com/charmbracelet/bubbles/textinput"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
	"github.com/charmbracelet/x/ansi"
)

const (
	csvDialogMinWidth = 50
	csvDialogMaxWidth = 100
)

// csvFileName creates a default file name `<prefix>-<hash>.csv` where the hash
// identifies the corpus and all the arguments the result was calculated with
// (args is expected to be a flat API args struct incl. the query)
func csvFileName(prefix, corpusID string, args any) string {
	h := sha256.Sum256([]byte(corpusID + "\n" + fmt.Sprintf("%#v", args)))
	return fmt.Sprintf("%s-%s.csv", prefix, hex.EncodeToString(h[:])[:10])
}

func fmtCSVInt(n int64) string { return strconv.FormatInt(n, 10) }

func fmtCSVFloat(f float64) string { return strconv.FormatFloat(f, 'f', -1, 64) }

// pctOf returns `n` as a percentage of `total` (0 for total <= 0)
func pctOf(n, total int64) float64 {
	if total <= 0 {
		return 0
	}
	return float64(n) / float64(total) * 100
}

// csvDialog is a modal "save as CSV" window with a path input. The rows
// are a snapshot taken when the dialog is opened. It is meant to be embedded
// in a view which forwards keys to it and renders it via placeOverlay.
type csvDialog struct {
	title string
	rows  [][]string
	input textinput.Model
	err   string
	// overwrite is a path the user has already been warned about
	overwrite string
}

func newCSVDialog(title, defaultPath string, rows [][]string) *csvDialog {
	ti := textinput.New()
	ti.Prompt = "› "
	ti.PromptStyle = accentStyle
	ti.Placeholder = "file path"
	ti.Cursor.SetMode(cursor.CursorStatic)
	ti.SetValue(defaultPath)
	ti.CursorEnd()
	ti.Focus()
	return &csvDialog{title: title, rows: rows, input: ti}
}

// Update processes a key. On selectChosen, the file has been written
// and the returned value is its absolute path.
func (d *csvDialog) Update(msg tea.KeyMsg) (selectResult, string) {
	switch msg.String() {
	case "esc":
		return selectCancelled, ""
	case "enter":
		path, err := expandPath(strings.TrimSpace(d.input.Value()))
		if err != nil {
			d.err = err.Error()
			return selectPending, ""
		}
		if path != d.overwrite {
			if st, err := os.Stat(path); err == nil {
				if st.IsDir() {
					d.err = "the path is a directory"
					return selectPending, ""
				}
				d.overwrite = path
				d.err = "file exists – enter again to overwrite"
				return selectPending, ""
			} else if !errors.Is(err, fs.ErrNotExist) {
				d.err = err.Error()
				return selectPending, ""
			}
		}
		if err := writeCSV(path, d.rows); err != nil {
			d.err = err.Error()
			return selectPending, ""
		}
		return selectChosen, path
	}
	prev := d.input.Value()
	d.input, _ = d.input.Update(msg)
	if d.input.Value() != prev {
		d.err = ""
		d.overwrite = ""
	}
	return selectPending, ""
}

// expandPath expands a leading `~` and makes the path absolute
func expandPath(p string) (string, error) {
	if p == "" {
		return "", errors.New("enter a file path")
	}
	if p == "~" || strings.HasPrefix(p, "~/") {
		home, err := os.UserHomeDir()
		if err != nil {
			return "", err
		}
		p = filepath.Join(home, p[1:])
	}
	return filepath.Abs(p)
}

func writeCSV(path string, rows [][]string) error {
	var buf bytes.Buffer
	w := csv.NewWriter(&buf)
	if err := w.WriteAll(rows); err != nil {
		return err
	}
	return os.WriteFile(path, buf.Bytes(), 0o644)
}

func (d *csvDialog) View(maxWidth int) string {
	innerW := max(csvDialogMinWidth, min(ansi.StringWidth(d.input.Value())+6, maxWidth-6, csvDialogMaxWidth))
	d.input.Width = innerW - 3
	var b strings.Builder
	b.WriteString(titleStyle.Render(d.title) + "\n")
	b.WriteString(mutedStyle.Render(itemsCount(len(d.rows)-1)+" + header row, in the current order") + "\n\n")
	b.WriteString(d.input.View())
	if d.err != "" {
		b.WriteString("\n" + errorStyle.Render(ansi.Truncate(d.err, innerW, "…")))
	}
	return lipgloss.NewStyle().
		Border(lipgloss.RoundedBorder()).
		BorderForeground(colAccent).
		Padding(0, 1).
		Width(innerW + 2).
		Render(b.String())
}

func (d *csvDialog) Help() string {
	return "enter save • esc cancel • ~ = home directory, relative paths are relative to the working directory"
}

func savedInfo(path string, numItems int) string {
	return fmt.Sprintf("✓ %s saved to %s", itemsCount(numItems), path)
}

func itemsCount(n int) string {
	if n == 1 {
		return "1 item"
	}
	return fmt.Sprintf("%d items", n)
}
