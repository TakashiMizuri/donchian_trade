package api

import (
	"math"
	"net/http"
	"sort"
	"strconv"
	"time"

	"donchian.trade/bot/internal/store"
	"donchian.trade/bot/internal/telemetry"
)

func tradeView(t store.TradeRow) map[string]any {
	buy := t.Direction == "BUY"
	entrySh := t.EntryPxShadow
	if entrySh == 0 {
		entrySh = t.EntryPrice.Float64
	}
	exitSh := t.ExitPxShadow
	if exitSh == 0 {
		exitSh = t.ExitPrice.Float64
	}
	entryLive := t.EntryPxLive
	if entryLive == 0 {
		entryLive = t.EntryPrice.Float64
	}
	exitLive := t.ExitPxLive
	if exitLive == 0 {
		exitLive = t.ExitPrice.Float64
	}
	entrySlip, exitSlip, _, sideSlip := telemetry.SlipBps(buy, entryLive, entrySh, exitLive, exitSh)
	riskUSD := t.RiskDistance.Float64 * t.Quantity.Float64
	rMult := 0.0
	if riskUSD > 0 && t.Outcome.Valid && t.Outcome.String != "" && t.Outcome.String != "open" {
		rMult = t.Net.Float64 / riskUSD
	}
	return map[string]any{
		"id": t.ID, "symbol": t.Symbol, "profile": t.Profile, "direction": t.Direction,
		"signal_time": t.SignalTime, "entry_time": t.EntryTime.Int64, "entry_price": t.EntryPrice.Float64,
		"stop": t.Stop.Float64, "exit_time": t.ExitTime.Int64, "exit_price": t.ExitPrice.Float64,
		"outcome": t.Outcome.String, "risk_distance": t.RiskDistance.Float64, "quantity": t.Quantity.Float64,
		"gross": t.Gross.Float64, "entry_fee": t.EntryFee.Float64, "exit_fee": t.ExitFee.Float64,
		"net": t.Net.Float64, "funding": t.Funding.Float64,
		"entry_px_shadow": entrySh, "entry_px_live": entryLive,
		"exit_px_shadow": exitSh, "exit_px_live": exitLive,
		"entry_slip_bps": entrySlip, "exit_slip_bps": exitSlip, "side_slip_bps": sideSlip,
		"r_multiple": rMult, "risk_usd": riskUSD,
	}
}

func (s *Server) trades(w http.ResponseWriter, r *http.Request) {
	rows, err := s.Store.ListTrades(r.Context(), 800)
	if err != nil {
		writeJSON(w, 500, map[string]string{"error": err.Error()})
		return
	}
	out := make([]map[string]any, 0, len(rows))
	for _, t := range rows {
		out = append(out, tradeView(t))
	}
	writeJSON(w, 200, out)
}

func (s *Server) barLogs(w http.ResponseWriter, r *http.Request) {
	symbol := r.URL.Query().Get("symbol")
	limit := 200
	if v := r.URL.Query().Get("limit"); v != "" {
		if n, err := strconv.Atoi(v); err == nil && n > 0 {
			limit = n
		}
	}
	rows, err := s.Store.ListBarLogs(r.Context(), symbol, limit)
	if err != nil {
		writeJSON(w, 500, map[string]string{"error": err.Error()})
		return
	}
	out := make([]map[string]any, 0, len(rows))
	for _, b := range rows {
		out = append(out, map[string]any{
			"symbol": b.Symbol, "bar_time": b.BarTime, "atr": b.ATR,
			"upper_n": b.UpperN, "lower_n": b.LowerN, "close": b.Close,
			"want_baseline": b.WantBaseline, "want_live": b.WantLS5,
			"live_desired": b.LiveDesired, "live_actual": b.LiveActual,
			"shadow_twin": b.ShadowLS5, "shadow_baseline": b.ShadowBase,
			"breakout_atr": b.BreakoutATR, "vol_rank": b.VolRank,
			"skipped_reason": b.SkippedReason,
		})
	}
	writeJSON(w, 200, out)
}

func (s *Server) analysis(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	live, err := s.Store.ClosedLiveTrades(ctx)
	if err != nil {
		writeJSON(w, 500, map[string]string{"error": err.Error()})
		return
	}
	sn := s.Engine.Snapshot(ctx)
	skips, _ := s.Store.SkipCounts(ctx)
	curve, _ := s.Store.ListCurve(ctx, 4000)

	var wins, losses, closed int
	var sumR, sumNet float64
	var entrySlips, exitSlips, sideSlips, slExitSlips []float64
	lossStreak, maxLossStreak := 0, 0
	dailyNet := map[string]float64{}

	for _, t := range live {
		if !t.Outcome.Valid || t.Outcome.String == "" || t.Outcome.String == "open" {
			continue
		}
		closed++
		v := tradeView(t)
		net := t.Net.Float64
		sumNet += net
		r := v["r_multiple"].(float64)
		sumR += r
		if net > 0 {
			wins++
			lossStreak = 0
		} else {
			losses++
			lossStreak++
			if lossStreak > maxLossStreak {
				maxLossStreak = lossStreak
			}
		}
		if _, ok := v["entry_slip_bps"].(float64); ok && (t.EntryPxLive > 0 || t.EntryPxShadow > 0) {
			entrySlips = append(entrySlips, v["entry_slip_bps"].(float64))
		}
		if _, ok := v["exit_slip_bps"].(float64); ok && (t.ExitPxLive > 0 || t.ExitPxShadow > 0) {
			exitSlips = append(exitSlips, v["exit_slip_bps"].(float64))
		}
		if ss, ok := v["side_slip_bps"].(float64); ok {
			sideSlips = append(sideSlips, ss)
		}
		if t.Outcome.String == "sl" {
			if xs, ok := v["exit_slip_bps"].(float64); ok {
				slExitSlips = append(slExitSlips, xs)
			}
		}
		if t.ExitTime.Valid && t.ExitTime.Int64 > 0 {
			day := time.Unix(t.ExitTime.Int64, 0).UTC().Format("2006-01-02")
			dailyNet[day] += net
		}
	}

	avgR := 0.0
	winRate := 0.0
	if closed > 0 {
		avgR = sumR / float64(closed)
		winRate = float64(wins) / float64(closed)
	}

	days := make([]string, 0, len(dailyNet))
	for d := range dailyNet {
		days = append(days, d)
	}
	sort.Strings(days)
	dailySeries := make([]map[string]any, 0, len(days))
	for _, d := range days {
		dailySeries = append(dailySeries, map[string]any{"date": d, "net": dailyNet[d]})
	}

	daysLive := 0.0
	if sn.GoLiveTS > 0 {
		daysLive = time.Since(time.Unix(sn.GoLiveTS, 0)).Hours() / 24
	}

	writeJSON(w, 200, map[string]any{
		"trades": closed, "wins": wins, "losses": losses,
		"win_rate": winRate, "avg_r": avgR, "expectancy_r": avgR,
		"net": sumNet,
		"loss_streak": lossStreak, "max_loss_streak": maxLossStreak,
		"days_since_go_live": math.Round(daysLive*10) / 10,
		"go_live_ts":         sn.GoLiveTS,
		"entry_slip": map[string]any{
			"median_bps": telemetry.Median(entrySlips),
			"p90_bps":    telemetry.Percentile(entrySlips, 0.90),
			"n":          len(entrySlips),
		},
		"exit_slip": map[string]any{
			"median_bps": telemetry.Median(exitSlips),
			"p90_bps":    telemetry.Percentile(exitSlips, 0.90),
			"n":          len(exitSlips),
		},
		"side_slip": map[string]any{
			"median_bps": telemetry.Median(sideSlips),
			"p90_bps":    telemetry.Percentile(sideSlips, 0.90),
			"n":          len(sideSlips),
		},
		"sl_exit_slip": map[string]any{
			"median_bps": telemetry.Median(slExitSlips),
			"p90_bps":    telemetry.Percentile(slExitSlips, 0.90),
			"n":          len(slExitSlips),
		},
		"skip_counts":    skips,
		"daily_pnl":      dailySeries,
		"curve_points":   len(curve),
		"match_rate_ltd": sn.MatchRateLTD,
		"verdict":        sn.Verdict,
	})
}
