package engine

import (
	"testing"

	"donchian.trade/bot/internal/exchange/lighter"
	"donchian.trade/bot/internal/strategy"
)

func TestStopIsRestingSides(t *testing.T) {
	if !stopIsResting(false, 2063, 2142) {
		t.Fatal("long sell-stop below mark must rest")
	}
	if stopIsResting(false, 2200, 2142) {
		t.Fatal("long stop above mark is through")
	}
	if !stopIsResting(true, 2182, 2142) {
		t.Fatal("short buy-stop above mark must rest")
	}
	if stopIsResting(true, 2063, 2142) {
		t.Fatal("short trail below mark is through")
	}
	if stopIsResting(true, 2142, 2142) {
		t.Fatal("at mark is not resting")
	}
}

func TestIntendedProtectiveStopTrailWhenResting(t *testing.T) {
	cfg := strategy.DefaultConfig()
	cfg.LateTrailOnBars = 3
	cfg.LateTrailR = 2
	cfg.LateTrailMinMFE = 6
	cfg.ChannelN = 3
	cfg.ExitM = 2
	cfg.ATRPeriod = 2
	bars := make([]strategy.Bar, 9)
	for i := range bars {
		bars[i] = strategy.Bar{Time: int64(i), Open: 100, High: 101, Low: 99, Close: 100}
	}
	bars[3] = strategy.Bar{Time: 3, Open: 100, High: 110, Low: 100, Close: 109}
	bars[4] = strategy.Bar{Time: 4, Open: 100, High: 101, Low: 99.5, Close: 100.5}
	st := strategy.NewState(10_000)
	atr := strategy.ATR(bars, cfg.ATRPeriod)
	enter := strategy.Decide(bars, atr, 3, bars[4].Open, bars[4].Time, st, cfg)
	strategy.Apply(&st, cfg, bars, enter)
	risk := st.Position.RiskDistance
	entry := st.Position.EntryPrice
	peak := entry + 10*risk
	for i := 5; i <= 7; i++ {
		px := entry + float64(i-4)*risk
		bars[i] = strategy.Bar{Time: int64(i), Open: px, High: px + 0.1, Low: px - 0.1, Close: px}
	}
	bars[7] = strategy.Bar{Time: 7, Open: peak - risk, High: peak, Low: peak - risk, Close: peak - 0.5}
	trail := peak - 2*risk
	mark := peak
	want, kind, gotTrail, ok := intendedProtectiveStop(st.Position, bars[:8], cfg, mark)
	if !ok || kind != "trail" {
		t.Fatalf("kind=%s ok=%v", kind, ok)
	}
	if !stopClose(want, trail) || !stopClose(gotTrail, trail) {
		t.Fatalf("want %v trail %v got %v", trail, gotTrail, want)
	}
}

func TestIntendedProtectiveStopKeepsOrigWhenTrailThrough(t *testing.T) {
	pos := &strategy.Position{
		Direction:    strategy.DirSell,
		EntryPrice:   2200,
		Stop:         2182.7,
		RiskDistance: 17.3,
		EntryIdx:     0,
	}
	cfg := strategy.DefaultConfig()
	cfg.LateTrailOnBars = 1
	cfg.LateTrailR = 2
	cfg.LateTrailMinMFE = 1
	// Short: peak is the low. One bar low enough to arm trail below current mark.
	bars := []strategy.Bar{
		{Time: 0, Open: 2200, High: 2201, Low: 2199, Close: 2200},
		{Time: 1, Open: 2100, High: 2101, Low: 2000, Close: 2050},
	}
	trail, ok := strategy.TrailStopForBar(bars, len(bars), pos, cfg)
	if !ok {
		t.Fatal("trail should be armed")
	}
	mark := 2142.0
	if stopIsResting(true, trail, mark) {
		t.Fatalf("fixture trail %v should be through mark %v", trail, mark)
	}
	want, kind, gotTrail, trailOK := intendedProtectiveStop(pos, bars, cfg, mark)
	if !trailOK || kind != "orig_trail_through" {
		t.Fatalf("kind=%s trailOK=%v trail=%v", kind, trailOK, gotTrail)
	}
	if want != pos.Stop {
		t.Fatalf("must keep orig %v got %v", pos.Stop, want)
	}
}

func TestPickKeepStopPrefersClosestTrigger(t *testing.T) {
	orders := []lighter.OpenOrder{
		{ClientOrderIndex: 1, Trigger: 2000},
		{ClientOrderIndex: 2, Trigger: 2182.7},
		{ClientOrderIndex: 3, Trigger: 0},
	}
	keep := pickKeepStop(orders, 2182.69)
	if keep.ClientOrderIndex != 2 {
		t.Fatalf("keep %+v", keep)
	}
}

func TestIsProtectiveStop(t *testing.T) {
	if !isProtectiveStop(lighter.OpenOrder{ReduceOnly: true}) {
		t.Fatal("reduce-only is protection")
	}
	if !isProtectiveStop(lighter.OpenOrder{Trigger: 100}) {
		t.Fatal("trigger is protection")
	}
	if !isProtectiveStop(lighter.OpenOrder{Type: "STOP_LOSS"}) {
		t.Fatal("type stop is protection")
	}
	if isProtectiveStop(lighter.OpenOrder{Type: "limit"}) {
		t.Fatal("plain limit is not")
	}
}
