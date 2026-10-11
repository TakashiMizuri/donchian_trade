package engine

import (
	"sort"
	"time"
)

const lagRingMax = 240 // ~4h of 1m bars (both symbols)

// LagSample is one closed-bar detect measurement for the dashboard.
type LagSample struct {
	Symbol      string `json:"symbol"`
	BarTime     int64  `json:"bar_time"`
	DetectLagMs int64  `json:"detect_lag_ms"`
	Source      string `json:"source"`
	DetectedAt  int64  `json:"detected_at"`
}

// LagStats aggregates recent candle-detect latency (wall clock − bar close).
type LagStats struct {
	N          int            `json:"n"`
	LastMs     int64          `json:"last_ms"`
	MedianMs   int64          `json:"median_ms"`
	P90Ms      int64          `json:"p90_ms"`
	MaxMs      int64          `json:"max_ms"`
	MeanMs     float64        `json:"mean_ms"`
	Gt5sPct    float64        `json:"gt_5s_pct"`
	Gt30sPct   float64        `json:"gt_30s_pct"`
	Gt60sPct   float64        `json:"gt_60s_pct"`
	BySource   map[string]int `json:"by_source"`
	Healthy    bool           `json:"healthy"`
	Alert      string         `json:"alert,omitempty"`
	Samples    []LagSample    `json:"samples"`
	PerSymbol  map[string]LagStatsSoft `json:"per_symbol,omitempty"`
}

// LagStatsSoft is a compact per-symbol summary (no nested samples).
type LagStatsSoft struct {
	N        int     `json:"n"`
	LastMs   int64   `json:"last_ms"`
	MedianMs int64   `json:"median_ms"`
	P90Ms    int64   `json:"p90_ms"`
	MaxMs    int64   `json:"max_ms"`
	MeanMs   float64 `json:"mean_ms"`
	Healthy  bool    `json:"healthy"`
}

func (e *Engine) recordDetectLag(symbol string, barTime, detectLagMs int64, source string) {
	// boot_catchup replays stale bars on purpose — do not poison the dashboard.
	if source == "boot_catchup" {
		return
	}
	if e.lagRing == nil {
		e.lagRing = make([]LagSample, 0, lagRingMax)
	}
	s := LagSample{
		Symbol:      symbol,
		BarTime:     barTime,
		DetectLagMs: detectLagMs,
		Source:      source,
		DetectedAt:  time.Now().UTC().Unix(),
	}
	e.lagRing = append(e.lagRing, s)
	if len(e.lagRing) > lagRingMax {
		e.lagRing = append([]LagSample(nil), e.lagRing[len(e.lagRing)-lagRingMax:]...)
	}
	e.lagLast = s
}

func (e *Engine) candleLagStats() LagStats {
	samples := e.lagRing
	if len(samples) == 0 {
		return LagStats{
			BySource:  map[string]int{},
			Healthy:   true,
			Samples:   []LagSample{},
			PerSymbol: map[string]LagStatsSoft{},
		}
	}
	st := summarizeLag(samples)
	st.Samples = lastNSamples(samples, 60)
	st.PerSymbol = map[string]LagStatsSoft{}
	bySym := map[string][]LagSample{}
	for _, s := range samples {
		bySym[s.Symbol] = append(bySym[s.Symbol], s)
	}
	for sym, xs := range bySym {
		soft := summarizeLag(xs)
		st.PerSymbol[sym] = LagStatsSoft{
			N: soft.N, LastMs: soft.LastMs, MedianMs: soft.MedianMs,
			P90Ms: soft.P90Ms, MaxMs: soft.MaxMs, MeanMs: soft.MeanMs,
			Healthy: soft.Healthy,
		}
	}
	return st
}

func lastNSamples(samples []LagSample, n int) []LagSample {
	if n <= 0 || len(samples) == 0 {
		return []LagSample{}
	}
	if len(samples) <= n {
		out := make([]LagSample, len(samples))
		copy(out, samples)
		// newest first for the table
		for i, j := 0, len(out)-1; i < j; i, j = i+1, j-1 {
			out[i], out[j] = out[j], out[i]
		}
		return out
	}
	out := make([]LagSample, n)
	copy(out, samples[len(samples)-n:])
	for i, j := 0, len(out)-1; i < j; i, j = i+1, j-1 {
		out[i], out[j] = out[j], out[i]
	}
	return out
}

func summarizeLag(samples []LagSample) LagStats {
	n := len(samples)
	xs := make([]int64, n)
	var sum float64
	var gt5, gt30, gt60 int
	by := map[string]int{}
	var max int64
	for i, s := range samples {
		xs[i] = s.DetectLagMs
		sum += float64(s.DetectLagMs)
		if s.DetectLagMs > max {
			max = s.DetectLagMs
		}
		if s.DetectLagMs >= 5000 {
			gt5++
		}
		if s.DetectLagMs >= 30000 {
			gt30++
		}
		if s.DetectLagMs >= 60000 {
			gt60++
		}
		src := s.Source
		if src == "" {
			src = "?"
		}
		by[src]++
	}
	sort.Slice(xs, func(i, j int) bool { return xs[i] < xs[j] })
	med := xs[n/2]
	p90 := xs[(n*9)/10]
	if (n*9)/10 >= n {
		p90 = xs[n-1]
	}
	last := samples[n-1].DetectLagMs
	mean := sum / float64(n)
	// Healthy: median under 5s and last under 15s. 1m bug was ~180–230s floor.
	healthy := med < 5000 && last < 15000
	alert := ""
	if !healthy {
		switch {
		case med >= 60000 || last >= 60000:
			alert = "detect_lag критический (≥60s) — свечи доходят слишком поздно"
		case med >= 15000 || last >= 15000:
			alert = "detect_lag высокий — проверь boundary/REST"
		default:
			alert = "detect_lag выше нормы"
		}
	}
	return LagStats{
		N:        n,
		LastMs:   last,
		MedianMs: med,
		P90Ms:    p90,
		MaxMs:    max,
		MeanMs:   mean,
		Gt5sPct:  100 * float64(gt5) / float64(n),
		Gt30sPct: 100 * float64(gt30) / float64(n),
		Gt60sPct: 100 * float64(gt60) / float64(n),
		BySource: by,
		Healthy:  healthy,
		Alert:    alert,
	}
}
