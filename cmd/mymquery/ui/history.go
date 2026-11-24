package ui

import (
	"strings"

	"mquery/cmd/mymquery/storage"
)

const DefaultHistorySize = 100

// queryHistory keeps submitted queries (oldest first, no duplicates).
// When db is set, queries are stored there.
type queryHistory struct {
	db      *storage.DB
	maxSize int
	items   []string
}

func newQueryHistory(db *storage.DB, maxSize int) *queryHistory {
	return &queryHistory{db: db, maxSize: max(1, maxSize)}
}

// Load reads recent queries from the database
func (h *queryHistory) Load() error {
	if h.db == nil {
		return nil
	}
	items, err := h.db.RecentQueries(h.maxSize)
	if err != nil {
		return err
	}
	for _, q := range items {
		h.add(q)
	}
	return nil
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

// Add stores a query as the most recent one (errors are returned but
// the in-memory history is updated anyway). The database logs every
// submitted query, i.e. repeated ones too.
func (h *queryHistory) Add(corpusID, feature, q string) error {
	h.add(q)
	q = normalizeHistoryItem(q)
	if h.db == nil || q == "" {
		return nil
	}
	return h.db.AddQuery(corpusID, feature, q)
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
