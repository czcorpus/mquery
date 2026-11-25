package llm

import (
	"sync"
	"time"
)

// fallbackRecoveryInterval is how long a backend stays on a fallback
// model after a retryable error before it tries the primary model again.
const fallbackRecoveryInterval = 5 * time.Minute

// modelSet is a primary model with optional fallbacks. On a retryable error
// (e.g. a 503 from an overloaded model), a backend switches to the next model
// (if any) and it switches back to the primary model (models[0]) once
// fallbackRecoveryInterval has passed. It is safe for concurrent use.
type modelSet struct {
	mu               sync.Mutex
	models           []string
	activeIdx        int
	primaryDownSince time.Time
	// now is time.Now (replaceable in tests)
	now func() time.Time
}

func newModelSet(models []string) *modelSet {
	return &modelSet{models: models, now: time.Now}
}

// active returns the currently active model
func (s *modelSet) active() string {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.models[s.activeIdx]
}

// current returns the model the next request should use, switching
// back to the primary model first if fallbackRecoveryInterval has passed
// since the last fallback.
func (s *modelSet) current() string {
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.activeIdx != 0 && s.now().Sub(s.primaryDownSince) >= fallbackRecoveryInterval {
		s.activeIdx = 0
		s.primaryDownSince = time.Time{}
	}
	return s.models[s.activeIdx]
}

// switchToFallback moves to the next fallback model (cycling among
// models[1:] if there's more than one). It is a no-op if no fallback
// is configured.
func (s *modelSet) switchToFallback() {
	s.mu.Lock()
	defer s.mu.Unlock()
	if len(s.models) < 2 {
		return
	}
	if s.activeIdx == 0 {
		s.primaryDownSince = s.now()
		s.activeIdx = 1

	} else {
		s.activeIdx = s.activeIdx%(len(s.models)-1) + 1
	}
}
