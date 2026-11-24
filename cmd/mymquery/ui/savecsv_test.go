package ui

import (
	"os"
	"path/filepath"
	"regexp"
	"testing"

	"mquery/cmd/mymquery/api"

	tea "github.com/charmbracelet/bubbletea"
)

func TestCSVFileName(t *testing.T) {
	a := api.FreqsArgs{Query: `[lemma="dog"]`, Attr: "word", MaxItems: 50}
	n1 := csvFileName("token-freqs", "bnc", a)
	if !regexp.MustCompile(`^token-freqs-[0-9a-f]{10}\.csv$`).MatchString(n1) {
		t.Fatalf("unexpected name %q", n1)
	}
	if n2 := csvFileName("token-freqs", "bnc", a); n2 != n1 {
		t.Errorf("name is not deterministic: %q vs %q", n1, n2)
	}
	b := a
	b.MatchCase = true
	if csvFileName("token-freqs", "bnc", b) == n1 {
		t.Error("different args must give a different name")
	}
	if csvFileName("token-freqs", "syn", a) == n1 {
		t.Error("different corpus must give a different name")
	}
}

func TestCSVDialogWrite(t *testing.T) {
	path := filepath.Join(t.TempDir(), "out.csv")
	rows := [][]string{{"word", "freq"}, {`a "b", c`, "3"}}
	enter := tea.KeyMsg{Type: tea.KeyEnter}

	d := newCSVDialog("t", path, rows)
	res, got := d.Update(enter)
	if res != selectChosen || got != path {
		t.Fatalf("expected the file to be written, got %v %q (err %q)", res, got, d.err)
	}
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	if want := "word,freq\n\"a \"\"b\"\", c\",3\n"; string(data) != want {
		t.Errorf("unexpected content %q", data)
	}

	// an existing file requires a second enter
	d = newCSVDialog("t", path, [][]string{{"x"}})
	if res, _ := d.Update(enter); res != selectPending || d.err == "" {
		t.Fatalf("expected an overwrite warning, got %v", res)
	}
	if res, _ := d.Update(enter); res != selectChosen {
		t.Fatalf("expected overwrite on the second enter, got %v (err %q)", res, d.err)
	}
	if data, _ := os.ReadFile(path); string(data) != "x\n" {
		t.Errorf("file not overwritten: %q", data)
	}

	if res, _ := newCSVDialog("t", path, rows).Update(tea.KeyMsg{Type: tea.KeyEsc}); res != selectCancelled {
		t.Errorf("expected cancel, got %v", res)
	}
}

func TestExpandPath(t *testing.T) {
	home, _ := os.UserHomeDir()
	if p, _ := expandPath("~/x.csv"); p != filepath.Join(home, "x.csv") {
		t.Errorf("unexpected %q", p)
	}
	if _, err := expandPath(""); err == nil {
		t.Error("expected an error for an empty path")
	}
}
