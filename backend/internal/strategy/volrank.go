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
	window := make([]float64, 0, lookback)
	for j := i - lookback + 1; j <= i; j++ {
		vp, ok := VolPctAt(bars, j-1, volBars)
		if !ok {
			return 0, false
		}
		window = append(window, vp)
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
