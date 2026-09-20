package engine

import (
	"context"
	"fmt"
	"log/slog"
	"sort"
	"sync"
	"time"

	"donchian.trade/bot/internal/config"
	"donchian.trade/bot/internal/exchange/lighter"
	"donchian.trade/bot/internal/notify"
	"donchian.trade/bot/internal/risk"
	"donchian.trade/bot/internal/store"
	"donchian.trade/bot/internal/strategy"
	"donchian.trade/bot/internal/telemetry"
)

type Engine struct {
	Cfg     *config.Config
	Store   *store.Store
	HTTP    *lighter.HTTPClient
	Signer  *lighter.Signer
	Markets map[string]lighter.MarketMeta
	IDBySym map[string]uint16
	SymByID map[uint16]string
	Notify  notify.Notifier
	Risk    *risk.Guard
	Log     *slog.Logger

	mu          sync.Mutex
	bars        map[string][]strategy.Bar
	liveST      map[string]strategy.State
	shadowLS5   map[string]strategy.State
	shadowBase  map[string]strategy.State
	lastPrice   map[string]float64
	lastBar     map[string]int64
	busy        map[string]bool
	wsOK        bool
	wsErr       string
	started     time.Time
	liveCfg     strategy.Config
	baseCfg     strategy.Config
	lastVerdict string

	idxMu        sync.Mutex
	entryIdx     map[string]int64
	slIdx        map[string]int64
	fundingBasis map[string]float64
	fundingLast  map[string]float64

	alertMu sync.Mutex
	alertAt map[string]time.Time
}

func New(cfg *config.Config, st *store.Store, httpc *lighter.HTTPClient, signer *lighter.Signer, markets map[string]lighter.MarketMeta, ntf notify.Notifier, rg *risk.Guard, log *slog.Logger) *Engine {
	live := strategy.DefaultConfig()
	live.FeeRate = cfg.FeeRate
	base := strategy.DefaultConfig()
	base.FeeRate = cfg.FeeRate
	base.LS5.Enabled = false
	idBy := map[string]uint16{}
	symBy := map[uint16]string{}
	for s, m := range markets {
		idBy[s] = m.MarketID
		symBy[m.MarketID] = s
	}
	if ntf == nil {
		ntf = notify.Nop{}
	}
	if log == nil {
		log = slog.Default()
	}
	return &Engine{
		Cfg: cfg, Store: st, HTTP: httpc, Signer: signer, Markets: markets,
		IDBySym: idBy, SymByID: symBy, Notify: ntf, Risk: rg, Log: log,
		bars: map[string][]strategy.Bar{}, liveST: map[string]strategy.State{},
		shadowLS5: map[string]strategy.State{}, shadowBase: map[string]strategy.State{},
		lastPrice: map[string]float64{}, lastBar: map[string]int64{}, busy: map[string]bool{},
		entryIdx: map[string]int64{}, slIdx: map[string]int64{},
		fundingBasis: map[string]float64{}, fundingLast: map[string]float64{},
		alertAt: map[string]time.Time{},
		started: time.Now(), liveCfg: live, baseCfg: base,
	}
}

const alertRepeatEvery = time.Hour

func SuppressRepeat(at map[string]time.Time, now time.Time, window time.Duration, level, kind, msg string) bool {
	if (level != "warn" && level != "error") || at == nil {
		return false
	}
	key := kind + "\n" + msg
	if t, ok := at[key]; ok && now.Sub(t) < window {
		return true
	}
	at[key] = now
	return false
}

func (e *Engine) alert(ctx context.Context, level, kind, msg string) {
	e.Log.Info(msg, "level", level, "kind", kind)
	e.alertMu.Lock()
	skip := SuppressRepeat(e.alertAt, time.Now(), alertRepeatEvery, level, kind, msg)
	e.alertMu.Unlock()
	if skip {
		return
	}
	_ = e.Store.InsertEvent(ctx, level, kind, msg, nil)
	e.Notify.Alert(ctx, level, kind, msg)
	g, _ := e.Store.LoadGlobal(ctx)
	if level == "error" || level == "warn" {
		g.LastError = msg
		_ = e.Store.SaveGlobal(ctx, g)
	}
}

func (e *Engine) SetWS(ok bool, err string) {
	e.mu.Lock()
	e.wsOK = ok
	e.wsErr = err
	e.mu.Unlock()
	ctx := context.Background()
	g, _ := e.Store.LoadGlobal(ctx)
	g.WSConnected = ok
	if err != "" {
		g.LastError = err
	}
	_ = e.Store.SaveGlobal(ctx, g)
	if !ok && err != "" {
		e.alert(ctx, "warn", "ws", "Lighter WS disconnected: "+err)
	}
}

func (e *Engine) Bootstrap(ctx context.Context) error {
	from := time.Now().Add(-e.Cfg.CandleWarmup)
	for _, sym := range e.Cfg.Symbols {
		meta, ok := e.Markets[sym]
		if !ok {
			return fmt.Errorf("no market meta for %s", sym)
		}
		existing, err := e.Store.LoadCandles(ctx, sym)
		if err != nil {
			return err
		}
		needFrom := from
		if len(existing) > 0 {
			needFrom = time.Unix(existing[len(existing)-1].Time, 0).Add(-2 * time.Hour)
		}
		fresh, err := e.HTTP.Backfill1h(ctx, meta.MarketID, needFrom)
		if err != nil {
			return fmt.Errorf("backfill %s: %w", sym, err)
		}
		if err := e.Store.UpsertCandles(ctx, sym, fresh); err != nil {
			return err
		}
		bars, err := e.Store.LoadCandles(ctx, sym)
		if err != nil {
			return err
		}
		bars = dropUnclosed(bars, time.Hour)
		e.mu.Lock()
		e.bars[sym] = bars
		if len(bars) > 0 {
			e.lastBar[sym] = bars[len(bars)-1].Time
			e.lastPrice[sym] = bars[len(bars)-1].Close
		}
		e.mu.Unlock()

		sst, err := e.Store.LoadSymbolState(ctx, sym)
		if err != nil {
			return err
		}
		live := strategy.NewState(e.cashBase())
		live.ConsecLosses = sst.ConsecLosses
		live.PauseUntilIdx = sst.PauseUntilIdx
		if open, err := e.Store.OpenLiveTrade(ctx, sym); err == nil && open != nil {
			live.Position = &strategy.Position{
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
		e.idxMu.Lock()
		e.entryIdx[sym] = sst.EntryClientIdx
		e.slIdx[sym] = sst.SLClientOrderIdx
		e.fundingBasis[sym] = sst.FundingBasis
		e.fundingLast[sym] = sst.FundingLast
		e.idxMu.Unlock()
		ls5 := e.restoreBook(ctx, sym, telemetry.BookLS5, bars)
		base := e.restoreBook(ctx, sym, telemetry.BookBaseline, bars)
		e.mu.Lock()
		e.liveST[sym] = live
		e.shadowLS5[sym] = ls5
		e.shadowBase[sym] = base
		e.mu.Unlock()
		e.Log.Info("bootstrapped", "symbol", sym, "bars", len(bars))
	}
	acc, err := e.HTTP.Account(ctx, e.Cfg.AccountIndex)
	if err == nil && acc != nil {
		e.applyExchangeSnapshot(ctx, acc)
		e.syncCash(ctx, acc)
	}
	g, _ := e.Store.LoadGlobal(ctx)
	e.mu.Lock()
	e.lastVerdict = g.LastVerdict
	e.mu.Unlock()
	e.snapshotBooks(ctx, "boot")
	e.maybeMissedDaily(ctx)
	_ = e.Store.PruneBarLogs(ctx, time.Now().Add(-120*24*time.Hour).Unix())
	return nil
}

func dropUnclosed(bars []strategy.Bar, tf time.Duration) []strategy.Bar {
	if len(bars) == 0 {
		return bars
	}
	cutoff := time.Now().UTC().Truncate(tf).Unix()
	out := bars[:0]
	for _, b := range bars {
		if b.Time < cutoff {
			out = append(out, b)
		}
	}
	return out
}

func indexOf(bars []strategy.Bar, t int64) int {
	for i, b := range bars {
		if b.Time == t {
			return i
		}
	}
	return len(bars) - 1
}

func (e *Engine) OnLiveCandle(marketID uint16, closed *strategy.Bar, live strategy.Bar) {
	sym := e.SymByID[marketID]
	if sym == "" {
		return
	}
	e.mu.Lock()
	if live.Close > 0 {
		e.lastPrice[sym] = live.Close
	}
	e.mu.Unlock()
	if closed != nil {
		e.HandleClosed(context.Background(), sym, *closed, live.Open)
	}
}

func (e *Engine) HandleClosed(ctx context.Context, symbol string, bar strategy.Bar, nextOpen float64) {
	e.mu.Lock()
	if e.busy[symbol] {
		e.mu.Unlock()
		return
	}
	if bar.Time <= e.lastBar[symbol] && candleIn(e.bars[symbol], bar.Time) {
		// already processed this closed bar
		e.mu.Unlock()
		return
	}
	e.busy[symbol] = true
	e.mu.Unlock()
	defer func() {
		e.mu.Lock()
		e.busy[symbol] = false
		e.mu.Unlock()
	}()

	if nextOpen <= 0 {
		nextOpen = bar.Close
	}
	if err := e.Store.UpsertCandles(ctx, symbol, []strategy.Bar{bar}); err != nil {
		e.alert(ctx, "error", "db", "candle persist: "+err.Error())
	}
	e.mu.Lock()
	bars := append(e.bars[symbol], bar)
	sort.Slice(bars, func(i, j int) bool { return bars[i].Time < bars[j].Time })
	bars = uniqueBars(bars)
	e.bars[symbol] = bars
	e.lastBar[symbol] = bar.Time
	st := e.liveST[symbol]
	ls5 := e.shadowLS5[symbol]
	base := e.shadowBase[symbol]
	e.mu.Unlock()

	i := len(bars) - 1
	atr := strategy.ATR(bars, e.liveCfg.ATRPeriod)
	nextTime := bar.Time + int64(e.Cfg.Timeframe.Seconds())

	if e.Cfg.ShadowLS5 {
		e.stepBook(ctx, symbol, telemetry.BookLS5, bars, atr, i, nextOpen, nextTime, &ls5, e.liveCfg)
	}
	if e.Cfg.ShadowBaseline {
		e.stepBook(ctx, symbol, telemetry.BookBaseline, bars, atr, i, nextOpen, nextTime, &base, e.baseCfg)
	}

	act := strategy.Decide(bars, atr, i, nextOpen, nextTime, st, e.liveCfg)
	switch act.Kind {
	case strategy.ActionExitStop:
		e.noteStopHit(ctx, symbol, &st, bars, act)
	case strategy.ActionExitChannel:
		if err := e.executeExit(ctx, symbol, &st, bars, act, false); err != nil {
			e.alert(ctx, "error", "exit", symbol+" channel exit failed: "+err.Error())
		}
	case strategy.ActionEnter:
		if e.skipNewEntries() {
			e.alert(ctx, "warn", "verdict", symbol+" skip entry: verdict STOP (shadow continues)")
		} else if err := e.executeEntry(ctx, symbol, &st, bars, act); err != nil {
			e.alert(ctx, "error", "entry", symbol+" entry failed: "+err.Error())
		}
	}
	e.writeBarLog(ctx, symbol, bars, atr, i, st, ls5, base)
	e.persistSymbol(ctx, symbol, st, bars)
	e.persistBook(ctx, symbol, telemetry.BookLS5, ls5, bars)
	e.persistBook(ctx, symbol, telemetry.BookBaseline, base, bars)
	e.mu.Lock()
	e.liveST[symbol] = st
	e.shadowLS5[symbol] = ls5
	e.shadowBase[symbol] = base
	e.mu.Unlock()
	e.snapshotBooks(ctx, "bar")
	e.maybeDailyReport(ctx, bar.Time)
}

func candleIn(bars []strategy.Bar, t int64) bool {
	for _, b := range bars {
		if b.Time == t {
			return true
		}
	}
	return false
}

func uniqueBars(bars []strategy.Bar) []strategy.Bar {
	out := bars[:0]
	var last int64 = -1
	for _, b := range bars {
		if b.Time == last {
			if len(out) > 0 {
				out[len(out)-1] = b
			}
			continue
		}
		out = append(out, b)
		last = b.Time
	}
	return out
}

func (e *Engine) persistSymbol(ctx context.Context, symbol string, st strategy.State, bars []strategy.Bar) {
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
	entry, sl, basis, last := e.snapshotIdx(symbol)
	sst.EntryClientIdx = entry
	sst.SLClientOrderIdx = sl
	sst.FundingBasis = basis
	sst.FundingLast = last
	_ = e.Store.SaveSymbolState(ctx, symbol, sst)
}

func (e *Engine) executeEntry(ctx context.Context, symbol string, st *strategy.State, bars []strategy.Bar, act strategy.Action) error {
	if ok, why := e.Risk.AllowEntry(0, st.Equity); !ok {
		e.alert(ctx, "warn", "risk", symbol+" skip entry: "+why)
		return nil
	}
	if st.Position != nil {
		return nil
	}
	qty := strategy.Quantity(st.Equity, e.liveCfg.RiskPct, e.liveCfg.MaxRiskUSD, act.Risk)
	meta := e.Markets[symbol]
	qty = lighter.RoundQty(qty, meta.SizeDecimals)
	if qty <= 0 {
		return fmt.Errorf("qty rounded to 0")
	}
	notional := qty * act.Price
	if ok, why := e.Risk.AllowEntry(notional, st.Equity); !ok {
		e.alert(ctx, "warn", "risk", symbol+" skip entry: "+why)
		return nil
	}
	clientIdx, err := e.Store.NextClientOrderIndex(ctx)
	if err != nil {
		return err
	}
	id, inserted, err := e.Store.TryOpenTrade(ctx, symbol, "live", string(act.Direction), act.SignalTime, act.FillTime, act.Price, act.Stop, act.Risk, qty, clientIdx)
	if err != nil {
		return err
	}
	if !inserted {
		e.Log.Info("idempotent skip", "symbol", symbol, "signal", act.SignalTime)
		return nil
	}
	buy := act.Direction == strategy.DirBuy
	if e.Cfg.DryRun {
		strategy.Apply(st, e.liveCfg, bars, act)
		if st.Position != nil {
			st.Position.ID = int(id)
			st.Position.Quantity = qty
		}
		e.alert(ctx, "info", "entry", fmt.Sprintf("[dry-run] %s %s qty=%.6f px=%.2f stop=%.2f", symbol, act.Direction, qty, act.Price, act.Stop))
		return nil
	}
	if e.Signer == nil {
		return fmt.Errorf("signer is not configured")
	}
	res, err := e.Signer.MarketIOC(symbol, buy, qty, act.Price, e.Cfg.MarketSlippage, clientIdx, false)
	if err != nil {
		_ = e.Store.CloseTrade(ctx, id, time.Now().Unix(), act.Price, "rejected", 0, 0, 0, 0, 0, st.ConsecLosses, int64(st.PauseUntilIdx))
		return err
	}
	_ = e.Store.InsertOrder(ctx, symbol, clientIdx, "entry", "sent", false, act.Price, 0, qty, res.TxHash)
	e.setOrderIdx(symbol, clientIdx, 0)
	e.recordEntryFill(ctx, symbol, id, clientIdx, res.TxHash, act.Price)
	slIdx, err := e.Store.NextClientOrderIndex(ctx)
	if err != nil {
		return err
	}
	if _, err := e.Signer.StopLoss(symbol, buy, qty, act.Stop, e.Cfg.MarketSlippage, slIdx); err != nil {
		e.alert(ctx, "error", "stop", symbol+" failed to place SL after entry: "+err.Error()+" — flattening")
		if _, err2 := e.Signer.MarketIOC(symbol, !buy, qty, act.Price, e.Cfg.MarketSlippage, slIdx+1, true); err2 != nil {
			return fmt.Errorf("entry ok but SL failed (%v) and flatten failed (%v)", err, err2)
		}
		_ = e.Store.CloseTrade(ctx, id, time.Now().Unix(), act.Price, string(strategy.OutcomeWatch), 0, 0, 0, 0, 0, st.ConsecLosses, int64(st.PauseUntilIdx))
		e.clearOrderIdx(symbol)
		return err
	}
	_ = e.Store.InsertOrder(ctx, symbol, slIdx, "stop", "open", true, 0, act.Stop, qty, "")
	e.setOrderIdx(symbol, clientIdx, slIdx)
	strategy.Apply(st, e.liveCfg, bars, act)
	if st.Position != nil {
		st.Position.ID = int(id)
		st.Position.Quantity = qty
	}
	e.alert(ctx, "info", "entry", fmt.Sprintf("%s %s qty=%.6f px=%.2f stop=%.2f hash=%s", symbol, act.Direction, qty, act.Price, act.Stop, res.TxHash))
	return nil
}

func (e *Engine) executeExit(ctx context.Context, symbol string, st *strategy.State, bars []strategy.Bar, act strategy.Action, reduceOnly bool) error {
	if st.Position == nil {
		return nil
	}
	pos := *st.Position
	buy := pos.Direction == strategy.DirBuy
	qty := pos.Quantity
	if e.Cfg.DryRun {
		t := strategy.Apply(st, e.liveCfg, bars, act)
		e.finishClose(ctx, int64(pos.ID), st, t, act)
		e.alert(ctx, "info", "exit", fmt.Sprintf("[dry-run] %s %s %s px=%.2f", symbol, act.Kind, pos.Direction, act.Price))
		return nil
	}
	if e.Signer == nil {
		return fmt.Errorf("signer is not configured")
	}
	clientIdx, err := e.Store.NextClientOrderIndex(ctx)
	if err != nil {
		return err
	}
	res, err := e.Signer.MarketIOC(symbol, !buy, qty, act.Price, e.Cfg.MarketSlippage, clientIdx, true)
	if err != nil {
		return err
	}
	tx := ""
	if res != nil {
		tx = res.TxHash
	}
	e.cancelStops(ctx, symbol)
	t := strategy.Apply(st, e.liveCfg, bars, act)
	e.finishCloseLive(ctx, symbol, int64(pos.ID), st, t, clientIdx, tx)
	e.clearOrderIdx(symbol)
	e.alert(ctx, "info", "exit", fmt.Sprintf("%s %s %s px=%.2f outcome=%s", symbol, act.Kind, pos.Direction, act.Price, act.Kind))
	return nil
}

func (e *Engine) noteStopHit(ctx context.Context, symbol string, st *strategy.State, bars []strategy.Bar, act strategy.Action) {
	if st.Position == nil {
		return
	}
	pos := *st.Position
	t := strategy.Apply(st, e.liveCfg, bars, act)
	if e.Cfg.DryRun {
		e.finishClose(ctx, int64(pos.ID), st, t, act)
	} else {
		e.finishCloseLive(ctx, symbol, int64(pos.ID), st, t, e.slClientIdx(symbol), "")
		e.clearOrderIdx(symbol)
	}
	e.alert(ctx, "info", "sl", fmt.Sprintf("%s ATR stop hit at %.2f", symbol, act.Price))
}

func (e *Engine) finishClose(ctx context.Context, id int64, st *strategy.State, t *strategy.Trade, act strategy.Action) {
	if t == nil {
		return
	}
	_ = e.Store.CloseTrade(ctx, id, t.ExitTime, t.ExitPrice, string(t.Outcome), t.Gross, t.EntryFee, t.ExitFee, 0, t.Net, st.ConsecLosses, int64(st.PauseUntilIdx))
	e.Risk.AddRealized(t.Net)
}

func (e *Engine) cancelStops(ctx context.Context, symbol string) {
	if e.Cfg.DryRun {
		return
	}
	meta := e.Markets[symbol]
	orders, err := e.HTTP.ActiveOrders(ctx, e.Cfg.AccountIndex, meta.MarketID)
	if err != nil {
		e.Log.Warn("list orders", "err", err)
		return
	}
	for _, o := range orders {
		if o.ReduceOnly || o.Trigger > 0 {
			if _, err := e.Signer.Cancel(symbol, o.OrderIndex); err != nil && o.ClientOrderIndex > 0 {
				_, _ = e.Signer.Cancel(symbol, o.ClientOrderIndex)
			}
		}
	}
}

func (e *Engine) Reconcile(ctx context.Context) {
	acc, err := e.HTTP.Account(ctx, e.Cfg.AccountIndex)
	if err != nil {
		e.alert(ctx, "warn", "reconcile", "account fetch: "+err.Error())
		return
	}
	eq, _, _ := accountNumbers(acc)
	e.applyExchangeSnapshot(ctx, acc)
	byMarket := map[uint16]lighter.Position{}
	for _, p := range acc.Positions {
		if p.Size > 0 {
			byMarket[p.MarketID] = p
		}
	}

	type slClose struct {
		sym  string
		st   strategy.State
		id   int64
		stop float64
		bars []strategy.Bar
	}
	var pending []slClose

	e.mu.Lock()
	for _, sym := range e.Cfg.Symbols {
		st := e.liveST[sym]
		st.Equity = eq
		meta := e.Markets[sym]
		ex, hasEx := byMarket[meta.MarketID]
		local := st.Position != nil
		switch {
		case local && !hasEx:
			open, _ := e.Store.OpenLiveTrade(ctx, sym)
			if open != nil {
				bars := append([]strategy.Bar(nil), e.bars[sym]...)
				pending = append(pending, slClose{sym: sym, st: st, id: open.ID, stop: open.Stop.Float64, bars: bars})
			}
			st.Position = nil
		case !local && hasEx:
			e.alert(ctx, "error", "reconcile", fmt.Sprintf("%s unexpected exchange position size=%.6f — flattening", sym, ex.Size))
			e.mu.Unlock()
			e.flatten(ctx, sym, ex)
			e.mu.Lock()
			st = e.liveST[sym]
		case local && hasEx:
			wantSign := 1
			if st.Position.Direction == strategy.DirSell {
				wantSign = -1
			}
			if ex.Sign != 0 && ex.Sign != wantSign {
				e.alert(ctx, "error", "reconcile", sym+" side mismatch — flattening")
				e.mu.Unlock()
				e.flatten(ctx, sym, ex)
				e.mu.Lock()
				st = e.liveST[sym]
				st.Position = nil
			} else {
				e.ensureStop(ctx, sym, st)
				st = e.liveST[sym]
			}
		}
		e.liveST[sym] = st
		e.persistSymbol(ctx, sym, st, e.bars[sym])
	}
	e.mu.Unlock()

	for _, p := range pending {
		act := strategy.Action{Kind: strategy.ActionExitStop, Price: p.stop, FillTime: time.Now().Unix(), FillIdx: len(p.bars) - 1}
		t := strategy.Apply(&p.st, e.liveCfg, p.bars, act)
		if e.Cfg.DryRun {
			e.finishClose(ctx, p.id, &p.st, t, act)
		} else {
			e.finishCloseLive(ctx, p.sym, p.id, &p.st, t, e.slClientIdx(p.sym), "")
			e.clearOrderIdx(p.sym)
		}
		e.alert(ctx, "info", "reconcile", p.sym+" exchange flat — closed local as SL/fill")
		e.mu.Lock()
		st := e.liveST[p.sym]
		st.Position = nil
		st.ConsecLosses = p.st.ConsecLosses
		st.PauseUntilIdx = p.st.PauseUntilIdx
		e.liveST[p.sym] = st
		e.persistSymbol(ctx, p.sym, st, e.bars[p.sym])
		e.mu.Unlock()
	}
	// After local SL closes so realized/open-fees line up with the exchange snapshot.
	e.syncCash(ctx, acc)
	if len(pending) > 0 {
		e.snapshotBooks(ctx, "fill")
	}
}

func (e *Engine) ensureStop(ctx context.Context, symbol string, st strategy.State) {
	if st.Position == nil || e.Cfg.DryRun {
		return
	}
	meta := e.Markets[symbol]
	orders, err := e.HTTP.ActiveOrders(ctx, e.Cfg.AccountIndex, meta.MarketID)
	if err != nil {
		e.alert(ctx, "warn", "watchdog", symbol+" cannot list orders: "+err.Error())
		return
	}
	hasSL := false
	for _, o := range orders {
		if o.ReduceOnly && o.Trigger > 0 {
			hasSL = true
			break
		}
	}
	if hasSL {
		return
	}
	e.alert(ctx, "error", "watchdog", symbol+" protective stop missing — flattening")
	pos := lighter.Position{Size: st.Position.Quantity, Sign: 1, AvgEntry: st.Position.EntryPrice}
	if st.Position.Direction == strategy.DirSell {
		pos.Sign = -1
	}
	e.mu.Unlock()
	e.flatten(ctx, symbol, pos)
	e.mu.Lock()
}

func (e *Engine) flatten(ctx context.Context, symbol string, ex lighter.Position) {
	if e.Cfg.DryRun || ex.Size <= 0 {
		return
	}
	buy := ex.Sign < 0 // short → buy to flatten
	px := ex.AvgEntry
	e.mu.Lock()
	if e.lastPrice[symbol] > 0 {
		px = e.lastPrice[symbol]
	}
	e.mu.Unlock()
	idx, err := e.Store.NextClientOrderIndex(ctx)
	if err != nil {
		e.alert(ctx, "error", "flatten", err.Error())
		return
	}
	if _, err := e.Signer.MarketIOC(symbol, buy, ex.Size, px, e.Cfg.MarketSlippage, idx, true); err != nil {
		e.alert(ctx, "error", "flatten", symbol+" "+err.Error())
		return
	}
	e.cancelStops(ctx, symbol)
	e.alert(ctx, "warn", "flatten", fmt.Sprintf("%s flattened size=%.6f", symbol, ex.Size))
}

func (e *Engine) KillAll(ctx context.Context) error {
	e.Risk.SetKill(true)
	g, _ := e.Store.LoadGlobal(ctx)
	g.KillSwitch = true
	_ = e.Store.SaveGlobal(ctx, g)
	acc, err := e.HTTP.Account(ctx, e.Cfg.AccountIndex)
	if err != nil {
		e.alert(ctx, "error", "kill", "kill-switch on, but account fetch failed: "+err.Error())
		return err
	}
	for _, p := range acc.Positions {
		if p.Size == 0 {
			continue
		}
		sym := p.Symbol
		if s, ok := e.SymByID[p.MarketID]; ok {
			sym = s
		}
		e.flatten(ctx, sym, p)
	}
	e.alert(ctx, "warn", "kill", "kill-switch engaged — flattened all positions")
	return nil
}

func (e *Engine) Resume(ctx context.Context) {
	e.Risk.SetKill(false)
	g, _ := e.Store.LoadGlobal(ctx)
	g.KillSwitch = false
	g.LastVerdict = ""
	_ = e.Store.SaveGlobal(ctx, g)
	e.mu.Lock()
	e.lastVerdict = ""
	e.mu.Unlock()
	e.alert(ctx, "info", "kill", "kill-switch cleared — new entries allowed (verdict latch reset)")
}

func (e *Engine) PollClosedBars(ctx context.Context) {
	cutoff := time.Now().UTC().Truncate(time.Hour).Unix()
	for _, sym := range e.Cfg.Symbols {
		meta := e.Markets[sym]
		end := time.Now()
		start := end.Add(-6 * time.Hour)
		kl, err := e.HTTP.Candles(ctx, meta.MarketID, "1h", start.UnixMilli(), end.UnixMilli(), 12)
		if err != nil {
			continue
		}
		var nextOpen float64
		for _, b := range kl {
			if b.Time >= cutoff && b.Open > 0 {
				nextOpen = b.Open
				break
			}
		}
		if nextOpen == 0 {
			nextOpen = e.LastPrice(sym)
		}
		for _, b := range kl {
			if b.Time >= cutoff {
				continue
			}
			e.HandleClosed(ctx, sym, b, nextOpen)
		}
	}
}

type Snapshot struct {
	Network              string           `json:"network"`
	Strategy             string           `json:"strategy"`
	KillSwitch           bool             `json:"kill_switch"`
	DryRun               bool             `json:"dry_run"`
	WSConnected          bool             `json:"ws_connected"`
	WSError              string           `json:"ws_error"`
	UptimeSec            int64            `json:"uptime_sec"`
	DailyPnL             float64          `json:"daily_pnl"`
	Equity               float64          `json:"equity"`
	EquityShadowLS5      float64          `json:"equity_shadow_ls5"`
	EquityShadowBaseline float64          `json:"equity_shadow_baseline"`
	EquityLiveExFunding  float64          `json:"equity_live_ex_funding"`
	GapUSD               float64          `json:"gap_usd"`
	GapPct               float64          `json:"gap_pct"`
	PnLRatioXF           float64          `json:"pnl_ratio_xf"`
	PnLRatioOK           bool             `json:"pnl_ratio_ok"`
	Verdict              string           `json:"verdict"`
	StatusA              string           `json:"status_a"`
	StatusB              string           `json:"status_b"`
	StatusC              string           `json:"status_c"`
	MatchRateLTD         float64          `json:"match_rate_ltd"`
	MatchRate7d          float64          `json:"match_rate_7d"`
	SideSlipMedianBps    float64          `json:"side_slip_median_bps"`
	Funding              float64          `json:"funding"`
	StartEquity          float64          `json:"start_equity"`
	CashBase             float64          `json:"cash_base"`
	LastCashKind         string           `json:"last_cash_kind"`
	LastCashAmount       float64          `json:"last_cash_amount"`
	LastCashTS           int64            `json:"last_cash_ts"`
	GoLiveTS             int64            `json:"go_live_ts"`
	Available            float64          `json:"available"`
	LastError            string           `json:"last_error"`
	Symbols              []SymbolSnap     `json:"symbols"`
	Report               telemetry.Report `json:"report"`
}

type SymbolSnap struct {
	Symbol          string  `json:"symbol"`
	MarketID        uint16  `json:"market_id"`
	LastBarTime     int64   `json:"last_bar_time"`
	LastPrice       float64 `json:"last_price"`
	Bars            int     `json:"bars"`
	Position        string  `json:"position"`
	Entry           float64 `json:"entry"`
	Stop            float64 `json:"stop"`
	Qty             float64 `json:"qty"`
	ConsecLosses    int     `json:"consec_losses"`
	PauseUntilTime  int64   `json:"pause_until_time"`
	Paused          bool    `json:"paused"`
	ShadowLS5       string  `json:"shadow_ls5"`
	ShadowBaseline  string  `json:"shadow_baseline"`
	ShadowConsecLS5 int     `json:"shadow_consec_ls5"`
}

func (e *Engine) Snapshot(ctx context.Context) Snapshot {
	g, _ := e.Store.LoadGlobal(ctx)
	eq, avail := 0.0, 0.0
	if pt, err := e.Store.LatestEquity(ctx, "exchange"); err == nil {
		eq = pt.Equity
		avail = pt.Available
	}
	daily, _ := e.Risk.DailyPnL()
	rep := e.ComputeReport(ctx)
	lastCash, _ := e.Store.LatestCashFlow(ctx)
	e.mu.Lock()
	defer e.mu.Unlock()
	sn := Snapshot{
		Network:              string(e.Cfg.Network),
		Strategy:             "1h_N30_M15_ATR1.5+ls5_cond_brk2.0",
		KillSwitch:           e.Risk.Kill(),
		DryRun:               e.Cfg.DryRun,
		WSConnected:          e.wsOK,
		WSError:              e.wsErr,
		UptimeSec:            int64(time.Since(e.started).Seconds()),
		DailyPnL:             daily,
		Equity:               eq,
		EquityShadowLS5:      rep.EquityShadowLS5,
		EquityShadowBaseline: rep.EquityShadowBaseline,
		EquityLiveExFunding:  rep.EquityLiveExFunding,
		GapUSD:               rep.GapUSD,
		GapPct:               rep.GapPct,
		PnLRatioXF:           rep.PnLRatioXF,
		PnLRatioOK:           rep.PnLRatioOK,
		Verdict:              rep.Verdict,
		StatusA:              rep.StatusA,
		StatusB:              rep.StatusB,
		StatusC:              rep.StatusC,
		MatchRateLTD:         rep.MatchRateLTD,
		MatchRate7d:          rep.MatchRate7d,
		SideSlipMedianBps:    rep.SideSlipMedianBps,
		Funding:              rep.Funding,
		StartEquity:          e.cashBase(),
		CashBase:             e.cashBase(),
		LastCashKind:         lastCash.Kind,
		LastCashAmount:       lastCash.Amount,
		LastCashTS:           lastCash.TS,
		GoLiveTS:             g.GoLiveTS,
		Available:            avail,
		LastError:            g.LastError,
		Report:               rep,
		Symbols:              make([]SymbolSnap, 0, len(e.Cfg.Symbols)),
	}
	for _, sym := range e.Cfg.Symbols {
		st := e.liveST[sym]
		sh := e.shadowLS5[sym]
		base := e.shadowBase[sym]
		sst, _ := e.Store.LoadSymbolState(ctx, sym)
		item := SymbolSnap{
			Symbol:          sym,
			MarketID:        e.IDBySym[sym],
			LastBarTime:     e.lastBar[sym],
			LastPrice:       e.lastPrice[sym],
			Bars:            len(e.bars[sym]),
			Position:        "FLAT",
			ConsecLosses:    st.ConsecLosses,
			PauseUntilTime:  sst.PauseUntilTime,
			Paused:          st.PauseUntilIdx > len(e.bars[sym])-1,
			ShadowLS5:       posLabel(sh),
			ShadowBaseline:  posLabel(base),
			ShadowConsecLS5: sh.ConsecLosses,
		}
		if st.Position != nil {
			item.Position = string(st.Position.Direction)
			item.Entry = st.Position.EntryPrice
			item.Stop = st.Position.Stop
			item.Qty = st.Position.Quantity
		}
		sn.Symbols = append(sn.Symbols, item)
	}
	return sn
}

func (e *Engine) Health() (ok bool, detail string) {
	e.mu.Lock()
	defer e.mu.Unlock()
	if e.Risk.Kill() {
		return true, "kill-switch on"
	}
	for _, sym := range e.Cfg.Symbols {
		if len(e.bars[sym]) < strategy.Warmup(e.liveCfg) {
			return false, "warming up " + sym
		}
	}
	if !e.wsOK {
		if e.wsErr != "" {
			return true, "ok; ws down: " + e.wsErr
		}
		return true, "ok; ws down"
	}
	return true, "ok"
}

func (e *Engine) WSState() (ok bool, err string) {
	e.mu.Lock()
	defer e.mu.Unlock()
	return e.wsOK, e.wsErr
}

func (e *Engine) LastPrice(symbol string) float64 {
	e.mu.Lock()
	defer e.mu.Unlock()
	return e.lastPrice[symbol]
}
