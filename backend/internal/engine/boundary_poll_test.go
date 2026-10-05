package engine

import (
	"testing"
	"time"

	"donchian.trade/bot/internal/config"
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

func TestBoundaryPollConstants(t *testing.T) {
	if boundaryPollInterval != 200*time.Millisecond {
		t.Fatalf("interval %v", boundaryPollInterval)
	}
	if boundaryPollTimeout < 3*time.Second {
		t.Fatalf("timeout too short %v", boundaryPollTimeout)
	}
	// ~25 attempts max in 5s at 200ms — guaranteed path vs old 4-shot + 15s ticker.
	maxAttempts := int(boundaryPollTimeout / boundaryPollInterval)
	if maxAttempts < 20 {
		t.Fatalf("attempts %d", maxAttempts)
	}
}
