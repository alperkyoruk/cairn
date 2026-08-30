package httpapi

import (
	"net/http"
	"sync"
	"time"
)

// Password checking is deliberately expensive -- argon2id at 64MB and three
// passes -- which is the right call for a stored hash and the wrong thing to
// expose without a limit. Unlimited attempts would be a password oracle and a
// memory exhaustion vector at the same time: fifty concurrent guesses is three
// gigabytes of RAM on a box that is probably running other things.
//
// Because Cairn has exactly one user, this can be a single global counter
// rather than a per-IP one. There is no fairness question to answer, nothing to
// key on, and no need to decide which proxy header to trust. One person cannot
// fail ten logins in five minutes by accident, and an attacker gets ten guesses
// per five minutes no matter how many hosts they come from.
type throttle struct {
	mu       sync.Mutex
	failures []time.Time
	limit    int
	window   time.Duration
	now      func() time.Time
}

func newThrottle(limit int, window time.Duration) *throttle {
	return &throttle{limit: limit, window: window, now: time.Now}
}

// allow reports whether another attempt may be made.
func (t *throttle) allow() bool {
	t.mu.Lock()
	defer t.mu.Unlock()
	t.prune()
	return len(t.failures) < t.limit
}

// fail records a rejected attempt.
func (t *throttle) fail() {
	t.mu.Lock()
	defer t.mu.Unlock()
	t.prune()
	t.failures = append(t.failures, t.now())
}

// succeed clears the record: whoever it was, they knew the password.
func (t *throttle) succeed() {
	t.mu.Lock()
	defer t.mu.Unlock()
	t.failures = nil
}

// retryAfter reports how long until the oldest failure leaves the window.
func (t *throttle) retryAfter() time.Duration {
	t.mu.Lock()
	defer t.mu.Unlock()
	t.prune()
	if len(t.failures) == 0 {
		return 0
	}
	return t.window - t.now().Sub(t.failures[0])
}

func (t *throttle) prune() {
	cutoff := t.now().Add(-t.window)
	kept := t.failures[:0]
	for _, at := range t.failures {
		if at.After(cutoff) {
			kept = append(kept, at)
		}
	}
	t.failures = kept
}

// writeThrottled refuses an attempt. This is the one failure the service layer
// knows nothing about, because how often a request may be made is a property of
// the front door rather than of the rules behind it.
func writeThrottled(w http.ResponseWriter, retryAfter time.Duration) {
	seconds := int(retryAfter.Seconds()) + 1
	w.Header().Set("Retry-After", itoa(seconds))

	var body errorBody
	body.Error.Kind = "too_many_attempts"
	body.Error.Message = "too many failed sign-in attempts; wait a few minutes and try again"
	writeJSON(w, http.StatusTooManyRequests, body)
}

func itoa(n int) string {
	if n <= 0 {
		return "1"
	}
	var b [20]byte
	i := len(b)
	for n > 0 {
		i--
		b[i] = byte('0' + n%10)
		n /= 10
	}
	return string(b[i:])
}
