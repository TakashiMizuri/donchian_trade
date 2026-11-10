package engine

import (
	"context"
	"fmt"
	"math"
	"strings"

	"donchian.trade/bot/internal/exchange/lighter"
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

// stopIsResting is true when trigger can sit on the book without firing at mark.
// buyToCover is a short (buy stop): trigger must be strictly above mark.
func stopIsResting(buyToCover bool, trigger, mark float64) bool {
	if trigger <= 0 || mark <= 0 {
		return false
	}
	if buyToCover {
		return trigger > mark
	}
	return trigger < mark
}

func isProtectiveStop(o lighter.OpenOrder) bool {
	if o.ReduceOnly || o.Trigger > 0 {
		return true
	}
	t := strings.ToLower(o.Type)
	return strings.Contains(t, "stop")
}

// intendedProtectiveStop is the exchange SL that matches paper for the next bar:
// lagged trail when it can rest vs mark, otherwise the original ATR stop.
func intendedProtectiveStop(pos *strategy.Position, bars []strategy.Bar, cfg strategy.Config, mark float64) (stop float64, kind string, trail float64, trailOK bool) {
	if pos == nil {
		return 0, "", 0, false
	}
	orig := pos.Stop
	trail, trailOK = strategy.TrailStopForBar(bars, len(bars), pos, cfg)
	if !trailOK {
		return orig, "orig", 0, false
	}
	buyToCover := pos.Direction == strategy.DirSell
	if stopIsResting(buyToCover, trail, mark) {
		return trail, "trail", trail, true
	}
	return orig, "orig_trail_through", trail, true
}

func pickKeepStop(orders []lighter.OpenOrder, want float64) lighter.OpenOrder {
	best := orders[0]
	bestDist := math.Inf(1)
	if best.Trigger > 0 && want > 0 {
		bestDist = math.Abs(best.Trigger - want)
	}
	for _, o := range orders[1:] {
		d := math.Inf(1)
		if o.Trigger > 0 && want > 0 {
			d = math.Abs(o.Trigger - want)
		}
		if d < bestDist {
			best = o
			bestDist = d
		}
	}
	return best
}

func slOrderFields(o lighter.OpenOrder, rawTrigger float64) map[string]any {
	return map[string]any{
		"order_idx":  o.OrderIndex,
		"client_idx": o.ClientOrderIndex,
		"trigger":    o.Trigger,
		"trigger_raw": rawTrigger,
		"price":      o.Price,
		"reduce_only": o.ReduceOnly,
		"is_ask":     o.IsAsk,
		"type":       o.Type,
		"status":     o.Status,
		"remaining":  o.Remaining,
	}
}

func (e *Engine) flog(kind string, fields map[string]any) {
	if e.Flog != nil {
		e.Flog.Event(kind, fields)
	}
}

func (e *Engine) priceDecimals(symbol string) int {
	if m, ok := e.Markets[symbol]; ok {
		return m.PriceDecimals
	}
	return 0
}

func (e *Engine) normalizeOrderTrigger(o lighter.OpenOrder, ref float64, decimals int) (norm, raw float64) {
	raw = o.Trigger
	norm = lighter.NormalizeTrigger(raw, ref, decimals)
	return norm, raw
}

// maybeArmTrailStop rests the lagged trail on the exchange for the next bar when
// that trigger is still a valid stop vs mark. Paper uses the same trail on the
// next closed bar; a through-market trail is not placed (Decide market-exits).
func (e *Engine) maybeArmTrailStop(ctx context.Context, symbol string, bars []strategy.Bar, st strategy.State) {
	if st.Position == nil || e.liveCfg.LateTrailOnBars <= 0 {
		return
	}
	mark := e.LastPrice(symbol)
	want, kind, trail, trailOK := intendedProtectiveStop(st.Position, bars, e.liveCfg, mark)
	if kind != "trail" {
		if trailOK {
			e.flog("sl_skip_through_market", map[string]any{
				"symbol": symbol,
				"side":   st.Position.Direction,
				"trail":  trail,
				"orig":   st.Position.Stop,
				"mark":   mark,
				"kept":   want,
			})
			e.Log.Info("trail through mark — keeping orig SL",
				"symbol", symbol, "trail", trail, "mark", mark, "orig", st.Position.Stop)
		}
		return
	}
	if stopClose(e.slTriggerPx(symbol), want) {
		return
	}
	e.advanceProtectiveStop(ctx, symbol, st, want, "trail")
}

func (e *Engine) advanceProtectiveStop(ctx context.Context, symbol string, st strategy.State, stop float64, reason string) {
	if st.Position == nil || e.Cfg.DryRun || e.Signer == nil || stop <= 0 {
		return
	}
	mark := e.LastPrice(symbol)
	buyToCover := st.Position.Direction == strategy.DirSell
	if mark > 0 && !stopIsResting(buyToCover, stop, mark) {
		e.flog("sl_skip_through_market", map[string]any{
			"symbol": symbol, "reason": reason, "stop": stop, "mark": mark,
			"side": st.Position.Direction,
		})
		return
	}
	oldClient := e.slClientIdx(symbol)
	oldTrig := e.slTriggerPx(symbol)
	idx, err := e.placeProtectiveStop(ctx, symbol, st, stop, reason)
	if err != nil {
		e.alert(ctx, "error", "stop", symbol+" failed to place "+reason+" SL: "+err.Error()+" — left previous SL in place")
		return
	}
	e.cancelProtectiveStops(ctx, symbol, idx, 0)
	e.flog("sl_advance", map[string]any{
		"symbol": symbol, "reason": reason, "stop": stop, "mark": mark,
		"old_stop": oldTrig, "old_client_idx": oldClient, "new_client_idx": idx,
		"side": st.Position.Direction,
	})
	e.Log.Info("advanced protective SL", "symbol", symbol, "reason", reason, "stop", stop, "prev", oldTrig)
}

func (e *Engine) placeProtectiveStop(ctx context.Context, symbol string, st strategy.State, stop float64, reason string) (int64, error) {
	pos := st.Position
	if pos == nil {
		return 0, nil
	}
	if e.Signer == nil {
		return 0, fmt.Errorf("signer is not configured")
	}
	buy := pos.Direction == strategy.DirBuy
	slIdx, err := e.Store.NextClientOrderIndex(ctx)
	if err != nil {
		return 0, err
	}
	dec := e.priceDecimals(symbol)
	e.txMu.Lock()
	_, slErr := e.Signer.StopLoss(symbol, buy, pos.Quantity, stop, e.Cfg.MarketSlippage, slIdx)
	e.txMu.Unlock()
	if slErr != nil {
		e.flog("sl_place_failed", map[string]any{
			"symbol": symbol, "reason": reason, "stop": stop, "client_idx": slIdx,
			"qty": pos.Quantity, "side": pos.Direction, "err": slErr.Error(),
			"mark": e.LastPrice(symbol), "scaled_trigger": lighter.ScalePrice(stop, dec),
		})
		return 0, slErr
	}
	_ = e.Store.InsertOrder(ctx, symbol, slIdx, "stop", "open", true, 0, stop, pos.Quantity, "")
	e.setOrderIdx(symbol, 0, slIdx)
	e.setSLTrigger(symbol, stop)
	e.flog("sl_placed", map[string]any{
		"symbol": symbol, "reason": reason, "stop": stop, "client_idx": slIdx,
		"qty": pos.Quantity, "side": pos.Direction, "mark": e.LastPrice(symbol),
		"price_decimals": dec, "scaled_trigger": lighter.ScalePrice(stop, dec),
	})
	return slIdx, nil
}

func (e *Engine) cancelProtectiveStops(ctx context.Context, symbol string, keepClient, keepOrder int64) {
	if e.Cfg.DryRun {
		return
	}
	meta := e.Markets[symbol]
	orders, err := e.HTTP.ActiveOrders(ctx, e.Cfg.AccountIndex, meta.MarketID)
	if err != nil {
		e.Log.Warn("list orders", "err", err)
		e.flog("sl_cancel", map[string]any{"symbol": symbol, "phase": "list", "err": err.Error()})
		return
	}
	for _, o := range orders {
		if !isProtectiveStop(o) {
			continue
		}
		if keepClient > 0 && o.ClientOrderIndex == keepClient {
			continue
		}
		if keepOrder > 0 && o.OrderIndex == keepOrder {
			continue
		}
		e.txMu.Lock()
		_, err := e.Signer.Cancel(symbol, o.OrderIndex)
		if err != nil && o.ClientOrderIndex > 0 {
			_, err = e.Signer.Cancel(symbol, o.ClientOrderIndex)
		}
		e.txMu.Unlock()
		fields := slOrderFields(o, o.Trigger)
		fields["symbol"] = symbol
		if err != nil {
			fields["err"] = err.Error()
		}
		e.flog("sl_cancel", fields)
	}
}
