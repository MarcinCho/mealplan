package auth

import (
	"sync"
	"time"
)

const limiterForget = time.Hour

// Limiter throttles repeated failures per key. State is in memory (per process).
type Limiter struct {
	mu      sync.Mutex
	now     func() time.Time
	maxFree int
	base    time.Duration
	max     time.Duration
	entries map[string]*limiterEntry
}

type limiterEntry struct {
	fails       int
	lockedUntil time.Time
	lastFail    time.Time
}

// NewLimiter locks a key from its maxFree-th consecutive failure on:
// 1s, doubling with each further failure, capped at 15 minutes.
func NewLimiter(now func() time.Time, maxFree int) *Limiter {
	return &Limiter{now: now, maxFree: maxFree, base: time.Second, max: 15 * time.Minute, entries: map[string]*limiterEntry{}}
}

func (l *Limiter) Allow(key string) (bool, time.Duration) {
	l.mu.Lock()
	defer l.mu.Unlock()
	e := l.entries[key]
	if e == nil {
		return true, 0
	}
	now := l.now()
	if now.Sub(e.lastFail) > limiterForget {
		delete(l.entries, key)
		return true, 0
	}
	if now.Before(e.lockedUntil) {
		return false, e.lockedUntil.Sub(now)
	}
	return true, 0
}

func (l *Limiter) Fail(key string) {
	l.mu.Lock()
	defer l.mu.Unlock()
	now := l.now()
	e := l.entries[key]
	if e == nil {
		e = &limiterEntry{}
		l.entries[key] = e
	}
	if e.fails > 0 && now.Sub(e.lastFail) > limiterForget {
		e.fails = 0
	}
	e.fails++
	e.lastFail = now
	if e.fails >= l.maxFree {
		shift := e.fails - l.maxFree
		if shift > 20 {
			shift = 20
		}
		d := l.base << uint(shift)
		if d > l.max {
			d = l.max
		}
		e.lockedUntil = now.Add(d)
	}
	if len(l.entries) > 10000 {
		for k, v := range l.entries {
			if now.Sub(v.lastFail) > limiterForget {
				delete(l.entries, k)
			}
		}
	}
}

func (l *Limiter) Reset(key string) {
	l.mu.Lock()
	defer l.mu.Unlock()
	delete(l.entries, key)
}
