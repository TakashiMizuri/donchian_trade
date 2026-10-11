package strategy

import "math"

// VolPctAt is rolling std(close[j-bars+1..j]) / close[j] (sample std, ddof=1).
// Matches research squeeze_vol_rank input before the causal shift.
func VolPctAt(bars []Bar, j, volBars int) (float64, bool) {
	if volBars < 2 || j < volBars-1 || j >= len(bars) {
		return 0, false
	}
	c := bars[j].Close
	if c == 0 || math.IsNaN(c) {
		return 0, false
	}
	start := j - volBars + 1
	var sum float64
	for k := start; k <= j; k++ {
		sum += bars[k].Close
	}
	n := float64(volBars)
	mean := sum / n
	var ss float64
	for k := start; k <= j; k++ {
		d := bars[k].Close - mean
		ss += d * d
	}
	std := math.Sqrt(ss / (n - 1))
	return std / c, true
}

// VolRankAt is the causal vol percentile at bar i (research R-004/R-005):
//
//	vol_pct[j] = std(close[j-volBars+1..j]) / close[j]
//	rank_input[j] = vol_pct[j-1]          // shift(1)
//	vol_rank[i] = percentile_rank(rank_input[i] among [i-lookback+1 .. i])
//
// Complexity is O(volBars + lookback) via a sliding sum/sumsq — not
// O(volBars*lookback). The naive form made 1m (1200/60000) hold busy for
// hundreds of ms×N books and missed minute boundaries.
func VolRankAt(bars []Bar, i, volBars, lookback int) (float64, bool) {
	if lookback < 2 || volBars < 2 || i < 0 || i >= len(bars) {
		return 0, false
	}
	// Need rank_input[i-lookback+1]: that equals vol_pct[i-lookback], which needs
	// index i-lookback >= volBars-1 ⇒ i >= lookback + volBars - 1.
	minI := lookback + volBars - 1
	if i < minI {
		return 0, false
	}
	// vol_pct indices p ∈ [i-lookback, i-1] (= rank_input for j ∈ [i-lookback+1, i]).
	p0 := i - lookback
	start := p0 - volBars + 1
	if start < 0 {
		return 0, false
	}
	n := float64(volBars)
	var sum, sumsq float64
	for k := start; k <= p0; k++ {
		c := bars[k].Close
		sum += c
		sumsq += c * c
	}
	window := make([]float64, 0, lookback)
	for p := p0; p <= i-1; p++ {
		c := bars[p].Close
		if c == 0 || math.IsNaN(c) {
			return 0, false
		}
		vari := (sumsq - sum*sum/n) / (n - 1)
		if vari < 0 {
			vari = 0
		}
		window = append(window, math.Sqrt(vari)/c)
		if p == i-1 {
			break
		}
		old := bars[start].Close
		sum -= old
		sumsq -= old * old
		start++
		next := bars[p+1].Close
		sum += next
		sumsq += next * next
	}
	v := window[len(window)-1]
	return percentileRankLE(window, v), true
}

// percentileRankLE = (# of window values ≤ v) / len(window), in (0, 1].
func percentileRankLE(window []float64, v float64) float64 {
	if len(window) == 0 {
		return math.NaN()
	}
	le := 0
	for _, x := range window {
		if x <= v {
			le++
		}
	}
	return float64(le) / float64(len(window))
}

// BreakoutATRDistance is how far close is past the N-channel edge, in ATR units.
// Positive = beyond the edge in the breakout direction; 0 if inside / flat.
func BreakoutATRDistance(close, upper, lower, atr float64, dir Direction) float64 {
	if atr <= 0 {
		return 0
	}
	switch dir {
	case DirBuy:
		return (close - upper) / atr
	case DirSell:
		return (lower - close) / atr
	default:
		return 0
	}
}
