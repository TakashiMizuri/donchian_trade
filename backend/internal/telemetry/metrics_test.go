package telemetry

import "testing"

func TestSlipBps(t *testing.T) {
	// BUY: live entry 100.10 vs shadow 100 → +10 bps adverse; exit live 110 vs shadow 110.11 → ~10 bps
	e, x, rt, side := SlipBps(true, 100.10, 100, 110, 110.11)
	if e < 9.9 || e > 10.1 {
		t.Fatalf("entry slip %v", e)
	}
	if rt < 15 || side < 7 {
		t.Fatalf("rt=%v side=%v exit=%v", rt, side, x)
	}
	// SELL: live entry lower is better
	e2, _, _, _ := SlipBps(false, 99.90, 100, 90, 90)
	if e2 < 9.9 || e2 > 10.1 {
		t.Fatalf("sell entry slip should be adverse if sold cheaper: %v", e2)
	}
}

func TestMatchRate(t *testing.T) {
	sh := []ClosedTrade{
		{Key: Key("BTC", "BUY", 1)},
		{Key: Key("BTC", "SELL", 2)},
	}
	lv := []ClosedTrade{
		{Key: Key("BTC", "BUY", 1)},
		{Key: Key("ETH", "BUY", 3)},
	}
	m, lo, so, rate, _ := Match(sh, lv)
	if len(m) != 1 || len(lo) != 1 || len(so) != 1 {
		t.Fatalf("m=%d lo=%d so=%d", len(m), len(lo), len(so))
	}
	if rate < 0.32 || rate > 0.34 {
		t.Fatalf("rate %v want 1/3", rate)
	}
}

func TestPnLRatioNA(t *testing.T) {
	_, ok := PnLRatio(10, 0.1, 10000, 100)
	if ok {
		t.Fatal("tiny shadow should be NA")
	}
	r, ok := PnLRatio(80, 100, 10000, 100)
	if !ok || r < 0.79 || r > 0.81 {
		t.Fatalf("ratio %v ok=%v", r, ok)
	}
}

func TestVerdictWaitOnGreenAB(t *testing.T) {
	v := Verdict(VerdictIn{A: StatusGreen, B: StatusGreen, C: StatusNA, NMatched: 20, DDLive: -0.06, DDShadow: -0.05})
	if v != VerdictWAIT {
		t.Fatalf("got %s", v)
	}
}

func TestVerdictStopOnRedA(t *testing.T) {
	v := Verdict(VerdictIn{A: StatusRed, B: StatusGreen, C: StatusGreen, NMatched: 80, DDLive: 0.1, DDShadow: 0.05})
	if v != VerdictSTOP {
		t.Fatalf("got %s", v)
	}
}

func TestVerdictStopOnLiveDDDivergence(t *testing.T) {
	v := Verdict(VerdictIn{A: StatusGreen, B: StatusGreen, C: StatusGreen, NMatched: 80, DDLive: -0.40, DDShadow: -0.10})
	if v != VerdictSTOP {
		t.Fatalf("got %s", v)
	}
}

func TestBuildWaitOnMatchedGreen(t *testing.T) {
	r := Build(Input{
		StartEquity: 10000, TypicalRisk: 100,
		EquityLive: 9400, EquityLiveExFunding: 9430, EquityShadowLS5: 9500, EquityShadowBaseline: 9600,
		LiveEqCurve: []float64{10000, 9800, 9400}, ShadowEqCurve: []float64{10000, 9850, 9500},
		Live: []ClosedTrade{
			{Key: Key("BTC", "BUY", 1), EntryLive: 100.02, ExitLive: 98.5, Net: -160, Outcome: "sl"},
			{Key: Key("ETH", "SELL", 2), EntryLive: 2000, ExitLive: 1980, Net: -80, Outcome: "sl"},
		},
		Shadow: []ClosedTrade{
			{Key: Key("BTC", "BUY", 1), EntryShadow: 100, ExitShadow: 98.5, Net: -150, Outcome: "sl"},
			{Key: Key("ETH", "SELL", 2), EntryShadow: 2000, ExitShadow: 1980, Net: -70, Outcome: "sl"},
		},
		Now: 1_000_000,
	})
	if r.Verdict != VerdictWAIT {
		t.Fatalf("verdict %s A=%s B=%s C=%s n=%d", r.Verdict, r.StatusA, r.StatusB, r.StatusC, r.NMatched)
	}
	if r.NMatched != 2 || r.NUnion != 2 {
		t.Fatalf("match %d union %d", r.NMatched, r.NUnion)
	}
}

func TestMedian(t *testing.T) {
	if Median([]float64{1, 3, 2}) != 2 {
		t.Fatal("odd")
	}
	if Median([]float64{1, 2, 3, 4}) != 2.5 {
		t.Fatal("even")
	}
}
