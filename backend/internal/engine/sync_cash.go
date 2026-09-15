package engine

import (
	"context"
	"fmt"
	"time"

	"donchian.trade/bot/internal/exchange/lighter"
	"donchian.trade/bot/internal/store"
	"donchian.trade/bot/internal/telemetry"
)

func accountNumbers(acc *lighter.Account) (eq, wallet, upnl float64) {
	if acc == nil {
		return 0, 0, 0
	}
	for _, p := range acc.Positions {
		upnl += p.UnrealizedPnL
	}
	eq = acc.TotalAssetValue
	if eq <= 0 {
		eq = acc.Collateral
	}
	wallet = SettledWallet(eq, upnl)
	if abs64(wallet) < 1e-9 && acc.Collateral > 0 {
		wallet = acc.Collateral
	}
	return eq, wallet, upnl
}

func (e *Engine) cashBase() float64 {
	n, err := e.Store.SumCash(context.Background())
	if err != nil {
		return 0
	}
	return n
}

func (e *Engine) openFeeEstimate(ctx context.Context) float64 {
	rows, err := e.Store.TradesByProfile(ctx, telemetry.BookLive)
	if err != nil {
		return 0
	}
	sum := 0.0
	for _, r := range rows {
		if r.Outcome.String != "open" {
			continue
		}
		if r.EntryFee.Valid {
			sum += r.EntryFee.Float64
		}
	}
	return sum
}

func (e *Engine) applyExchangeSnapshot(ctx context.Context, acc *lighter.Account) {
	if acc == nil {
		return
	}
	eq, _, _ := accountNumbers(acc)
	_ = e.Store.InsertEquity(ctx, "exchange", eq, acc.Available)
	e.mu.Lock()
	for s, st := range e.liveST {
		st.Equity = eq
		e.liveST[s] = st
	}
	e.mu.Unlock()
	e.markFunding(acc)
	e.persistOpenFunding(ctx)
}

func (e *Engine) syncCash(ctx context.Context, acc *lighter.Account) {
	eq, wallet, _ := accountNumbers(acc)
	realized, _, _, _ := e.Store.SumNet(ctx, telemetry.BookLive)
	openFees := e.openFeeEstimate(ctx)
	watch, err := e.Store.LoadCashWatch(ctx)
	if err != nil {
		e.Log.Warn("cash watch load", "err", err)
		return
	}
	minUSD := e.Cfg.CashFlowMinUSD
	if !watch.Inited {
		watch = store.CashWatch{Inited: true, LastWallet: wallet, LastRealized: realized, LastOpenFees: openFees}
		_ = e.Store.SaveCashWatch(ctx, watch)
		// wallet − realized + openFees = deposits/withdrawals to date (works on upgrade too)
		implied := wallet - realized + openFees
		if abs64(implied) < cashThreshold(minUSD, eq) {
			implied = eq
		}
		if abs64(implied) >= cashThreshold(minUSD, eq) {
			e.recordCash(ctx, implied, "seed", eq, wallet, 0, "first live snapshot — shadow cash base (auto)")
		}
		return
	}
	flow := ResidualCash(watch.LastWallet, wallet, watch.LastRealized, realized, watch.LastOpenFees, openFees)
	explained := (wallet - watch.LastWallet) - flow
	watch.LastWallet = wallet
	watch.LastRealized = realized
	watch.LastOpenFees = openFees
	_ = e.Store.SaveCashWatch(ctx, watch)
	if abs64(flow) < cashThreshold(minUSD, eq) {
		return
	}
	e.recordCash(ctx, flow, classifyCash(flow), eq, wallet, explained, "auto-detected: live wallet move not explained by trading")
}

func (e *Engine) recordCash(ctx context.Context, amount float64, kind string, liveEq, wallet, explained float64, note string) {
	id, err := e.Store.InsertCashFlow(ctx, store.CashFlow{
		Amount: amount, Kind: kind, LiveEquity: liveEq, LiveWallet: wallet, Explained: explained, Note: note,
	})
	if err != nil {
		e.Log.Warn("cash flow persist", "err", err)
		return
	}
	e.creditShadowCash(amount)
	g, _ := e.Store.LoadGlobal(ctx)
	if g.GoLiveTS == 0 {
		g.GoLiveTS = time.Now().Unix()
	}
	g.StartEquity = e.cashBase()
	_ = e.Store.SaveGlobal(ctx, g)
	e.alert(ctx, "info", "cash", fmt.Sprintf("%s %+.2f → shadow cash base %.2f (id=%d)", kind, amount, e.cashBase(), id))
}

func (e *Engine) creditShadowCash(amount float64) {
	e.mu.Lock()
	defer e.mu.Unlock()
	for sym, st := range e.shadowLS5 {
		st.Equity += amount
		e.shadowLS5[sym] = st
	}
	for sym, st := range e.shadowBase {
		st.Equity += amount
		e.shadowBase[sym] = st
	}
}

func abs64(x float64) float64 {
	if x < 0 {
		return -x
	}
	return x
}
