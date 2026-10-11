package strategy

import (
	"math"
	"testing"
)

func TestVolPctSampleStd(t *testing.T) {
	// closes 1,2,3 → mean 2, sample var ((1+0+1)/2)=1, std=1, /close=1/3
	bars := []Bar{{Close: 1}, {Close: 2}, {Close: 3}}
	vp, ok := VolPctAt(bars, 2, 3)
	if !ok {
		t.Fatal("expected ok")
	}
	almostEqual(t, "volpct", vp, 1.0/3.0, 1e-12)
}

func TestVolRankCausalShift(t *testing.T) {
	// Build enough bars with rising then flat closes so rank is defined.
	n := 30
	bars := make([]Bar, n)
	for i := 0; i < n; i++ {
		bars[i] = Bar{Close: 100 + float64(i)*0.01, High: 101, Low: 99, Open: 100}
	}
	volBars, lookback := 5, 10
	vr, ok := VolRankAt(bars, 20, volBars, lookback)
	if !ok || math.IsNaN(vr) || vr <= 0 || vr > 1 {
		t.Fatalf("vol_rank=%v ok=%v", vr, ok)
	}
	if _, ok := VolRankAt(bars, 5, volBars, lookback); ok {
		t.Fatal("should be cold before warmup")
	}
}

func volRankAtNaive(bars []Bar, i, volBars, lookback int) (float64, bool) {
	if lookback < 2 || volBars < 2 || i < 0 || i >= len(bars) {
		return 0, false
	}
	minI := lookback + volBars - 1
	if i < minI {
		return 0, false
	}
	window := make([]float64, 0, lookback)
	for j := i - lookback + 1; j <= i; j++ {
		vp, ok := VolPctAt(bars, j-1, volBars)
		if !ok {
			return 0, false
		}
		window = append(window, vp)
	}
	return percentileRankLE(window, window[len(window)-1]), true
}

func TestVolRankMatchesNaive(t *testing.T) {
	n := 80
	bars := make([]Bar, n)
	for i := 0; i < n; i++ {
		bars[i] = Bar{Close: 100 + float64(i)*0.01 + float64(i%7)*0.05, High: 101, Low: 99, Open: 100}
	}
	volBars, lookback := 5, 10
	for i := 0; i < n; i++ {
		got, ok1 := VolRankAt(bars, i, volBars, lookback)
		want, ok2 := volRankAtNaive(bars, i, volBars, lookback)
		if ok1 != ok2 {
			t.Fatalf("i=%d ok %v vs %v", i, ok1, ok2)
		}
		if !ok1 {
			continue
		}
		if math.Abs(got-want) > 1e-9 {
			t.Fatalf("i=%d got %v want %v", i, got, want)
		}
	}
}

func TestMinBreakoutFiltersWeakBreak(t *testing.T) {
	cfg := DefaultConfig()
	cfg.ChannelN = 5
	cfg.ExitM = 3
	cfg.ATRPeriod = 3
	cfg.MinBreakoutATR = 0.5
	bars := trendBars(20, 100, 0.5)
	// Force a tiny break: flatten then nudge close just above channel.
	atr := ATR(bars, cfg.ATRPeriod)
	i := 15
	upper, _ := ChannelAt(bars, i, cfg.ChannelN)
	atrI := atr[i]
	bars[i].Close = upper + 0.1*atrI // weak
	bars[i].High = bars[i].Close
	probe := ProbeEntry(bars, atr, i, cfg)
	if probe.SkipReason != "brk" {
		t.Fatalf("want brk skip, got %+v", probe)
	}
	bars[i].Close = upper + 0.6*atrI
	bars[i].High = bars[i].Close
	probe = ProbeEntry(bars, atr, i, cfg)
	if probe.SkipReason != "ok" || probe.Direction != DirBuy {
		t.Fatalf("want enter, got %+v", probe)
	}
}

func TestVolRankFilterSkipsHighRank(t *testing.T) {
	cfg := DefaultConfig()
	cfg.ChannelN = 3
	cfg.ExitM = 2
	cfg.ATRPeriod = 3
	cfg.MaxVolRank = 0.01 // almost everything fails
	cfg.VolRankBars = 5
	cfg.VolRankLookback = 10
	bars := trendBars(40, 100, 1)
	atr := ATR(bars, cfg.ATRPeriod)
	i := 30
	upper, _ := ChannelAt(bars, i, cfg.ChannelN)
	bars[i].Close = upper + 10
	bars[i].High = bars[i].Close
	probe := ProbeEntry(bars, atr, i, cfg)
	if probe.SkipReason != "vol_rank" {
		t.Fatalf("expected vol_rank skip, got %+v", probe)
	}
}

func TestWarmupIncludesVolRank(t *testing.T) {
	cfg := DefaultConfig()
	cfg.MaxVolRank = 0.64
	cfg.VolRankBars = 80
	cfg.VolRankLookback = 4000
	if Warmup(cfg) < 4080 {
		t.Fatalf("warmup %d", Warmup(cfg))
	}
}
