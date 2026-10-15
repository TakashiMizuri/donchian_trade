package engine

import (
	"context"

	"donchian.trade/bot/internal/store"
	"donchian.trade/bot/internal/strategy"
	"donchian.trade/bot/internal/telemetry"
)

func bookKey(symbol, book string) string {
	if book == telemetry.BookLive || book == "" {
		return symbol
	}
	return symbol + ":" + book
}

func posLabel(st strategy.State) string {
	if st.Position == nil {
		return "FLAT"
	}
	return string(st.Position.Direction)
}

func markPnL(st strategy.State, bars []strategy.Bar) float64 {
	if st.Position == nil || len(bars) == 0 {
		return 0
	}
	px := bars[len(bars)-1].Close
	sign := 1.0
	if st.Position.Direction == strategy.DirSell {
		sign = -1.0
	}
	return st.Position.Quantity * (px - st.Position.EntryPrice) * sign
}

func (e *Engine) typicalRisk() float64 {
	base := e.cashBase()
	if base <= 0 {
		base = 10000
	}
	r := base * (e.liveCfg.RiskPct / 100)
	if e.liveCfg.MaxRiskUSD > 0 && r > e.liveCfg.MaxRiskUSD {
		return e.liveCfg.MaxRiskUSD
	}
	return r
}

func (e *Engine) restoreBook(ctx context.Context, symbol, book string, bars []strategy.Bar) strategy.State {
	net, _, _, _ := e.Store.SumNet(ctx, book)
	st := strategy.NewState(e.cashBase() + net)
	open, err := e.Store.OpenBookTrade(ctx, symbol, book)
	if err == nil && open != nil {
		st.Position = &strategy.Position{
			ID:           int(open.ID),
			Direction:    strategy.Direction(open.Direction),
			EntryPrice:   open.EntryPrice.Float64,
			Stop:         open.Stop.Float64,
			RiskDistance: open.RiskDistance.Float64,
			SignalTime:   open.SignalTime,
			Quantity:     open.Quantity.Float64,
			EntryIdx:     indexOf(bars, open.EntryTime.Int64),
		}
	}
	return st
}

func (e *Engine) persistBook(ctx context.Context, symbol, book string, st strategy.State, bars []strategy.Bar) {
	sst := store.SymbolState{
		BarsSeen: len(bars),
	}
	if len(bars) > 0 {
		sst.LastBarTime = bars[len(bars)-1].Time
	}
	if st.Position != nil {
		sst.LastSignalTime = st.Position.SignalTime
		sst.OpenTradeID = int64(st.Position.ID)
	}
	_ = e.Store.SaveSymbolState(ctx, bookKey(symbol, book), sst)
}

func (e *Engine) stepBook(ctx context.Context, symbol, book string, bars []strategy.Bar, atr []float64, i int, nextOpen float64, nextTime int64, st *strategy.State, cfg strategy.Config) {
	if net, _, _, err := e.Store.SumNet(ctx, book); err == nil {
		st.Equity = e.cashBase() + net
	}
	defer func() {
		net, _, _, _ := e.Store.SumNet(ctx, book)
		st.Equity = e.cashBase() + net
	}()
	act := strategy.Decide(bars, atr, i, nextOpen, nextTime, *st, cfg)
	switch act.Kind {
	case strategy.ActionEnter:
		if st.Position != nil {
			return
		}
		strategy.Apply(st, cfg, bars, act)
		qty := 0.0
		if st.Position != nil {
			qty = st.Position.Quantity
		}
		id, _, err := e.Store.TryOpenTrade(ctx, symbol, book, string(act.Direction), act.SignalTime, act.FillTime, act.Price, act.Stop, act.Risk, qty, 0)
		if err != nil {
			e.Log.Warn("shadow open", "book", book, "err", err)
			return
		}
		if st.Position != nil && id > 0 {
			st.Position.ID = int(id)
		}
	case strategy.ActionExitStop, strategy.ActionExitChannel, strategy.ActionExitTimeStop:
		e.closeBookWithAction(ctx, symbol, book, bars, st, cfg, act)
	}
}

// closeBookWithAction applies a shadow exit and persists it.
func (e *Engine) closeBookWithAction(ctx context.Context, symbol, book string, bars []strategy.Bar, st *strategy.State, cfg strategy.Config, act strategy.Action) {
	if st.Position == nil {
		return
	}
	if net, _, _, err := e.Store.SumNet(ctx, book); err == nil {
		st.Equity = e.cashBase() + net
	}
	sig := st.Position.SignalTime
	t := strategy.Apply(st, cfg, bars, act)
	if t == nil {
		return
	}
	row, err := e.Store.TradeBySignal(ctx, symbol, book, sig)
	if err != nil || row == nil {
		id, _, _ := e.Store.TryOpenTrade(ctx, symbol, book, string(t.Direction), t.SignalTime, t.EntryTime, t.EntryPrice, t.StopLoss, t.RiskDistance, t.Quantity, 0)
		if id > 0 {
			_ = e.Store.CloseTrade(ctx, id, t.ExitTime, t.ExitPrice, string(t.Outcome), t.Gross, t.EntryFee, t.ExitFee, 0, t.Net, 0, 0)
		}
		return
	}
	_ = e.Store.CloseTrade(ctx, row.ID, t.ExitTime, t.ExitPrice, string(t.Outcome), t.Gross, t.EntryFee, t.ExitFee, 0, t.Net, 0, 0)
}

// shouldSyncShadowOnLiveSL is true when a shadow book still holds the *same*
// trade live just stopped on the exchange (same side + signal_time). Matching
// only on side would kill an older twin hold after live already tstop'd and
// re-entered — wrong for the no-tstop shadow experiment.
func shouldSyncShadowOnLiveSL(liveDir strategy.Direction, liveSignalTime int64, shadowPos *strategy.Position) bool {
	return shadowPos != nil &&
		shadowPos.Direction == liveDir &&
		shadowPos.SignalTime == liveSignalTime
}

// syncShadowsAfterLiveSL force-closes twin/baseline when live was flattened by the
// exchange stop, even if the closed candle high/low never crossed the ATR stop.
// Shadow exit is filled at the paper stop (research), not the live slip price.
func (e *Engine) syncShadowsAfterLiveSL(ctx context.Context, symbol string, liveDir strategy.Direction, liveSignalTime, exitTime int64) {
	e.mu.Lock()
	ls5 := e.shadowLS5[symbol]
	base := e.shadowBase[symbol]
	bars := append([]strategy.Bar(nil), e.bars[symbol]...)
	e.mu.Unlock()

	fillIdx := len(bars) - 1
	if fillIdx < 0 {
		fillIdx = 0
	}
	closeOne := func(book string, st *strategy.State, cfg strategy.Config) bool {
		if !shouldSyncShadowOnLiveSL(liveDir, liveSignalTime, st.Position) {
			return false
		}
		stop := st.Position.Stop
		act := strategy.Action{
			Kind:       strategy.ActionExitStop,
			Direction:  st.Position.Direction,
			Price:      stop,
			Stop:       stop,
			Risk:       st.Position.RiskDistance,
			SignalTime: st.Position.SignalTime,
			FillIdx:    fillIdx,
			FillTime:   exitTime,
		}
		e.closeBookWithAction(ctx, symbol, book, bars, st, cfg, act)
		return true
	}

	synced := false
	if e.Cfg.ShadowBrkVol {
		if closeOne(telemetry.BookLS5, &ls5, e.twinCfg) {
			synced = true
		}
	}
	if e.Cfg.ShadowBaseline {
		if closeOne(telemetry.BookBaseline, &base, e.baseCfg) {
			synced = true
		}
	}
	if !synced {
		return
	}
	e.mu.Lock()
	e.shadowLS5[symbol] = ls5
	e.shadowBase[symbol] = base
	e.mu.Unlock()
	e.persistBook(ctx, symbol, telemetry.BookLS5, ls5, bars)
	e.persistBook(ctx, symbol, telemetry.BookBaseline, base, bars)
	if e.Flog != nil {
		e.Flog.Event("shadow_sync", map[string]any{
			"symbol": symbol, "reason": "live_exchange_sl", "dir": string(liveDir),
			"signal_time": liveSignalTime,
			"twin_flat": ls5.Position == nil, "base_flat": base.Position == nil,
		})
	}
	e.alert(ctx, "info", "shadow", symbol+" shadow synced flat after live exchange SL")
}

func (e *Engine) writeBarLog(ctx context.Context, symbol string, bars []strategy.Bar, atr []float64, i int, live, ls5, base strategy.State) {
	if i < 0 || i >= len(bars) {
		return
	}
	upper, lower := strategy.ChannelAt(bars, i, e.liveCfg.ChannelN)
	b := bars[i]
	c := b.Close
	a := 0.0
	if i < len(atr) {
		a = atr[i]
	}
	wantBase := strategy.SignalWant(c, upper, lower)
	probe := strategy.ProbeEntry(bars, atr, i, e.liveCfg)
	wantLive := probe.Direction
	if probe.SkipReason != "" && probe.SkipReason != "ok" {
		wantLive = ""
	}
	skip := probe.SkipReason
	if wantBase == "" {
		skip = ""
	}
	_ = e.Store.UpsertBarLog(ctx, store.BarLog{
		Symbol: symbol, BarTime: b.Time, ATR: a, UpperN: upper, LowerN: lower, Close: c,
		WantBaseline: string(wantBase), WantLS5: string(wantLive),
		PauseActive: false, ResumeReady: true,
		LiveDesired: string(wantLive), LiveActual: posLabel(live),
		ShadowLS5: posLabel(ls5), ShadowBase: posLabel(base),
		BreakoutATR: probe.BreakoutATR, VolRank: probe.VolRank, SkippedReason: skip,
	})
	if e.Flog != nil {
		e.Flog.Event("bar", map[string]any{
			"symbol": symbol, "bar_time": b.Time,
			"open": b.Open, "high": b.High, "low": b.Low, "close": c, "volume": b.Volume,
			"atr": a, "upper_n": upper, "lower_n": lower,
			"vol_rank": probe.VolRank, "breakout_atr": probe.BreakoutATR, "skip": skip,
			"live_desired": string(wantLive), "live_pos": posLabel(live),
			"twin_pos": posLabel(ls5), "base_pos": posLabel(base),
		})
	}
}
