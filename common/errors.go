package common

import "sync"

// ErrorThreshold is the number of consecutive failures before an alert fires.
const ErrorThreshold = 3

// ErrorTracker counts consecutive failures per key and reports when a failure streak
// reaches ErrorThreshold. Callers use it to alert (e.g. via Discord) when an
// operation keeps failing, while avoiding repeated alerts during the same streak.
type ErrorTracker struct {
	mu       sync.Mutex
	counters map[string]int
}

// NewErrorTracker creates an ErrorTracker with no recorded failures.
func NewErrorTracker() *ErrorTracker {
	return &ErrorTracker{counters: make(map[string]int)}
}

// Fail records a failure for key. It returns true only when the consecutive failure
// streak first reaches ErrorThreshold; subsequent failures return false until Success
// resets the counter, so the caller alerts exactly once per streak.
func (t *ErrorTracker) Fail(key string) bool {
	t.mu.Lock()
	defer t.mu.Unlock()
	t.counters[key]++
	return t.counters[key] == ErrorThreshold
}

// Success records a successful call for key and resets its failure streak.
func (t *ErrorTracker) Success(key string) {
	t.mu.Lock()
	defer t.mu.Unlock()
	delete(t.counters, key)
}
