package strategy

import (
	"math"
	"testing"
)

func almostEqual(t *testing.T, name string, got, want, eps float64) {
	t.Helper()
	if math.Abs(got-want) > eps {
		t.Fatalf("%s: got %v want %v", name, got, want)
	}
}

func TestATRMatchesResearchFormula(t *testing.T) {
	bars := []Bar{
		{Time: 0, Open: 10, High: 12, Low: 9, Close: 11},  // TR0=3, ATR0=3
		{Time: 1, Open: 11, High: 13, Low: 10, Close: 12}, // TR=3, ATR=3
		{Time: 2, Open: 12, High: 20, Low: 11, Close: 19}, // TR=9, ATR=6
		{Time: 3, Open: 19, High: 21, Low: 18, Close: 20}, // TR=max(3,2,1)=3, ATR=mean(3,9,3)=5
	}
	atr := ATR(bars, 20)
	almostEqual(t, "atr0", atr[0], 3, 1e-12)
	almostEqual(t, "atr1", atr[1], 3, 1e-12)
	almostEqual(t, "atr2", atr[2], 6, 1e-12)
	almostEqual(t, "atr3", atr[3], 5, 1e-12)
}

func TestATRWindowNeverIncludesTR0(t *testing.T) {
	bars := make([]Bar, 5)
	// Bar 0 is a huge range that must not leak into later ATR windows.
	bars[0] = Bar{High: 100, Low: 0, Close: 50, Open: 50}
	for i := 1; i < 5; i++ {
		bars[i] = Bar{Open: 50, High: 51, Low: 49, Close: 50}
	}
	atr := ATR(bars, 2)
	// For i=3: start=max(1,3-2+1)=2, mean(TR[2],TR[3]) — TR[0] excluded.
	// TR[i>=1] = max(2, |51-50|, |49-50|) = 2
	almostEqual(t, "atr3", atr[3], 2, 1e-12)
	if atr[1] > 10 {
		t.Fatalf("ATR[1] leaked TR[0]: %v", atr[1])
	}
}

func TestChannelCausalityExcludesBarI(t *testing.T) {
	bars := []Bar{
		{Time: 0, High: 10, Low: 1, Close: 5, Open: 5},
		{Time: 1, High: 11, Low: 2, Close: 6, Open: 6},
		{Time: 2, High: 99, Low: 0, Close: 50, Open: 7}, // bar i — must NOT enter channel
	}
	upper, lower := channelMaxMin(bars, 0, 2) // [0, 2) = bars 0 and 1
	almostEqual(t, "upper", upper, 11, 1e-12)
	almostEqual(t, "lower", lower, 1, 1e-12)
}

func TestSizing(t *testing.T) {
	risk := RiskUSD(10_000, 1.0, 1000)
	almostEqual(t, "risk", risk, 100, 1e-12)
	qty := Quantity(10_000, 1.0, 1000, 50)
	almostEqual(t, "qty", qty, 2, 1e-12)
	capped := RiskUSD(200_000, 1.0, 1000)
	almostEqual(t, "cap", capped, 1000, 1e-12)
}

func trendBars(n int, start float64, step float64) []Bar {
	bars := make([]Bar, n)
	px := start
	for i := 0; i < n; i++ {
		o := px
		c := px + step
		h, l := o, c
		if c > o {
			h, l = c+0.1, o-0.1
		} else {
			h, l = o+0.1, c-0.1
		}
		bars[i] = Bar{Time: int64(i * 3600), Open: o, High: h, Low: l, Close: c}
		px = c
	}
	return bars
}

func TestLongEntryStopOnNextOpen(t *testing.T) {
	cfg := DefaultConfig()
	cfg.LS5.Enabled = false
	cfg.ChannelN = 5
	cfg.ExitM = 3
	cfg.ATRPeriod = 3
	cfg.ATRStopMult = 1.5
	// Flat range then a breakout close.
	bars := make([]Bar, 12)
	for i := 0; i < 8; i++ {
		bars[i] = Bar{Time: int64(i * 3600), Open: 100, High: 101, Low: 99, Close: 100}
	}
	// signal bar: close above prior 5 highs (101)
	bars[8] = Bar{Time: 8 * 3600, Open: 100, High: 110, Low: 100, Close: 109}
	// entry bar
	bars[9] = Bar{Time: 9 * 3600, Open: 109.5, High: 112, Low: 108, Close: 111}
	// continue
	bars[10] = Bar{Time: 10 * 3600, Open: 111, High: 113, Low: 110, Close: 112}
	bars[11] = Bar{Time: 11 * 3600, Open: 112, High: 113, Low: 111, Close: 112}

	atr := ATR(bars, cfg.ATRPeriod)
	st := NewState(10_000)
	act := Decide(bars, atr, 8, bars[9].Open, bars[9].Time, st, cfg)
	if act.Kind != ActionEnter || act.Direction != DirBuy {
		t.Fatalf("expected BUY enter, got %+v", act)
	}
	almostEqual(t, "entry", act.Price, 109.5, 1e-9)
	wantStop := 109.5 - 1.5*atr[8]
	almostEqual(t, "stop", act.Stop, wantStop, 1e-9)
}

func TestStopPriorityOverChannel(t *testing.T) {
	cfg := DefaultConfig()
	cfg.LS5.Enabled = false
	cfg.ChannelN = 3
	cfg.ExitM = 2
	cfg.ATRPeriod = 2
	bars := []Bar{
		{Time: 0, Open: 100, High: 101, Low: 99, Close: 100},
		{Time: 3600, Open: 100, High: 101, Low: 99, Close: 100},
		{Time: 7200, Open: 100, High: 101, Low: 99, Close: 100},
		{Time: 10800, Open: 100, High: 120, Low: 100, Close: 119}, // signal
		{Time: 14400, Open: 119, High: 121, Low: 118, Close: 120}, // entry
		// Both SL (low punches stop) and channel exit (close below M-channel) same bar.
		{Time: 18000, Open: 120, High: 121, Low: 50, Close: 60},
		{Time: 21600, Open: 60, High: 61, Low: 59, Close: 60},
	}
	trades, _ := Replay(bars, cfg, 10_000, 0, math.MaxInt64)
	if len(trades) == 0 {
		t.Fatal("expected a trade")
	}
	last := trades[len(trades)-1]
	if last.Outcome != OutcomeSL {
		t.Fatalf("expected sl win over channel, got %s price=%v", last.Outcome, last.ExitPrice)
	}
	almostEqual(t, "sl fill", last.ExitPrice, last.StopLoss, 1e-9)
}

func TestNoSameBarReentryAfterExit(t *testing.T) {
	cfg := DefaultConfig()
	cfg.LS5.Enabled = false
	cfg.ChannelN = 3
	cfg.ExitM = 2
	cfg.ATRPeriod = 2
	bars := make([]Bar, 20)
	for i := range bars {
		px := 100.0
		bars[i] = Bar{Time: int64(i * 3600), Open: px, High: px + 1, Low: px - 1, Close: px}
	}
	// Force a long then an immediate SL on the next closed bar.
	bars[5] = Bar{Time: 5 * 3600, Open: 100, High: 130, Low: 100, Close: 129}
	bars[6] = Bar{Time: 6 * 3600, Open: 129, High: 130, Low: 128, Close: 129}
	bars[7] = Bar{Time: 7 * 3600, Open: 129, High: 130, Low: 1, Close: 2} // SL
	// Same bar 7 is also a massive breakdown that would be a short signal if we re-entered.
	trades, _ := Replay(bars, cfg, 10_000, 0, math.MaxInt64)
	entriesOnBar7 := 0
	for _, tr := range trades {
		if tr.EntryTime == 7*3600 {
			entriesOnBar7++
		}
	}
	if entriesOnBar7 != 0 {
		t.Fatalf("re-entered on the exit bar, trades=%+v", trades)
	}
}

func TestLS5PauseAndEarlyResume(t *testing.T) {
	cfg := DefaultConfig()
	cfg.ChannelN = 3
	cfg.ExitM = 2
	cfg.ATRPeriod = 2
	cfg.LS5 = LS5Config{Enabled: true, StreakN: 5, PauseBars: 24, ResumeATR: 2.0}

	st := NewState(10_000)
	// Five gross losses → pause.
	for i := 0; i < 5; i++ {
		onTradeClose(&st, cfg, DirBuy, 100, 90, 10+i)
	}
	if st.PauseUntilIdx < 0 {
		t.Fatal("expected pause after 5 losses")
	}
	if st.ConsecLosses != 0 {
		t.Fatalf("counter should reset after pause trigger, got %d", st.ConsecLosses)
	}

	// Weak breakout during pause — skip.
	upper, lower := 10.0, 9.0
	if resumeReady(st, cfg, st.PauseUntilIdx-1, upper, lower, 10.1, 1.0) {
		t.Fatal("weak breakout should not resume")
	}
	// Strong breakout ≥ 2 ATR above channel.
	if !resumeReady(st, cfg, st.PauseUntilIdx-1, upper, lower, 10+2.1, 1.0) {
		t.Fatal("2ATR breakout should resume")
	}
	if !resumeReady(st, cfg, st.PauseUntilIdx, upper, lower, 9.5, 1.0) {
		t.Fatal("pause expiry should resume")
	}
}

func TestReplayGeneratesLongAndShort(t *testing.T) {
	cfg := DefaultConfig()
	cfg.LS5.Enabled = false
	cfg.ChannelN = 5
	cfg.ExitM = 3
	cfg.ATRPeriod = 4
	up := trendBars(40, 100, 1.5)
	down := trendBars(40, up[len(up)-1].Close, -1.5)
	// Shift times of the down segment.
	off := up[len(up)-1].Time + 3600
	for i := range down {
		down[i].Time = off + int64(i*3600)
	}
	bars := append(up, down...)
	trades, _ := Replay(bars, cfg, 10_000, 0, math.MaxInt64)
	var buys, sells int
	for _, tr := range trades {
		if tr.Direction == DirBuy {
			buys++
		} else {
			sells++
		}
	}
	if buys == 0 || sells == 0 {
		t.Fatalf("expected both sides, buys=%d sells=%d trades=%d", buys, sells, len(trades))
	}
}
