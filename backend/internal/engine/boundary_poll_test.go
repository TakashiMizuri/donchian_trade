package engine

import (
	"testing"
	"time"

	"donchian.trade/bot/internal/config"
	"donchian.trade/bot/internal/strategy"
)

func TestHaveClosedBar(t *testing.T) {
	e := &Engine{
		Cfg:     &config.Config{Symbols: []string{"BTC", "ETH"}},
		lastBar: map[string]int64{"BTC": 1000, "ETH": 900},
	}
	if e.haveClosedBar(1000) {
		t.Fatal("ETH behind")
	}
	e.lastBar["ETH"] = 1000
	if !e.haveClosedBar(1000) {
		t.Fatal("both caught up")
	}
}

func TestNextBarBoundaryEveryBar(t *testing.T) {
	tf := 5 * time.Minute
	// Just after the 20:35 poll returns, the next close is 20:40 — not 20:45.
	afterPoll := time.Date(2026, 10, 12, 20, 35, 0, int(300*time.Millisecond), time.UTC)
	next := nextBarBoundary(afterPoll, tf)
	want := time.Date(2026, 10, 12, 20, 40, 0, 0, time.UTC)
	if !next.Equal(want) {
		t.Fatalf("next boundary %s, want %s", next, want)
	}
	// Walking close-to-close hits every 5m mark. The old loop waited until
	// `next` and then applied Truncate+period again, landing 10m ahead.
	cur := time.Date(2026, 10, 12, 20, 0, 0, int(200*time.Millisecond), time.UTC)
	for i := 0; i < 6; i++ {
		got := nextBarBoundary(cur, tf)
		cur = got.Add(200 * time.Millisecond)
		want := time.Date(2026, 10, 12, 20, 5*(i+1), 0, 0, time.UTC)
		if !got.Equal(want) {
			t.Fatalf("step %d got %s want %s", i, got, want)
		}
	}
}

func TestBoundaryPollConstants(t *testing.T) {
	if boundaryPollInterval != 200*time.Millisecond {
		t.Fatalf("interval %v", boundaryPollInterval)
	}
	t1m := boundaryPollTimeoutFor(time.Minute)
	if t1m < 30*time.Second {
		t.Fatalf("1m timeout too short %v", t1m)
	}
	t5m := boundaryPollTimeoutFor(5 * time.Minute)
	if t5m < 5*time.Second {
		t.Fatalf("5m timeout too short %v", t5m)
	}
	maxAttempts := int(t1m / boundaryPollInterval)
	if maxAttempts < 20 {
		t.Fatalf("1m attempts %d", maxAttempts)
	}
}

func TestEnqueueClosedDedup(t *testing.T) {
	e := &Engine{
		lastBar: map[string]int64{},
		bars:    map[string][]strategy.Bar{},
		pending: map[string][]pendingClosed{},
		busy:    map[string]bool{},
	}
	e.enqueueClosed("BTC", strategy.Bar{Time: 100, Close: 1}, 2, "boundary")
	e.enqueueClosed("BTC", strategy.Bar{Time: 100, Close: 1.5}, 2.5, "reconcile")
	e.enqueueClosed("BTC", strategy.Bar{Time: 160, Close: 3}, 4, "boundary")
	if len(e.pending["BTC"]) != 2 {
		t.Fatalf("pending %d", len(e.pending["BTC"]))
	}
	if e.pending["BTC"][0].bar.Close != 1.5 {
		t.Fatalf("dedup overwrite got %v", e.pending["BTC"][0].bar.Close)
	}
	if e.pending["BTC"][0].bar.Time != 100 || e.pending["BTC"][1].bar.Time != 160 {
		t.Fatalf("order %+v", e.pending["BTC"])
	}
}
