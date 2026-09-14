// Package telemetry implements startpack §17: Live vs Shadow layers A/B/C and verdict.
package telemetry

import (
	"math"
	"sort"
)

const (
	BookLive     = "live"
	BookLS5      = "shadow_ls5"
	BookBaseline = "shadow_baseline"

	StatusGreen  = "green"
	StatusYellow = "yellow"
	StatusOrange = "orange"
	StatusRed    = "red"
	StatusNA     = "na"

	VerdictWAIT        = "WAIT"
	VerdictInvestigate = "INVESTIGATE"
	VerdictSTOP        = "STOP"
)

type TradeKey struct {
	Symbol     string `json:"symbol"`
	SignalTime int64  `json:"signal_time"`
	Direction  string `json:"direction"`
}

type ClosedTrade struct {
	Key         TradeKey
	Book        string
	Outcome     string
	EntryShadow float64
	EntryLive   float64
	ExitShadow  float64
	ExitLive    float64
	Stop        float64
	Gross       float64
	Net         float64
	Funding     float64
	RiskUSD     float64
	EntryFee    float64
	ExitFee     float64
	NotionalRT  float64
	Open        bool
}

func Key(symbol, direction string, signalTime int64) TradeKey {
	return TradeKey{Symbol: symbol, SignalTime: signalTime, Direction: direction}
}

func Match(shadow, live []ClosedTrade) (matched, liveOnly, shadowOnly []TradeKey, rate, dice float64) {
	S := map[TradeKey]struct{}{}
	L := map[TradeKey]struct{}{}
	for _, t := range shadow {
		S[t.Key] = struct{}{}
	}
	for _, t := range live {
		L[t.Key] = struct{}{}
	}
	union := map[TradeKey]struct{}{}
	for k := range S {
		union[k] = struct{}{}
		if _, ok := L[k]; ok {
			matched = append(matched, k)
		} else {
			shadowOnly = append(shadowOnly, k)
		}
	}
	for k := range L {
		union[k] = struct{}{}
		if _, ok := S[k]; !ok {
			liveOnly = append(liveOnly, k)
		}
	}
	if len(union) > 0 {
		rate = float64(len(matched)) / float64(len(union))
	}
	if len(S)+len(L) > 0 {
		dice = 2 * float64(len(matched)) / float64(len(S)+len(L))
	}
	return
}

// SlipBps: adverse > 0 (live worse than shadow). §17.7.1
func SlipBps(buy bool, entryLive, entryShadow, exitLive, exitShadow float64) (entry, exit, rt, side float64) {
	sign := 1.0
	if !buy {
		sign = -1.0
	}
	if entryShadow != 0 {
		entry = 10000 * sign * (entryLive - entryShadow) / entryShadow
	}
	if exitShadow != 0 {
		exit = 10000 * sign * (exitShadow - exitLive) / exitShadow
	}
	rt = entry + exit
	side = rt / 2
	return
}

func Median(xs []float64) float64 {
	if len(xs) == 0 {
		return 0
	}
	cp := append([]float64(nil), xs...)
	sort.Float64s(cp)
	n := len(cp)
	if n%2 == 1 {
		return cp[n/2]
	}
	return (cp[n/2-1] + cp[n/2]) / 2
}

func Percentile(xs []float64, p float64) float64 {
	if len(xs) == 0 {
		return 0
	}
	cp := append([]float64(nil), xs...)
	sort.Float64s(cp)
	if p <= 0 {
		return cp[0]
	}
	if p >= 1 {
		return cp[len(cp)-1]
	}
	idx := int(math.Ceil(p*float64(len(cp)))) - 1
	if idx < 0 {
		idx = 0
	}
	if idx >= len(cp) {
		idx = len(cp) - 1
	}
	return cp[idx]
}

func PnLRatio(liveNetXF, shadowNet, startEquity, typicalRisk float64) (ratio float64, ok bool) {
	eps := math.Max(0.005*startEquity, typicalRisk)
	if math.Abs(shadowNet) < eps {
		return 0, false
	}
	return liveNetXF / shadowNet, true
}

func StatusA(matchRate float64, nUnion, liveOnly7d, shadowOnly7d int) string {
	if nUnion < 10 {
		if liveOnly7d+shadowOnly7d >= 2 {
			return StatusYellow
		}
		return StatusNA
	}
	if matchRate < 0.95 {
		return StatusRed
	}
	if matchRate < 0.99 || liveOnly7d >= 2 || shadowOnly7d >= 2 {
		return StatusYellow
	}
	return StatusGreen
}

func StatusB(sideMedian, sideP90 float64, nMatched int) string {
	if nMatched < 20 {
		return StatusNA
	}
	if sideMedian >= 10 {
		return StatusRed
	}
	if sideMedian >= 5 {
		return StatusOrange
	}
	if sideMedian > 2 || sideP90 > 15 {
		return StatusYellow
	}
	return StatusGreen
}

func StatusC(ratio float64, ratioOK bool, nMatched int) string {
	if !ratioOK || nMatched < 20 {
		return StatusNA
	}
	if nMatched < 50 {
		if ratio < 0.40 {
			return StatusYellow
		}
		return StatusNA
	}
	if ratio >= 0.60 {
		return StatusGreen
	}
	if ratio >= 0.40 {
		return StatusYellow
	}
	if ratio >= 0.20 {
		return StatusOrange
	}
	return StatusRed
}

type VerdictIn struct {
	A, B, C          string
	NMatched         int
	DDLive, DDShadow float64
}

// Verdict implements §17.13. Never uses the sign of daily PnL.
func Verdict(in VerdictIn) string {
	if in.DDLive <= -0.35 && in.DDShadow > -0.15 {
		return VerdictSTOP
	}
	if in.A == StatusRed {
		return VerdictSTOP
	}
	if in.B == StatusRed {
		return VerdictSTOP
	}
	if in.A == StatusYellow {
		return VerdictInvestigate
	}
	if in.A == StatusGreen && in.B == StatusYellow {
		return VerdictInvestigate
	}
	if in.A == StatusGreen && in.B == StatusOrange {
		return VerdictInvestigate
	}
	if in.A == StatusGreen && (in.B == StatusGreen || in.B == StatusNA) && in.C == StatusRed && in.NMatched >= 150 {
		return VerdictInvestigate
	}
	if in.A == StatusGreen && (in.B == StatusGreen || in.B == StatusNA) {
		return VerdictWAIT
	}
	if in.A == StatusNA && in.B != StatusRed {
		return VerdictWAIT
	}
	return VerdictInvestigate
}

func Drawdown(equities []float64) float64 {
	peak, dd := 0.0, 0.0
	eq := 0.0
	for _, x := range equities {
		eq = x
		if eq > peak {
			peak = eq
		}
		if peak > 0 {
			d := (eq - peak) / peak
			if d < dd {
				dd = d
			}
		}
	}
	return dd
}

type Report struct {
	EquityLive           float64    `json:"equity_live"`
	EquityShadowLS5      float64    `json:"equity_shadow_ls5"`
	EquityShadowBaseline float64    `json:"equity_shadow_baseline"`
	EquityLiveExFunding  float64    `json:"equity_live_ex_funding"`
	GapUSD               float64    `json:"gap_usd"`
	GapPct               float64    `json:"gap_pct"`
	DDLive               float64    `json:"dd_live"`
	DDShadowLS5          float64    `json:"dd_shadow_ls5"`
	MatchRateLTD         float64    `json:"match_rate_ltd"`
	MatchRateDice        float64    `json:"match_rate_dice"`
	MatchRate7d          float64    `json:"match_rate_7d"`
	LiveOnly7d           int        `json:"live_only_7d"`
	ShadowOnly7d         int        `json:"shadow_only_7d"`
	NMatched             int        `json:"n_matched"`
	NUnion               int        `json:"n_union"`
	SideSlipMedianBps    float64    `json:"side_slip_median_bps"`
	SideSlipP90Bps       float64    `json:"side_slip_p90_bps"`
	SLExitSlipMedianBps  float64    `json:"sl_exit_slip_median_bps"`
	LiveNet              float64    `json:"live_net"`
	LiveNetXF            float64    `json:"live_net_xf"`
	ShadowNet            float64    `json:"shadow_net"`
	Funding              float64    `json:"funding"`
	PnLRatioXF           float64    `json:"pnl_ratio_xf"`
	PnLRatioOK           bool       `json:"pnl_ratio_ok"`
	StatusA              string     `json:"status_a"`
	StatusB              string     `json:"status_b"`
	StatusC              string     `json:"status_c"`
	Verdict              string     `json:"verdict"`
	Mismatches           []TradeKey `json:"mismatches"`
}
