package engine

import (
	"testing"

	"donchian.trade/bot/internal/strategy"
)

func TestShouldSyncShadowOnLiveSL(t *testing.T) {
	const sig int64 = 1_700_000_000
	sell := &strategy.Position{Direction: strategy.DirSell, Stop: 100, SignalTime: sig}
	buy := &strategy.Position{Direction: strategy.DirBuy, Stop: 90, SignalTime: sig}
	oldSell := &strategy.Position{Direction: strategy.DirSell, Stop: 100, SignalTime: sig - 3600}

	if !shouldSyncShadowOnLiveSL(strategy.DirSell, sig, sell) {
		t.Fatal("same trade short should sync")
	}
	if !shouldSyncShadowOnLiveSL(strategy.DirBuy, sig, buy) {
		t.Fatal("same trade long should sync")
	}
	if shouldSyncShadowOnLiveSL(strategy.DirSell, sig, buy) {
		t.Fatal("opposite side must not sync")
	}
	if shouldSyncShadowOnLiveSL(strategy.DirSell, sig, nil) {
		t.Fatal("flat shadow must not sync")
	}
	if shouldSyncShadowOnLiveSL(strategy.DirSell, sig, oldSell) {
		t.Fatal("same side but older signal must not sync")
	}
	if shouldSyncShadowOnLiveSL(strategy.DirSell, 0, sell) {
		t.Fatal("zero live signal must not match a real shadow signal")
	}
}
