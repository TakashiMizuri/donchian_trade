package strategy

import "math"

// TrueRange matches research strategies/mss/atr.py.
func TrueRange(bars []Bar) []float64 {
	n := len(bars)
	tr := make([]float64, n)
	if n == 0 {
		return tr
	}
	tr[0] = bars[0].High - bars[0].Low
	for i := 1; i < n; i++ {
		hl := bars[i].High - bars[i].Low
		hc := math.Abs(bars[i].High - bars[i-1].Close)
		lc := math.Abs(bars[i].Low - bars[i-1].Close)
		tr[i] = math.Max(hl, math.Max(hc, lc))
	}
	return tr
}

// ATR is SMA of True Range, NOT Wilder RMA.
// ATR[i] = mean(TR[start:i+1]) where start = max(1, i-period+1).
// Window never includes TR[0]. ATR[0] = high[0]-low[0].
func ATR(bars []Bar, period int) []float64 {
	n := len(bars)
	out := make([]float64, n)
	if n == 0 {
		return out
	}
	if period < 1 {
		period = 1
	}
	tr := TrueRange(bars)
	csum := make([]float64, n)
	csum[0] = tr[0]
	for i := 1; i < n; i++ {
		csum[i] = csum[i-1] + tr[i]
	}
	out[0] = math.Max(bars[0].High-bars[0].Low, 0)
	for i := 1; i < n; i++ {
		start := i - period + 1
		if start < 1 {
			start = 1
		}
		count := i - start + 1
		prev := csum[start-1]
		total := csum[i] - prev
		if count > 0 {
			out[i] = total / float64(count)
		} else {
			out[i] = math.Max(bars[i].High-bars[i].Low, 0)
		}
	}
	return out
}
