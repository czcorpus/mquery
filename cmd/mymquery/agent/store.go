package agent

import (
	"fmt"
	"strconv"
	"strings"
	"sync"
	"time"
)

// Result is a stored outcome of a single tool call. Results are kept for
// the whole session so the user can open any of them without requerying.
// A Result is immutable once added to a Store.
type Result struct {
	// ID is a short session-unique identifier (r1, r2, ...) the model
	// uses to refer to the result in its answers.
	ID   string
	Tool string
	// Args are the decoded tool call arguments
	Args map[string]any
	// Content is the full (i.e. not truncated) tool output
	Content string
	// OK is false for failed calls (Content then contains an error message)
	OK   bool
	Time time.Time
}

// StringArg returns a string argument (or "" if missing/of a different type)
func (r *Result) StringArg(name string) string {
	if v, ok := r.Args[name].(string); ok {
		return v
	}
	return ""
}

// IntArg returns an integer argument (or dflt if missing/of a different type)
func (r *Result) IntArg(name string, dflt int) int {
	switch v := r.Args[name].(type) {
	case float64:
		return int(v)
	case int:
		return v
	}
	return dflt
}

// BoolArg returns a boolean argument (or false if missing/of a different type)
func (r *Result) BoolArg(name string) bool {
	v, _ := r.Args[name].(bool)
	return v
}

// Store holds tool call results of a session. It is safe for concurrent use.
type Store struct {
	mu    sync.Mutex
	items []*Result
	// lastNum is the number of the last assigned ID (r<lastNum>)
	lastNum int
}

func NewStore() *Store {
	return &Store{}
}

// NewStoreFrom creates a store with previously saved results
// (new results get IDs following the highest one)
func NewStoreFrom(results []*Result) *Store {
	s := &Store{items: append([]*Result(nil), results...)}
	for _, r := range results {
		if n, err := strconv.Atoi(strings.TrimPrefix(r.ID, "r")); err == nil {
			s.lastNum = max(s.lastNum, n)
		}
	}
	return s
}

// Add stores a new result and assigns it an ID
func (s *Store) Add(tool string, args map[string]any, content string, ok bool) *Result {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.lastNum++
	r := &Result{
		ID:      fmt.Sprintf("r%d", s.lastNum),
		Tool:    tool,
		Args:    args,
		Content: content,
		OK:      ok,
		Time:    time.Now(),
	}
	s.items = append(s.items, r)
	return r
}

// Get returns a result by its ID
func (s *Store) Get(id string) (*Result, bool) {
	s.mu.Lock()
	defer s.mu.Unlock()
	for _, r := range s.items {
		if r.ID == id {
			return r, true
		}
	}
	return nil, false
}

// All returns all the results in the order they were added
func (s *Store) All() []*Result {
	s.mu.Lock()
	defer s.mu.Unlock()
	ans := make([]*Result, len(s.items))
	copy(ans, s.items)
	return ans
}
