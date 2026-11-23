package ui

import (
	"bufio"
	"errors"
	"io/fs"
	"os"
	"path/filepath"
	"strings"
)

const DefaultHistorySize = 100

// DefaultHistoryPath returns $XDG_STATE_HOME/mymquery/history
// (~/.local/state/mymquery/history when XDG_STATE_HOME is not set)
func DefaultHistoryPath() string {
	dir := os.Getenv("XDG_STATE_HOME")
	if dir == "" {
		home, err := os.UserHomeDir()
		if err != nil {
			return ""
		}
		dir = filepath.Join(home, ".local", "state")
	}
	return filepath.Join(dir, "mymquery", "history")
}

// queryHistory keeps submitted queries (oldest first, no duplicates).
// When path is set, it is stored as a plain text file, one query per line.
type queryHistory struct {
	path    string
	maxSize int
	items   []string
}

func newQueryHistory(path string, maxSize int) *queryHistory {
	return &queryHistory{path: path, maxSize: max(1, maxSize)}
}

// Load reads the history file; a missing file is not an error
func (h *queryHistory) Load() error {
	if h.path == "" {
		return nil
	}
	f, err := os.Open(h.path)
	if errors.Is(err, fs.ErrNotExist) {
		return nil
	} else if err != nil {
		return err
	}
	defer f.Close()
	sc := bufio.NewScanner(f)
	sc.Buffer(make([]byte, 0, 64*1024), 1024*1024)
	for sc.Scan() {
		h.add(sc.Text())
	}
	return sc.Err()
}

func (h *queryHistory) add(q string) bool {
	q = normalizeHistoryItem(q)
	if q == "" {
		return false
	}
	if n := len(h.items); n > 0 && h.items[n-1] == q {
		return false
	}
	for i, v := range h.items {
		if v == q {
			h.items = append(h.items[:i], h.items[i+1:]...)
			break
		}
	}
	h.items = append(h.items, q)
	if len(h.items) > h.maxSize {
		h.items = h.items[len(h.items)-h.maxSize:]
	}
	return true
}

// Add stores a query as the most recent one and saves the history
// (errors are returned but the in-memory history is updated anyway)
func (h *queryHistory) Add(q string) error {
	if !h.add(q) || h.path == "" {
		return nil
	}
	return h.save()
}

func (h *queryHistory) save() error {
	if err := os.MkdirAll(filepath.Dir(h.path), 0o755); err != nil {
		return err
	}
	tmp := h.path + ".tmp"
	if err := os.WriteFile(tmp, []byte(strings.Join(h.items, "\n")+"\n"), 0o600); err != nil {
		return err
	}
	return os.Rename(tmp, h.path)
}

func (h *queryHistory) Len() int {
	return len(h.items)
}

// Recent returns i-th most recent query (0 = the last one)
func (h *queryHistory) Recent(i int) string {
	return h.items[len(h.items)-1-i]
}

func normalizeHistoryItem(q string) string {
	q = strings.ReplaceAll(q, "\r", " ")
	q = strings.ReplaceAll(q, "\n", " ")
	return strings.TrimSpace(q)
}
