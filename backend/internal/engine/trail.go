package engine

import (
	"context"
	"math"

	"donchian.trade/bot/internal/strategy"
)

func stopClose(a, b float64) bool {
	if a == b {
		return true
	}
	scale := math.Max(math.Abs(a), math.Abs(b))
	if scale < 1 {
		scale = 1
	}
	return math.Abs(a-b)/scale < 1e-6
}

// maybeArmTrailStop moves the exchange SL to the lagged trail for the next bar.
func (e *Engine) maybeArmTrailStop(ctx context.Context, symbol string, bars []strategy.Bar, lastClosedIdx int, st strategy.State) {
	if st.Position == nil || e.liveCfg.LateTrailOnBars <= 0 {
		return
	}
	trail, ok := strategy.TrailStopForBar(bars, lastClosedIdx+1, st.Position, e.liveCfg)
	if !ok {
		return
	}
	if stopClose(e.slTriggerPx(symbol), trail) {
		return
	}
	e.replaceProtectiveStop(ctx, symbol, st, trail)
}

func (e *Engine) replaceProtectiveStop(ctx context.Context, symbol string, st strategy.State, stop float64) {
	if st.Position == nil || e.Cfg.DryRun || e.Signer == nil || stop <= 0 {
		return
	}
	pos := st.Position
	buy := pos.Direction == strategy.DirBuy
	e.cancelStops(ctx, symbol)
	slIdx, err := e.Store.NextClientOrderIndex(ctx)
	if err != nil {
		e.alert(ctx, "error", "stop", symbol+" trail SL index: "+err.Error())
		return
	}
	e.txMu.Lock()
	_, slErr := e.Signer.StopLoss(symbol, buy, pos.Quantity, stop, e.Cfg.MarketSlippage, slIdx)
	e.txMu.Unlock()
	if slErr != nil {
		e.alert(ctx, "error", "stop", symbol+" failed to replace SL with trail: "+slErr.Error())
		return
	}
	_ = e.Store.InsertOrder(ctx, symbol, slIdx, "stop", "open", true, 0, stop, pos.Quantity, "")
	e.setOrderIdx(symbol, 0, slIdx)
	e.setSLTrigger(symbol, stop)
	if e.Flog != nil {
		e.Flog.Event("sl_trail", map[string]any{"symbol": symbol, "stop": stop, "client_idx": slIdx})
	}
	e.Log.Info("armed late trail SL", "symbol", symbol, "stop", stop)
}
