package engine

import (
	"testing"
	"time"
)

func TestSuppressRepeatWarn(t *testing.T) {
	at := map[string]time.Time{}
	now := time.Unix(1_700_000_000, 0)
	if SuppressRepeat(at, now, time.Hour, "warn", "watchdog", "ETH 401") {
		t.Fatal("first warn must send")
	}
	if !SuppressRepeat(at, now.Add(15*time.Minute), time.Hour, "warn", "watchdog", "ETH 401") {
		t.Fatal("same warn within window must drop")
	}
	if SuppressRepeat(at, now.Add(time.Hour+time.Second), time.Hour, "warn", "watchdog", "ETH 401") {
		t.Fatal("same warn after window must send")
	}
}

func TestSuppressRepeatAllowsInfo(t *testing.T) {
	at := map[string]time.Time{}
	now := time.Unix(1_700_000_000, 0)
	if SuppressRepeat(at, now, time.Hour, "info", "entry", "ETH in") {
		t.Fatal("info must never suppress")
	}
	if SuppressRepeat(at, now, time.Hour, "info", "entry", "ETH in") {
		t.Fatal("info must never suppress")
	}
}

func TestSuppressRepeatNilMap(t *testing.T) {
	if SuppressRepeat(nil, time.Now(), time.Hour, "warn", "watchdog", "x") {
		t.Fatal("nil map must not suppress")
	}
}
