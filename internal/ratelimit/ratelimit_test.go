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

func TestPruneRemovesStaleBucket(t *testing.T) {
	l := New(10, 5)
	l.Allow("stale")
	time.Sleep(50 * time.Millisecond)
	pruned := l.Prune(20 * time.Millisecond)
	if pruned != 1 {
		t.Fatalf("expected 1 pruned bucket, got %d", pruned)
	}
	if got := l.BucketCount(); got != 0 {
		t.Fatalf("expected 0 buckets after prune, got %d", got)
	}
}

func TestPruneKeepsActiveBucket(t *testing.T) {
	l := New(10, 5)
	l.Allow("active")
	pruned := l.Prune(1 * time.Hour)
	if pruned != 0 {
		t.Fatalf("expected 0 pruned buckets, got %d", pruned)
	}
	if got := l.BucketCount(); got != 1 {
		t.Fatalf("expected 1 bucket retained, got %d", got)
	}
}

func TestPruneEmptyLimiterIsSafe(t *testing.T) {
	l := New(10, 5)
	pruned := l.Prune(20 * time.Millisecond)
	if pruned != 0 {
		t.Fatalf("expected 0 pruned buckets on empty limiter, got %d", pruned)
	}
	if got := l.BucketCount(); got != 0 {
		t.Fatalf("expected 0 buckets, got %d", got)
	}
}

func TestPruneMixOfStaleAndActive(t *testing.T) {
	l := New(10, 5)
	l.Allow("stale")
	time.Sleep(50 * time.Millisecond)
	l.Allow("active")
	pruned := l.Prune(20 * time.Millisecond)
	if pruned != 1 {
		t.Fatalf("expected 1 pruned bucket, got %d", pruned)
	}
	if got := l.BucketCount(); got != 1 {
		t.Fatalf("expected 1 bucket retained, got %d", got)
	}
}
