package engine

import (
	"context"
	"encoding/csv"
	"fmt"
	"math"
	"os"
	"path/filepath"
	"strconv"
	"time"

	"donchian.trade/bot/internal/notify"
	"donchian.trade/bot/internal/plot"
	"donchian.trade/bot/internal/store"
	"donchian.trade/bot/internal/strategy"
	"donchian.trade/bot/internal/telemetry"
)

func (e *Engine) dataDir() string {
	dir := filepath.Dir(e.Cfg.SQLitePath)
	if dir == "" || dir == "." {
		dir = "data"
	}
	return dir
}

func (e *Engine) toClosed(rows []store.TradeRow, book string) []telemetry.ClosedTrade {
	out := make([]telemetry.ClosedTrade, 0, len(rows))
	for _, r := range rows {
		if r.Outcome.String == "rejected" {
			continue
		}
		open := r.Outcome.String == "open" || r.Outcome.String == ""
		t := telemetry.ClosedTrade{
			Key:      telemetry.Key(r.Symbol, r.Direction, r.SignalTime),
			Book:     book,
			Outcome:  r.Outcome.String,
			Stop:     r.Stop.Float64,
			Gross:    r.Gross.Float64,
			Net:      r.Net.Float64,
			Funding:  r.Funding.Float64,
			RiskUSD:  r.Quantity.Float64 * r.RiskDistance.Float64,
			EntryFee: r.EntryFee.Float64,
			ExitFee:  r.ExitFee.Float64,
			Open:     open,
		}
		if book == telemetry.BookLive {
			t.EntryLive = r.EntryPxLive
			if t.EntryLive == 0 {
				t.EntryLive = r.EntryPrice.Float64
			}
			t.ExitLive = r.ExitPxLive
			if t.ExitLive == 0 {
				t.ExitLive = r.ExitPrice.Float64
			}
			t.EntryShadow = r.EntryPxShadow
			t.ExitShadow = r.ExitPxShadow
		} else {
			t.EntryShadow = r.EntryPrice.Float64
			t.ExitShadow = r.ExitPrice.Float64
		}
		out = append(out, t)
	}
	return out
}

func (e *Engine) bookMark(book string) float64 {
	e.mu.Lock()
	defer e.mu.Unlock()
	sum := 0.0
	var src map[string]strategy.State
	switch book {
	case telemetry.BookLS5:
		src = e.shadowLS5
	case telemetry.BookBaseline:
		src = e.shadowBase
	default:
		return 0
	}
	for sym, st := range src {
		sum += markPnL(st, e.bars[sym])
	}
	return sum
}

func (e *Engine) shadowDisplay(book string) float64 {
	net, _, _, err := e.Store.SumNet(context.Background(), book)
	if err != nil {
		net = 0
	}
	return e.cashBase() + net + e.bookMark(book)
}

func (e *Engine) liveDisplay(ctx context.Context) (eq, avail, wallet, upnl, funding float64) {
	if pt, err := e.Store.LatestEquity(ctx, "exchange"); err == nil {
		eq = pt.Equity
		avail = pt.Available
		wallet = pt.Available
	}
	e.mu.Lock()
	for sym, st := range e.liveST {
		upnl += markPnL(st, e.bars[sym])
		if st.Equity > 0 {
			eq = st.Equity
		}
	}
	e.mu.Unlock()
	_, _, funding, _ = e.Store.SumNet(ctx, telemetry.BookLive)
	if eq <= 0 {
		eq = e.cashBase()
	}
	return
}

func (e *Engine) ComputeReport(ctx context.Context) telemetry.Report {
	liveRows, _ := e.Store.TradesByProfile(ctx, telemetry.BookLive)
	ls5Rows, _ := e.Store.TradesByProfile(ctx, telemetry.BookLS5)
	eq, _, _, _, funding := e.liveDisplay(ctx)
	liveXF := eq - funding
	ls5 := e.shadowDisplay(telemetry.BookLS5)
	base := e.shadowDisplay(telemetry.BookBaseline)
	liveCurve, shCurve, _ := e.Store.CurveEquities(ctx)
	if len(liveCurve) == 0 {
		liveCurve = []float64{eq}
		shCurve = []float64{ls5}
	}
	return telemetry.Build(telemetry.Input{
		StartEquity:          e.cashBase(),
		TypicalRisk:          e.typicalRisk(),
		EquityLive:           eq,
		EquityLiveExFunding:  liveXF,
		EquityShadowLS5:      ls5,
		EquityShadowBaseline: base,
		LiveEqCurve:          liveCurve,
		ShadowEqCurve:        shCurve,
		Live:                 e.toClosed(liveRows, telemetry.BookLive),
		Shadow:               e.toClosed(ls5Rows, telemetry.BookLS5),
		Now:                  time.Now().Unix(),
	})
}

func (e *Engine) snapshotBooks(ctx context.Context, reason string) {
	rep := e.ComputeReport(ctx)
	eq, avail, wallet, upnl, funding := e.liveDisplay(ctx)
	_, feesLive, _, _ := e.Store.SumNet(ctx, telemetry.BookLive)
	_, feesLS5, _, _ := e.Store.SumNet(ctx, telemetry.BookLS5)
	p := store.CurvePoint{
		TS:                   time.Now().Unix(),
		Reason:               reason,
		EquityLive:           eq,
		EquityLiveExFunding:  eq - funding,
		UpnlLive:             upnl,
		WalletCash:           wallet,
		EquityShadowLS5:      rep.EquityShadowLS5,
		EquityShadowBaseline: rep.EquityShadowBaseline,
		GapVsLS5:             (eq - funding) - rep.EquityShadowLS5,
		CumFunding:           funding,
		CumFeesLive:          feesLive,
		CumFeesShadowLS5:     feesLS5,
		MatchRateLTD:         rep.MatchRateLTD,
		SideSlipMedianBps:    rep.SideSlipMedianBps,
		Verdict:              rep.Verdict,
	}
	if p.WalletCash == 0 {
		p.WalletCash = avail
	}
	_ = e.Store.InsertCurve(ctx, p)
	_ = e.appendCurveCSV(p)
	e.persistMismatches(ctx)
	e.noteVerdict(ctx, rep)
	_ = avail
}

func (e *Engine) persistMismatches(ctx context.Context) {
	liveRows, _ := e.Store.TradesByProfile(ctx, telemetry.BookLive)
	ls5Rows, _ := e.Store.TradesByProfile(ctx, telemetry.BookLS5)
	liveKeys := map[telemetry.TradeKey]struct{}{}
	shKeys := map[telemetry.TradeKey]struct{}{}
	for _, t := range e.toClosed(liveRows, telemetry.BookLive) {
		liveKeys[t.Key] = struct{}{}
	}
	for _, t := range e.toClosed(ls5Rows, telemetry.BookLS5) {
		shKeys[t.Key] = struct{}{}
	}
	for k := range liveKeys {
		if _, ok := shKeys[k]; !ok {
			if has, _ := e.Store.HasMismatch(ctx, "live_only", k.Symbol, k.Direction, k.SignalTime); !has {
				_ = e.Store.InsertMismatch(ctx, "live_only", k.Symbol, k.Direction, k.SignalTime, "live entry without shadow-ls5")
			}
		}
	}
	for k := range shKeys {
		if _, ok := liveKeys[k]; !ok {
			if has, _ := e.Store.HasMismatch(ctx, "shadow_only", k.Symbol, k.Direction, k.SignalTime); !has {
				_ = e.Store.InsertMismatch(ctx, "shadow_only", k.Symbol, k.Direction, k.SignalTime, "shadow-ls5 entry without live")
			}
		}
	}
}

func (e *Engine) noteVerdict(ctx context.Context, rep telemetry.Report) {
	g, _ := e.Store.LoadGlobal(ctx)
	prev := g.LastVerdict
	if prev == rep.Verdict {
		return
	}
	g.LastVerdict = rep.Verdict
	_ = e.Store.SaveGlobal(ctx, g)
	e.mu.Lock()
	e.lastVerdict = rep.Verdict
	e.mu.Unlock()
	if prev == "" && rep.Verdict == telemetry.VerdictWAIT {
		return
	}
	level := "info"
	if rep.Verdict == telemetry.VerdictInvestigate {
		level = "warn"
	}
	if rep.Verdict == telemetry.VerdictSTOP {
		level = "error"
		e.alert(ctx, level, "verdict", fmt.Sprintf("verdict STOP (A=%s B=%s C=%s) — flatten live, shadow continues", rep.StatusA, rep.StatusB, rep.StatusC))
		if err := e.KillAll(ctx); err != nil {
			e.alert(ctx, "error", "verdict", "STOP flatten failed: "+err.Error())
		}
		return
	}
	e.alert(ctx, level, "verdict", fmt.Sprintf("verdict %s A=%s B=%s C=%s gap=%+.0f", rep.Verdict, rep.StatusA, rep.StatusB, rep.StatusC, rep.GapUSD))
}

func (e *Engine) skipNewEntries() bool {
	e.mu.Lock()
	v := e.lastVerdict
	e.mu.Unlock()
	return v == telemetry.VerdictSTOP
}

func (e *Engine) maybeDailyReport(ctx context.Context, barTime int64) {
	t := time.Unix(barTime, 0).UTC()
	if t.Hour() != 23 {
		return
	}
	date := t.Format("2006-01-02")
	last, _ := e.Store.LastDailyReportDate(ctx)
	if last == date {
		return
	}
	e.WriteDailyReport(ctx, date)
}

func (e *Engine) WriteDailyReport(ctx context.Context, date string) {
	rep := e.ComputeReport(ctx)
	e.mu.Lock()
	extra := ""
	for _, sym := range e.Cfg.Symbols {
		st := e.liveST[sym]
		sh := e.shadowLS5[sym]
		extra += fmt.Sprintf("%s live_streak=%d live_pos=%s shadow_streak=%d shadow_pos=%s\n",
			sym, st.ConsecLosses, posLabel(st), sh.ConsecLosses, posLabel(sh))
	}
	e.mu.Unlock()
	body := telemetry.FormatDaily(date, rep, extra)
	_ = e.Store.SaveDailyReport(ctx, date, body, rep.Verdict)
	dir := filepath.Join(e.dataDir(), "reports")
	_ = os.MkdirAll(dir, 0o755)
	_ = os.WriteFile(filepath.Join(dir, date+".txt"), []byte(body), 0o644)
	_ = e.Store.InsertEvent(ctx, "info", "daily", "суточный отчёт "+date+" · "+rep.Verdict, nil)
	e.Notify.Report(ctx, notify.ReportMail{
		Date:    date,
		Caption: "Суточный отчёт " + date + " UTC",
		PNG:     e.equityPNG(ctx),
	})
	if t, err := time.Parse("2006-01-02", date); err == nil && t.Weekday() == time.Sunday {
		e.weeklyReplay(ctx)
	}
}

func (e *Engine) equityPNG(ctx context.Context) []byte {
	curve, err := e.Store.ListCurve(ctx, 2000)
	if err != nil || len(curve) < 2 {
		return nil
	}
	live := make([]float64, len(curve))
	sh := make([]float64, len(curve))
	for i, p := range curve {
		live[i] = p.EquityLive
		sh[i] = p.EquityShadowLS5
	}
	png, err := plot.EquityPNG(live, sh)
	if err != nil {
		return nil
	}
	return png
}

func (e *Engine) weeklyReplay(ctx context.Context) {
	g, _ := e.Store.LoadGlobal(ctx)
	y, w := time.Now().UTC().ISOWeek()
	week := fmt.Sprintf("%d-W%02d", y, w)
	if g.LastReplayISO == week {
		return
	}
	nCash, _ := e.Store.CashFlowCount(ctx)
	if nCash > 1 {
		e.alert(ctx, "info", "replay", "weekly shadow equity replay skipped — cash-flows after seed change sizing path")
		g.LastReplayISO = week
		_ = e.Store.SaveGlobal(ctx, g)
		return
	}
	start := e.cashBase()
	if start <= 0 {
		start = 10000
	}
	tol := math.Max(1, 0.001*start)
	maxDiff := 0.0
	e.mu.Lock()
	syms := append([]string(nil), e.Cfg.Symbols...)
	copies := map[string][]strategy.Bar{}
	for _, sym := range syms {
		copies[sym] = append([]strategy.Bar(nil), e.bars[sym]...)
	}
	e.mu.Unlock()
	for _, sym := range syms {
		_, final := strategy.Replay(copies[sym], e.liveCfg, start, 0, math.MaxInt64)
		inc, _, _, _ := e.Store.SumNetSymbol(ctx, telemetry.BookLS5, sym)
		incEq := start + inc
		diff := math.Abs(final.Equity - incEq)
		if diff > maxDiff {
			maxDiff = diff
		}
		if diff > tol {
			e.alert(ctx, "error", "replay", fmt.Sprintf("weekly %s shadow replay drift $%.2f (replay=%.2f incremental=%.2f)", sym, diff, final.Equity, incEq))
		}
	}
	if maxDiff <= tol {
		e.alert(ctx, "info", "replay", fmt.Sprintf("weekly shadow replay ok (max Δ=$%.2f)", maxDiff))
	}
	g.LastReplayISO = week
	_ = e.Store.SaveGlobal(ctx, g)
}

func (e *Engine) maybeMissedDaily(ctx context.Context) {
	yesterday := time.Now().UTC().Add(-24 * time.Hour).Format("2006-01-02")
	last, _ := e.Store.LastDailyReportDate(ctx)
	if last == "" || last < yesterday {
		e.WriteDailyReport(ctx, yesterday)
	}
}

func (e *Engine) appendCurveCSV(p store.CurvePoint) error {
	path := filepath.Join(e.dataDir(), "equity_snapshots.csv")
	_ = os.MkdirAll(filepath.Dir(path), 0o755)
	_, err := os.Stat(path)
	newFile := err != nil
	f, err := os.OpenFile(path, os.O_CREATE|os.O_APPEND|os.O_WRONLY, 0o644)
	if err != nil {
		return err
	}
	defer f.Close()
	w := csv.NewWriter(f)
	if newFile {
		_ = w.Write([]string{
			"ts_utc", "reason", "equity_live", "equity_live_ex_funding", "upnl_live", "wallet_cash",
			"equity_shadow_ls5", "equity_shadow_baseline", "gap_vs_ls5", "cum_funding",
			"cum_fees_live", "cum_fees_shadow_ls5", "match_rate_ltd", "side_slip_median_ltd_bps", "verdict",
		})
	}
	_ = w.Write([]string{
		time.Unix(p.TS, 0).UTC().Format(time.RFC3339),
		p.Reason,
		f64(p.EquityLive), f64(p.EquityLiveExFunding), f64(p.UpnlLive), f64(p.WalletCash),
		f64(p.EquityShadowLS5), f64(p.EquityShadowBaseline), f64(p.GapVsLS5), f64(p.CumFunding),
		f64(p.CumFeesLive), f64(p.CumFeesShadowLS5), f64(p.MatchRateLTD), f64(p.SideSlipMedianBps), p.Verdict,
	})
	w.Flush()
	return w.Error()
}

func f64(v float64) string { return strconv.FormatFloat(v, 'f', 8, 64) }
