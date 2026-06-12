package ratelimit

import (
	"testing"
	"time"
)

func TestLimiterBurst(t *testing.T) {
	l := New(10, 5)
	for i := 0; i < 5; i++ {
		if !l.Allow("k") {
			t.Fatalf("Allow %d: expected true within burst", i+1)
		}
	}
	if l.Allow("k") {
		t.Fatal("6th Allow on same key: expected false")
	}
}

func TestLimiterSeparateKeys(t *testing.T) {
	l := New(10, 1)
	if !l.Allow("a") {
		t.Fatal("first key should be allowed")
	}
	if !l.Allow("b") {
		t.Fatal("separate key should be independent")
	}
}

func TestLimiterRefill(t *testing.T) {
	l := New(10, 1)
	if !l.Allow("k") {
		t.Fatal("initial allow expected")
	}
	if l.Allow("k") {
		t.Fatal("second immediate allow should be denied")
	}
	time.Sleep(150 * time.Millisecond)
	if !l.Allow("k") {
		t.Fatal("expected refill after sleep at 10/s rate")
	}
}
