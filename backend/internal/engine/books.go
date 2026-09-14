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
	sst, err := e.Store.LoadSymbolState(ctx, bookKey(symbol, book))
	if err == nil {
		st.ConsecLosses = sst.ConsecLosses
		st.PauseUntilIdx = sst.PauseUntilIdx
	}
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
		ConsecLosses:  st.ConsecLosses,
		PauseUntilIdx: st.PauseUntilIdx,
		BarsSeen:      len(bars),
	}
	if len(bars) > 0 {
		sst.LastBarTime = bars[len(bars)-1].Time
	}
	if st.Position != nil {
		sst.LastSignalTime = st.Position.SignalTime
		sst.OpenTradeID = int64(st.Position.ID)
	}
	if st.PauseUntilIdx >= 0 && st.PauseUntilIdx < len(bars) {
		sst.PauseUntilTime = bars[st.PauseUntilIdx].Time
	} else if st.PauseUntilIdx >= len(bars) && len(bars) > 0 {
		extra := st.PauseUntilIdx - (len(bars) - 1)
		sst.PauseUntilTime = bars[len(bars)-1].Time + int64(extra)*3600
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
	case strategy.ActionExitStop, strategy.ActionExitChannel:
		if st.Position == nil {
			return
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
				_ = e.Store.CloseTrade(ctx, id, t.ExitTime, t.ExitPrice, string(t.Outcome), t.Gross, t.EntryFee, t.ExitFee, 0, t.Net, st.ConsecLosses, int64(st.PauseUntilIdx))
			}
			return
		}
		_ = e.Store.CloseTrade(ctx, row.ID, t.ExitTime, t.ExitPrice, string(t.Outcome), t.Gross, t.EntryFee, t.ExitFee, 0, t.Net, st.ConsecLosses, int64(st.PauseUntilIdx))
	}
}

func (e *Engine) writeBarLog(ctx context.Context, symbol string, bars []strategy.Bar, atr []float64, i int, live, ls5, base strategy.State) {
	if i < 0 || i >= len(bars) {
		return
	}
	upper, lower := strategy.ChannelAt(bars, i, e.liveCfg.ChannelN)
	c := bars[i].Close
	a := 0.0
	if i < len(atr) {
		a = atr[i]
	}
	wantBase := strategy.SignalWant(c, upper, lower)
	wantLS5 := wantBase
	pause := e.liveCfg.LS5.Enabled && i < ls5.PauseUntilIdx
	resume := true
	if pause {
		resume = c > upper+e.liveCfg.LS5.ResumeATR*a || c < lower-e.liveCfg.LS5.ResumeATR*a
		if a <= 0 {
			resume = false
		}
		if !resume {
			wantLS5 = ""
		}
	}
	_ = e.Store.UpsertBarLog(ctx, store.BarLog{
		Symbol: symbol, BarTime: bars[i].Time, ATR: a, UpperN: upper, LowerN: lower, Close: c,
		WantBaseline: string(wantBase), WantLS5: string(wantLS5),
		PauseActive: pause, ResumeReady: !pause || resume,
		LiveDesired: string(wantLS5), LiveActual: posLabel(live),
		ShadowLS5: posLabel(ls5), ShadowBase: posLabel(base),
	})
}
