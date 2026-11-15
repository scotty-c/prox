package util

import (
	"sync"
	"sync/atomic"
	"testing"
	"time"
)

func TestForEachVisitsEveryIndexOnce(t *testing.T) {
	for _, n := range []int{0, 1, 5, 100} {
		seen := make([]int32, n)
		ForEach(n, 4, func(i int) { atomic.AddInt32(&seen[i], 1) })
		for i, count := range seen {
			if count != 1 {
				t.Fatalf("n=%d: index %d visited %d times", n, i, count)
			}
		}
	}
}

func TestForEachNeverExceedsTheWorkerLimit(t *testing.T) {
	var running, peak int32
	var mu sync.Mutex
	ForEach(50, 3, func(int) {
		now := atomic.AddInt32(&running, 1)
		mu.Lock()
		if now > peak {
			peak = now
		}
		mu.Unlock()
		time.Sleep(time.Millisecond)
		atomic.AddInt32(&running, -1)
	})
	if peak > 3 || peak < 2 {
		t.Fatalf("peak concurrency = %d, want 2 or 3 with a limit of 3", peak)
	}
}

func TestForEachResultsAreVisibleAfterItReturns(t *testing.T) {
	out := make([]int, 20)
	ForEach(len(out), 5, func(i int) { out[i] = i * i })
	for i, v := range out {
		if v != i*i {
			t.Fatalf("out[%d] = %d", i, v)
		}
	}
}

func TestForEachToleratesBadWorkerCounts(t *testing.T) {
	var n int32
	ForEach(3, 0, func(int) { atomic.AddInt32(&n, 1) })
	ForEach(3, -2, func(int) { atomic.AddInt32(&n, 1) })
	if n != 6 {
		t.Fatalf("ran %d times, want 6", n)
	}
}

func TestHasTag(t *testing.T) {
	tests := []struct {
		tags, tag string
		want      bool
	}{
		{"prod;web", "web", true},
		{"web", "web", true},
		{"prod;web;db", "web", true},
		{"prod; web", "web", true},
		{"webserver", "web", false},
		{"prod;web", "we", false},
		{" prod ; web ", "web", true},
		{"", "web", false},
		{"web", "", false},
		{"", "", false},
	}
	for _, tt := range tests {
		if got := HasTag(tt.tags, tt.tag); got != tt.want {
			t.Errorf("HasTag(%q, %q) = %v, want %v", tt.tags, tt.tag, got, tt.want)
		}
	}
}
