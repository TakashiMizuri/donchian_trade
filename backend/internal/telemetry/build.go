package telemetry

import (
	"fmt"
	"time"
)

type Input struct {
	StartEquity          float64
	TypicalRisk          float64
	EquityLive           float64
	EquityLiveExFunding  float64
	EquityShadowLS5      float64
	EquityShadowBaseline float64
	LiveEqCurve          []float64
	ShadowEqCurve        []float64
	Live                 []ClosedTrade
	Shadow               []ClosedTrade
	Now                  int64
}

func Build(in Input) Report {
	now := in.Now
	if now == 0 {
		now = time.Now().Unix()
	}
	week := now - 7*24*3600

	matched, liveOnly, shadowOnly, rate, dice := Match(in.Shadow, in.Live)
	_, _, _, rate7, _ := Match(inWindow(in.Shadow, week), inWindow(in.Live, week))
	lo7 := countSince(liveOnly, in.Live, week)
	so7 := countSince(shadowOnly, in.Shadow, week)

	liveBy := indexByKey(in.Live)
	shBy := indexByKey(in.Shadow)
	var sides, slExits []float64
	for _, k := range matched {
		lv, okL := liveBy[k]
		sh, okS := shBy[k]
		if !okL || !okS || lv.Open || sh.Open {
			continue
		}
		buy := k.Direction == "BUY"
		_, exit, _, side := SlipBps(buy, lv.EntryLive, sh.EntryShadow, lv.ExitLive, sh.ExitShadow)
		sides = append(sides, side)
		if lv.Outcome == "sl" || sh.Outcome == "sl" {
			slExits = append(slExits, exit)
		}
	}

	liveNet, liveXF, funding := 0.0, 0.0, 0.0
	for _, t := range in.Live {
		if t.Open {
			continue
		}
		liveNet += t.Net
		funding += t.Funding
		liveXF += t.Net - t.Funding
	}
	shadowNet := 0.0
	for _, t := range in.Shadow {
		if t.Open {
			continue
		}
		shadowNet += t.Net
	}

	ratio, ratioOK := PnLRatio(liveXF, shadowNet, in.StartEquity, in.TypicalRisk)
	nUnion := len(matched) + lo7 + so7
	// LTD union is |S∪L|, not the 7d extras
	nUnion = len(matched) + len(liveOnly) + len(shadowOnly)
	stA := StatusA(rate, nUnion, lo7, so7)
	stB := StatusB(Median(sides), Percentile(sides, 0.90), len(sides))
	stC := StatusC(ratio, ratioOK, len(sides))
	ddL := Drawdown(in.LiveEqCurve)
	ddS := Drawdown(in.ShadowEqCurve)
	verdict := Verdict(VerdictIn{A: stA, B: stB, C: stC, NMatched: len(sides), DDLive: ddL, DDShadow: ddS})

	gap := in.EquityLiveExFunding - in.EquityShadowLS5
	gapPct := 0.0
	if in.EquityShadowLS5 != 0 {
		gapPct = gap / in.EquityShadowLS5
	}

	mism := append(append([]TradeKey{}, liveOnly...), shadowOnly...)
	return Report{
		EquityLive:           in.EquityLive,
		EquityShadowLS5:      in.EquityShadowLS5,
		EquityShadowBaseline: in.EquityShadowBaseline,
		EquityLiveExFunding:  in.EquityLiveExFunding,
		GapUSD:               gap,
		GapPct:               gapPct,
		DDLive:               ddL,
		DDShadowLS5:          ddS,
		MatchRateLTD:         rate,
		MatchRateDice:        dice,
		MatchRate7d:          rate7,
		LiveOnly7d:           lo7,
		ShadowOnly7d:         so7,
		NMatched:             len(matched),
		NUnion:               nUnion,
		SideSlipMedianBps:    Median(sides),
		SideSlipP90Bps:       Percentile(sides, 0.90),
		SLExitSlipMedianBps:  Median(slExits),
		LiveNet:              liveNet,
		LiveNetXF:            liveXF,
		ShadowNet:            shadowNet,
		Funding:              funding,
		PnLRatioXF:           ratio,
		PnLRatioOK:           ratioOK,
		StatusA:              stA,
		StatusB:              stB,
		StatusC:              stC,
		Verdict:              verdict,
		Mismatches:           mism,
	}
}

func inWindow(ts []ClosedTrade, since int64) []ClosedTrade {
	cp := make([]ClosedTrade, 0, len(ts))
	for _, t := range ts {
		if t.Key.SignalTime >= since {
			cp = append(cp, t)
		}
	}
	return cp
}

func indexByKey(ts []ClosedTrade) map[TradeKey]ClosedTrade {
	m := make(map[TradeKey]ClosedTrade, len(ts))
	for _, t := range ts {
		m[t.Key] = t
	}
	return m
}

func countSince(keys []TradeKey, src []ClosedTrade, since int64) int {
	by := indexByKey(src)
	n := 0
	for _, k := range keys {
		if t, ok := by[k]; ok && t.Key.SignalTime >= since {
			n++
		} else if !ok && k.SignalTime >= since {
			n++
		}
	}
	return n
}

func FormatDaily(date string, r Report, extra string) string {
	ratio := "NA"
	if r.PnLRatioOK {
		ratio = fmt.Sprintf("%.3f", r.PnLRatioXF)
	}
	return fmt.Sprintf(
		"date_utc %s\nverdict %s  A=%s B=%s C=%s\n"+
			"equity_live %.2f  shadow_ls5 %.2f  shadow_base %.2f\n"+
			"gap %+.2f (%.2f%%)  dd_live %.2f%%  dd_shadow %.2f%%\n"+
			"match_ltd %.3f  match_7d %.3f  live_only_7d %d  shadow_only_7d %d\n"+
			"side_slip_med %.2f bps  p90 %.2f  sl_exit_med %.2f\n"+
			"live_net %.2f  live_xf %.2f  shadow_net %.2f  funding %.2f\n"+
			"pnl_ratio_xf %s  n_matched %d  n_union %d\n%s",
		date, r.Verdict, r.StatusA, r.StatusB, r.StatusC,
		r.EquityLive, r.EquityShadowLS5, r.EquityShadowBaseline,
		r.GapUSD, r.GapPct*100, r.DDLive*100, r.DDShadowLS5*100,
		r.MatchRateLTD, r.MatchRate7d, r.LiveOnly7d, r.ShadowOnly7d,
		r.SideSlipMedianBps, r.SideSlipP90Bps, r.SLExitSlipMedianBps,
		r.LiveNet, r.LiveNetXF, r.ShadowNet, r.Funding,
		ratio, r.NMatched, r.NUnion, extra,
	)
}
