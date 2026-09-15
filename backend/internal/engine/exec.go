package engine

import (
	"context"

	"donchian.trade/bot/internal/exchange/lighter"
	"donchian.trade/bot/internal/strategy"
	"donchian.trade/bot/internal/telemetry"
)

// livePnL is layer-B accounting: actual fills, not research −1R on stops.
// funding is already in PnL sign (positive = received).
func livePnL(dir strategy.Direction, qty, entry, exit, entryFee, exitFee, funding float64) (gross, net float64) {
	gross = strategy.GrossMove(dir, entry, exit) * qty
	net = gross - entryFee - exitFee + funding
	return
}

// fundingPnL converts Lighter total_funding_paid_out into trade PnL.
// The exchange field grows when we pay. PnL is the opposite.
func fundingPnL(basisPaidOut, lastPaidOut float64) float64 {
	return basisPaidOut - lastPaidOut
}

func (e *Engine) waitFill(ctx context.Context, symbol string, clientIdx int64, txHash string, fallback float64) (px, fee float64) {
	if e.Cfg.DryRun || e.HTTP == nil {
		return fallback, 0
	}
	meta, ok := e.Markets[symbol]
	if !ok {
		return fallback, 0
	}
	px, fee = e.HTTP.WaitFill(ctx, e.Cfg.AccountIndex, meta.MarketID, clientIdx, txHash, 0)
	if px > 0 {
		return px, fee
	}
	if avg := e.positionAvg(ctx, symbol); avg > 0 {
		e.Log.Warn("fill missing, using position avg", "symbol", symbol, "client", clientIdx, "avg", avg)
		return avg, 0
	}
	return fallback, 0
}

func (e *Engine) positionAvg(ctx context.Context, symbol string) float64 {
	if e.HTTP == nil {
		return 0
	}
	acc, err := e.HTTP.Account(ctx, e.Cfg.AccountIndex)
	if err != nil || acc == nil {
		return 0
	}
	meta := e.Markets[symbol]
	for _, p := range acc.Positions {
		if p.MarketID == meta.MarketID && p.Size > 0 && p.AvgEntry > 0 {
			return p.AvgEntry
		}
	}
	return 0
}

func (e *Engine) markFunding(acc *lighter.Account) {
	if acc == nil {
		return
	}
	e.idxMu.Lock()
	defer e.idxMu.Unlock()
	if e.fundingLast == nil {
		e.fundingLast = map[string]float64{}
	}
	for _, p := range acc.Positions {
		sym := p.Symbol
		if s, ok := e.SymByID[p.MarketID]; ok {
			sym = s
		}
		if sym == "" {
			continue
		}
		e.fundingLast[sym] = p.FundingPaid
	}
}

func (e *Engine) seedFundingBasis(symbol string) {
	e.idxMu.Lock()
	defer e.idxMu.Unlock()
	if e.fundingBasis == nil {
		e.fundingBasis = map[string]float64{}
	}
	e.fundingBasis[symbol] = e.fundingLast[symbol]
}

func (e *Engine) pullFunding(symbol string) float64 {
	e.idxMu.Lock()
	defer e.idxMu.Unlock()
	return fundingPnL(e.fundingBasis[symbol], e.fundingLast[symbol])
}

func (e *Engine) persistOpenFunding(ctx context.Context) {
	e.idxMu.Lock()
	last := map[string]float64{}
	basis := map[string]float64{}
	for k, v := range e.fundingLast {
		last[k] = v
	}
	for k, v := range e.fundingBasis {
		basis[k] = v
	}
	e.idxMu.Unlock()
	for _, sym := range e.Cfg.Symbols {
		open, err := e.Store.OpenLiveTrade(ctx, sym)
		if err != nil || open == nil {
			continue
		}
		_ = e.Store.SetTradeFunding(ctx, open.ID, fundingPnL(basis[sym], last[sym]))
	}
}

func (e *Engine) recordEntryFill(ctx context.Context, symbol string, tradeID, clientIdx int64, txHash string, shadowPx float64) float64 {
	livePx, fee := e.waitFill(ctx, symbol, clientIdx, txHash, 0)
	if livePx <= 0 {
		e.Log.Warn("entry fill not found, using signal price", "symbol", symbol, "client", clientIdx, "px", shadowPx)
		livePx = shadowPx
		fee = 0
	}
	_ = e.Store.SetEntryLive(ctx, tradeID, shadowPx, livePx, fee)
	if e.HTTP != nil {
		if acc, err := e.HTTP.Account(ctx, e.Cfg.AccountIndex); err == nil {
			e.markFunding(acc)
		}
	}
	e.seedFundingBasis(symbol)
	return livePx
}

func (e *Engine) finishCloseLive(ctx context.Context, symbol string, tradeID int64, st *strategy.State, t *strategy.Trade, clientIdx int64, txHash string) {
	if t == nil {
		return
	}
	if row, _ := e.Store.TradeBySignal(ctx, symbol, telemetry.BookLive, t.SignalTime); row != nil {
		if row.Outcome.Valid && row.Outcome.String != "" && row.Outcome.String != "open" {
			return
		}
		if tradeID == 0 {
			tradeID = row.ID
		}
	}
	shadowExit := t.ExitPrice
	liveExit, exitFee := e.waitFill(ctx, symbol, clientIdx, txHash, 0)
	if liveExit <= 0 {
		exitFee = 0
		if px := e.LastPrice(symbol); px > 0 {
			e.Log.Warn("exit fill not found, using last price", "symbol", symbol, "client", clientIdx, "px", px)
			liveExit = px
		} else {
			e.Log.Warn("exit fill not found, using shadow exit", "symbol", symbol, "client", clientIdx, "px", shadowExit)
			liveExit = shadowExit
		}
	}
	row, _ := e.Store.TradeBySignal(ctx, symbol, telemetry.BookLive, t.SignalTime)
	liveEntry := t.EntryPrice
	entryFee := 0.0
	qty := t.Quantity
	if row != nil {
		if row.EntryPxLive > 0 {
			liveEntry = row.EntryPxLive
		} else if row.EntryPrice.Float64 > 0 {
			liveEntry = row.EntryPrice.Float64
		}
		if row.EntryFee.Valid {
			entryFee = row.EntryFee.Float64
		}
		if row.Quantity.Float64 > 0 {
			qty = row.Quantity.Float64
		}
		if tradeID == 0 {
			tradeID = row.ID
		}
	}
	funding := e.pullFunding(symbol)
	gross, net := livePnL(t.Direction, qty, liveEntry, liveExit, entryFee, exitFee, funding)
	_ = e.Store.CloseTrade(ctx, tradeID, t.ExitTime, liveExit, string(t.Outcome), gross, entryFee, exitFee, funding, net, st.ConsecLosses, int64(st.PauseUntilIdx))
	_ = e.Store.SetExitShadow(ctx, tradeID, shadowExit)
	e.Risk.AddRealized(net)
}

func (e *Engine) slClientIdx(symbol string) int64 {
	e.idxMu.Lock()
	defer e.idxMu.Unlock()
	return e.slIdx[symbol]
}

func (e *Engine) setOrderIdx(symbol string, entry, sl int64) {
	e.idxMu.Lock()
	defer e.idxMu.Unlock()
	if e.entryIdx == nil {
		e.entryIdx = map[string]int64{}
	}
	if e.slIdx == nil {
		e.slIdx = map[string]int64{}
	}
	if entry != 0 {
		e.entryIdx[symbol] = entry
	}
	if sl != 0 {
		e.slIdx[symbol] = sl
	}
}

func (e *Engine) clearOrderIdx(symbol string) {
	e.idxMu.Lock()
	delete(e.entryIdx, symbol)
	delete(e.slIdx, symbol)
	delete(e.fundingBasis, symbol)
	e.idxMu.Unlock()
}

func (e *Engine) snapshotIdx(symbol string) (entry, sl int64, basis, last float64) {
	e.idxMu.Lock()
	defer e.idxMu.Unlock()
	return e.entryIdx[symbol], e.slIdx[symbol], e.fundingBasis[symbol], e.fundingLast[symbol]
}
