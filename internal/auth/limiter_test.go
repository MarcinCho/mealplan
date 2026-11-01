package auth

import (
	"testing"
	"time"
)

type fakeClock struct{ t time.Time }

func (c *fakeClock) Now() time.Time          { return c.t }
func (c *fakeClock) Advance(d time.Duration) { c.t = c.t.Add(d) }

func newClock() *fakeClock { return &fakeClock{t: time.Date(2026, 11, 1, 12, 0, 0, 0, time.UTC)} }

func TestLimiterLocksOnThresholdAndDoubles(t *testing.T) {
	c := newClock()
	l := NewLimiter(c.Now, 5)
	for i := 0; i < 4; i++ {
		l.Fail("k")
		if ok, _ := l.Allow("k"); !ok {
			t.Fatalf("locked too early after %d failures", i+1)
		}
	}
	l.Fail("k") // 5th failure
	ok, wait := l.Allow("k")
	if ok || wait != time.Second {
		t.Fatalf("expected 1s lock, ok=%v wait=%v", ok, wait)
	}
	c.Advance(time.Second)
	if ok, _ := l.Allow("k"); !ok {
		t.Fatal("lock should expire")
	}
	l.Fail("k") // 6th failure
	if _, wait := l.Allow("k"); wait != 2*time.Second {
		t.Fatalf("expected 2s lock, got %v", wait)
	}
}

func TestLimiterLockoutIsCapped(t *testing.T) {
	c := newClock()
	l := NewLimiter(c.Now, 5)
	for i := 0; i < 60; i++ {
		l.Fail("k")
		if _, wait := l.Allow("k"); wait > 15*time.Minute {
			t.Fatalf("lock exceeds cap: %v", wait)
		}
		c.Advance(time.Minute)
	}
}

func TestLimiterResetAndForget(t *testing.T) {
	c := newClock()
	l := NewLimiter(c.Now, 2)
	l.Fail("a")
	l.Fail("a")
	if ok, _ := l.Allow("a"); ok {
		t.Fatal("expected lock")
	}
	l.Reset("a")
	if ok, _ := l.Allow("a"); !ok {
		t.Fatal("reset must unlock")
	}
	l.Fail("b")
	l.Fail("b")
	c.Advance(61 * time.Minute)
	if ok, _ := l.Allow("b"); !ok {
		t.Fatal("stale entries must be forgotten")
	}
	l.Fail("b") // counter starts from zero again
	if ok, _ := l.Allow("b"); !ok {
		t.Fatal("single failure after forgetting must not lock")
	}
}

func TestLimiterKeysAreIndependent(t *testing.T) {
	c := newClock()
	l := NewLimiter(c.Now, 1)
	l.Fail("a")
	if ok, _ := l.Allow("b"); !ok {
		t.Fatal("other key must be unaffected")
	}
}
