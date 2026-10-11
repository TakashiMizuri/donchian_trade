package engine

import (
	"testing"

	"donchian.trade/bot/internal/strategy"
)

func TestShouldSyncShadowOnLiveSL(t *testing.T) {
	sell := &strategy.Position{Direction: strategy.DirSell, Stop: 100}
	buy := &strategy.Position{Direction: strategy.DirBuy, Stop: 90}

	if !shouldSyncShadowOnLiveSL(strategy.DirSell, sell) {
		t.Fatal("same-side short should sync")
	}
	if !shouldSyncShadowOnLiveSL(strategy.DirBuy, buy) {
		t.Fatal("same-side long should sync")
	}
	if shouldSyncShadowOnLiveSL(strategy.DirSell, buy) {
		t.Fatal("opposite side must not sync")
	}
	if shouldSyncShadowOnLiveSL(strategy.DirSell, nil) {
		t.Fatal("flat shadow must not sync")
	}
}
