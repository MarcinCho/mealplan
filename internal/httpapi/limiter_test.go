package httpapi

import (
	"testing"
	"time"
)

func TestLimiterWindowSlides(t *testing.T) {
	now := time.Date(2026, 11, 3, 12, 0, 0, 0, time.UTC)
	l := newLimiter(2, time.Minute)
	l.now = func() time.Time { return now }
	for i := 0; i < 2; i++ {
		if ok, _ := l.allow("u"); !ok {
			t.Fatalf("call %d rejected", i)
		}
	}
	ok, retry := l.allow("u")
	if ok || retry != time.Minute {
		t.Fatalf("ok=%v retry=%v", ok, retry)
	}
	if ok, _ := l.allow("other"); !ok {
		t.Error("another user must have its own budget")
	}
	now = now.Add(61 * time.Second)
	if ok, _ := l.allow("u"); !ok {
		t.Error("budget must return after the window")
	}
}
