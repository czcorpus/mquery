package llm

import (
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
)

func TestModelSetFallback(t *testing.T) {
	now := time.Date(2026, 11, 25, 12, 0, 0, 0, time.UTC)
	s := newModelSet([]string{"a", "b", "c"})
	s.now = func() time.Time { return now }

	assert.Equal(t, "a", s.current())
	s.switchToFallback()
	assert.Equal(t, "b", s.current())
	s.switchToFallback()
	assert.Equal(t, "c", s.active())
	s.switchToFallback() // fallbacks cycle, the primary is not tried yet
	assert.Equal(t, "b", s.current())

	now = now.Add(fallbackRecoveryInterval)
	assert.Equal(t, "a", s.current())

	single := newModelSet([]string{"x"})
	single.switchToFallback()
	assert.Equal(t, "x", single.current())
}
