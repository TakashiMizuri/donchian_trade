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
	"donchian.trade/bot/internal/flog"
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
	Flog    *flog.Logger

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
	twinCfg     strategy.Config
	baseCfg     strategy.Config
	lastVerdict string

	idxMu        sync.Mutex
	entryIdx     map[string]int64
	slIdx        map[string]int64
	fundingBasis map[string]float64
	fundingLast  map[string]float64

	// Serializes Lighter sendTx (nonce/sign). HandleClosed may run per-symbol in parallel.
	txMu sync.Mutex

	alertMu sync.Mutex
	alertAt map[string]time.Time
}

func New(cfg *config.Config, st *store.Store, httpc *lighter.HTTPClient, signer *lighter.Signer, markets map[string]lighter.MarketMeta, ntf notify.Notifier, rg *risk.Guard, log *slog.Logger) *Engine {
	live := cfg.LiveConfig()
	twin := cfg.ShadowTwinConfig()
	base := cfg.BaselineConfig()
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
		started: time.Now(), liveCfg: live, twinCfg: twin, baseCfg: base,
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
	if e.Flog != nil && (level == "error" || level == "warn") {
		e.Flog.Event(level, map[string]any{"kind": kind, "message": msg})
	}
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
	if e.Flog != nil {
		ev := map[string]any{"connected": ok}
		if err != "" {
			ev["error"] = err
		}
		e.Flog.Event("ws", ev)
	}
	if !ok && err != "" {
		e.alert(ctx, "warn", "ws", "Lighter WS disconnected: "+err)
	}
}

func (e *Engine) backfillSymbol(ctx context.Context, sym string, meta lighter.MarketMeta, from time.Time, tf time.Duration) error {
	upsert := func(page []strategy.Bar) error {
		return e.Store.UpsertCandles(ctx, sym, page)
	}
	existing, err := e.Store.LoadCandles(ctx, sym)
	if err != nil {
		return err
	}
	now := time.Now().UTC()
	if len(existing) == 0 {
		_, err := e.HTTP.BackfillRange(ctx, meta.MarketID, e.Cfg.Resolution, tf, from, now, upsert)
		if err != nil {
			return fmt.Errorf("backfill %s: %w", sym, err)
		}
		return nil
	}
	oldest := time.Unix(existing[0].Time, 0).UTC()
	newest := time.Unix(existing[len(existing)-1].Time, 0).UTC()
	// Resume older history first (partial 1m warmups often die mid-walk).
	if oldest.After(from.Add(tf)) {
		n, err := e.HTTP.BackfillRange(ctx, meta.MarketID, e.Cfg.Resolution, tf, from, oldest, upsert)
		if err != nil {
			return fmt.Errorf("backfill %s history: %w", sym, err)
		}
		e.Log.Info("backfill history", "symbol", sym, "added", len(n), "from", from, "until", oldest)
	}
	tipFrom := newest.Add(-2 * tf)
	if tipFrom.Before(from) {
		tipFrom = from
	}
	n, err := e.HTTP.BackfillRange(ctx, meta.MarketID, e.Cfg.Resolution, tf, tipFrom, now, upsert)
	if err != nil {
		return fmt.Errorf("backfill %s tip: %w", sym, err)
	}
	e.Log.Info("backfill tip", "symbol", sym, "added", len(n))
	return nil
}

func (e *Engine) Bootstrap(ctx context.Context) error {
	if err := e.lockStrategyFingerprint(ctx); err != nil {
		return err
	}
	tf := e.Cfg.Timeframe
	if tf <= 0 {
		tf = time.Hour
	}
	// Prefer strategy warmup (+7d buffer) over a flat 120d window — 1m×120d
	// is ~170k candles and trips venue rate limits on cold start.
	needBars := strategy.Warmup(e.liveCfg) + int((7*24*time.Hour)/tf)
	if needBars < 500 {
		needBars = 500
	}
	fromWarm := time.Now().Add(-time.Duration(needBars) * tf)
	fromCap := time.Now().Add(-e.Cfg.CandleWarmup)
	from := fromWarm
	if from.Before(fromCap) {
		from = fromCap // never exceed configured CandleWarmup
	}
	for _, sym := range e.Cfg.Symbols {
		meta, ok := e.Markets[sym]
		if !ok {
			return fmt.Errorf("no market meta for %s", sym)
		}
		if err := e.backfillSymbol(ctx, sym, meta, from, tf); err != nil {
			return err
		}
		bars, err := e.Store.LoadCandles(ctx, sym)
		if err != nil {
			return err
		}
		bars = dropUnclosed(bars, tf)
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
	if e.Flog != nil {
		mids := make([]uint16, 0, len(e.Cfg.Symbols))
		for _, sym := range e.Cfg.Symbols {
			mids = append(mids, e.IDBySym[sym])
		}
		tf := e.Cfg.Timeframe
		if tf <= 0 {
			tf = time.Hour
		}
		e.Flog.Event("boot", map[string]any{
			"fingerprint":  e.Cfg.Fingerprint(),
			"tag":          e.Cfg.Tag,
			"tf":           e.Cfg.Resolution,
			"tf_sec":       int(tf.Seconds()),
			"brk_atr":      e.Cfg.MinBreakoutATR,
			"vol_rank_max": e.Cfg.MaxVolRank,
			"symbols":      e.Cfg.Symbols,
			"market_ids":   mids,
			"seed_equity":  e.cashBase(),
			"shadow_twin":  e.Cfg.ShadowBrkVol,
			"shadow_base":  e.Cfg.ShadowBaseline,
		})
	}
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
		// Don't block the WS read loop on entry/WaitFill — BTC must not stall ETH.
		bar, open := *closed, live.Open
		go e.HandleClosed(context.Background(), sym, bar, open, "ws")
	}
}

func (e *Engine) HandleClosed(ctx context.Context, symbol string, bar strategy.Bar, nextOpen float64, source string) {
	if source == "" {
		source = "ws"
	}
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
	tf := e.Cfg.Timeframe
	if tf <= 0 {
		tf = time.Hour
	}
	barCloseWall := time.Unix(bar.Time, 0).UTC().Add(tf)
	detectLagMs := time.Since(barCloseWall).Milliseconds()
	if e.Flog != nil {
		e.Flog.Event("bar_closed", map[string]any{
			"symbol": symbol, "bar_time": bar.Time,
			"detect_lag_ms": detectLagMs, "source": source,
		})
	}

	// Hot path: update memory + decide + live order before SQLite shadows/persist.
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
	nextTime := bar.Time + int64(tf.Seconds())

	act := strategy.Decide(bars, atr, i, nextOpen, nextTime, st, e.liveCfg)
	switch act.Kind {
	case strategy.ActionExitStop:
		e.noteStopHit(ctx, symbol, &st, bars, act)
	case strategy.ActionExitChannel, strategy.ActionExitTimeStop:
		label := "channel"
		if act.Kind == strategy.ActionExitTimeStop {
			label = "tstop"
		}
		if err := e.executeExit(ctx, symbol, &st, bars, act, false); err != nil {
			e.alert(ctx, "error", "exit", symbol+" "+label+" exit failed: "+err.Error())
		}
	case strategy.ActionEnter:
		if e.skipNewEntries() {
			e.alert(ctx, "warn", "verdict", symbol+" skip entry: verdict STOP (shadow continues)")
		} else if err := e.executeEntry(ctx, symbol, &st, bars, act, barCloseWall, detectLagMs, source); err != nil {
			e.alert(ctx, "error", "entry", symbol+" entry failed: "+err.Error())
		}
	}

	if err := e.Store.UpsertCandles(ctx, symbol, []strategy.Bar{bar}); err != nil {
		e.alert(ctx, "error", "db", "candle persist: "+err.Error())
	}
	if e.Cfg.ShadowBrkVol {
		// Twin of live filters (brk/vol); tstop usually off. Book key shadow_ls5.
		e.stepBook(ctx, symbol, telemetry.BookLS5, bars, atr, i, nextOpen, nextTime, &ls5, e.twinCfg)
	}
	if e.Cfg.ShadowBaseline {
		e.stepBook(ctx, symbol, telemetry.BookBaseline, bars, atr, i, nextOpen, nextTime, &base, e.baseCfg)
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
		BarsSeen: len(bars),
	}
	if len(bars) > 0 {
		sst.LastBarTime = bars[len(bars)-1].Time
	}
	if st.Position != nil {
		sst.LastSignalTime = st.Position.SignalTime
		sst.OpenTradeID = int64(st.Position.ID)
	}
	entry, sl, basis, last := e.snapshotIdx(symbol)
	sst.EntryClientIdx = entry
	sst.SLClientOrderIdx = sl
	sst.FundingBasis = basis
	sst.FundingLast = last
	_ = e.Store.SaveSymbolState(ctx, symbol, sst)
}

func (e *Engine) executeEntry(ctx context.Context, symbol string, st *strategy.State, bars []strategy.Bar, act strategy.Action, barCloseWall time.Time, detectLagMs int64, source string) error {
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

	sendAt := time.Now()
	lagMs := sendAt.Sub(barCloseWall).Milliseconds()
	if e.Flog != nil {
		e.Flog.Event("entry_intent", map[string]any{
			"symbol": symbol, "direction": act.Direction,
			"qty": qty, "stop": act.Stop, "risk": act.Risk, "ref_px": act.Price,
			"detect_lag_ms": detectLagMs, "source": source,
		})
	}
	e.txMu.Lock()
	res, err := e.Signer.MarketIOC(symbol, buy, qty, act.Price, e.Cfg.MarketSlippage, clientIdx, false)
	e.txMu.Unlock()
	if err != nil {
		_ = e.Store.CloseTrade(ctx, id, time.Now().Unix(), act.Price, "rejected", 0, 0, 0, 0, 0, 0, 0)
		return err
	}
	_ = e.Store.InsertOrder(ctx, symbol, clientIdx, "entry", "sent", false, act.Price, 0, qty, res.TxHash)
	if e.Flog != nil {
		e.Flog.Event("entry_sent", map[string]any{
			"symbol": symbol, "direction": act.Direction, "client_idx": clientIdx,
			"lag_ms": lagMs, "tx_hash": res.TxHash, "source": source,
		})
	}
	e.setOrderIdx(symbol, clientIdx, 0)
	e.seedFundingBasis(symbol)

	// Place protective SL before fill polling — WaitFill must not delay the stop.
	slIdx, err := e.Store.NextClientOrderIndex(ctx)
	if err != nil {
		return err
	}
	e.txMu.Lock()
	_, slErr := e.Signer.StopLoss(symbol, buy, qty, act.Stop, e.Cfg.MarketSlippage, slIdx)
	e.txMu.Unlock()
	if slErr != nil {
		if e.Flog != nil {
			e.Flog.Event("sl_failed", map[string]any{"symbol": symbol, "stop": act.Stop, "err": slErr.Error()})
		}
		e.alert(ctx, "error", "stop", symbol+" failed to place SL after entry: "+slErr.Error()+" — flattening")
		e.txMu.Lock()
		_, err2 := e.Signer.MarketIOC(symbol, !buy, qty, act.Price, e.Cfg.MarketSlippage, slIdx+1, true)
		e.txMu.Unlock()
		if err2 != nil {
			return fmt.Errorf("entry ok but SL failed (%v) and flatten failed (%v)", slErr, err2)
		}
		_ = e.Store.CloseTrade(ctx, id, time.Now().Unix(), act.Price, string(strategy.OutcomeWatch), 0, 0, 0, 0, 0, 0, 0)
		e.clearOrderIdx(symbol)
		return slErr
	}
	if e.Flog != nil {
		e.Flog.Event("sl_placed", map[string]any{"symbol": symbol, "stop": act.Stop, "client_idx": slIdx})
	}
	_ = e.Store.InsertOrder(ctx, symbol, slIdx, "stop", "open", true, 0, act.Stop, qty, "")
	e.setOrderIdx(symbol, clientIdx, slIdx)
	strategy.Apply(st, e.liveCfg, bars, act)
	if st.Position != nil {
		st.Position.ID = int(id)
		st.Position.Quantity = qty
	}

	e.Log.Info("entry lag",
		"symbol", symbol,
		"source", source,
		"detect_lag_ms", detectLagMs,
		"lag_ms", lagMs,
		"bar_close_wall", barCloseWall.UTC().Format(time.RFC3339),
		"send_at", sendAt.UTC().Format(time.RFC3339Nano),
		"hash", res.TxHash,
	)
	e.alert(ctx, "info", "entry", fmt.Sprintf("%s %s qty=%.6f px=%.2f stop=%.2f source=%s lag_ms=%d detect_lag_ms=%d hash=%s",
		symbol, act.Direction, qty, act.Price, act.Stop, source, lagMs, detectLagMs, res.TxHash))

	txHash := res.TxHash
	go e.recordEntryFill(context.Background(), symbol, id, clientIdx, txHash, act.Price, act.Direction)
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
		e.finishClose(ctx, symbol, int64(pos.ID), st, t, act)
		e.alert(ctx, "info", "exit", fmt.Sprintf("[dry-run] %s %s %s px=%.2f", symbol, act.Kind, pos.Direction, act.Price))
		return nil
	}
	if e.Signer == nil {
		return fmt.Errorf("signer is not configured")
	}
	if e.Flog != nil {
		e.Flog.Event("exit_intent", map[string]any{
			"symbol": symbol, "kind": act.Kind, "direction": pos.Direction,
			"qty": qty, "ref_px": act.Price,
		})
	}
	clientIdx, err := e.Store.NextClientOrderIndex(ctx)
	if err != nil {
		return err
	}
	e.txMu.Lock()
	res, err := e.Signer.MarketIOC(symbol, !buy, qty, act.Price, e.Cfg.MarketSlippage, clientIdx, true)
	e.txMu.Unlock()
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
		e.finishClose(ctx, symbol, int64(pos.ID), st, t, act)
	} else {
		e.finishCloseLive(ctx, symbol, int64(pos.ID), st, t, e.slClientIdx(symbol), "")
		e.clearOrderIdx(symbol)
	}
	e.alert(ctx, "info", "sl", fmt.Sprintf("%s ATR stop hit at %.2f", symbol, act.Price))
}

func (e *Engine) finishClose(ctx context.Context, symbol string, id int64, st *strategy.State, t *strategy.Trade, act strategy.Action) {
	if t == nil {
		return
	}
	_ = e.Store.CloseTrade(ctx, id, t.ExitTime, t.ExitPrice, string(t.Outcome), t.Gross, t.EntryFee, t.ExitFee, 0, t.Net, 0, 0)
	e.Risk.AddRealized(t.Net)
	if e.Flog != nil {
		e.Flog.Event("exit", map[string]any{
			"symbol": symbol, "outcome": t.Outcome, "exit_px": t.ExitPrice,
			"entry_px": t.EntryPrice, "net": t.Net, "kind": act.Kind, "dry_run": true,
		})
	}
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
			e.txMu.Lock()
			_, err := e.Signer.Cancel(symbol, o.OrderIndex)
			if err != nil && o.ClientOrderIndex > 0 {
				_, _ = e.Signer.Cancel(symbol, o.ClientOrderIndex)
			}
			e.txMu.Unlock()
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
			msg := fmt.Sprintf("%s unexpected exchange position size=%.6f — flattening", sym, ex.Size)
			if e.Flog != nil {
				e.Flog.Event("reconcile", map[string]any{"symbol": sym, "issue": "unexpected_position", "size": ex.Size})
			}
			e.alert(ctx, "error", "reconcile", msg)
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
				if e.Flog != nil {
					e.Flog.Event("reconcile", map[string]any{"symbol": sym, "issue": "side_mismatch"})
				}
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
			e.finishClose(ctx, p.sym, p.id, &p.st, t, act)
		} else {
			e.finishCloseLive(ctx, p.sym, p.id, &p.st, t, e.slClientIdx(p.sym), "")
			e.clearOrderIdx(p.sym)
		}
		e.alert(ctx, "info", "reconcile", p.sym+" exchange flat — closed local as SL/fill")
		if e.Flog != nil {
			e.Flog.Event("reconcile", map[string]any{"symbol": p.sym, "issue": "exchange_flat_closed_local"})
		}
		e.mu.Lock()
		st := e.liveST[p.sym]
		st.Position = nil
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
	e.txMu.Lock()
	_, err = e.Signer.MarketIOC(symbol, buy, ex.Size, px, e.Cfg.MarketSlippage, idx, true)
	e.txMu.Unlock()
	if err != nil {
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
	e.pollClosedBars(ctx, 0, "reconcile")
}

// pollClosedBars fetches recent candles and feeds closed bars into HandleClosed.
// If expectedClosed > 0, returns whether every symbol has already processed that bar
// (via WS or a prior poll). HandleClosed is idempotent.
func (e *Engine) pollClosedBars(ctx context.Context, expectedClosed int64, source string) bool {
	tf := e.Cfg.Timeframe
	if tf <= 0 {
		tf = time.Hour
	}
	res := e.Cfg.Resolution
	if res == "" {
		res = "1h"
	}
	cutoff := time.Now().UTC().Truncate(tf).Unix()
	lookback := 6 * time.Hour
	if lookback < 12*tf {
		lookback = 12 * tf
	}
	countBack := int(lookback/tf) + 2
	if countBack < 12 {
		countBack = 12
	}
	if countBack > 500 {
		countBack = 500
	}
	for _, sym := range e.Cfg.Symbols {
		meta := e.Markets[sym]
		end := time.Now()
		start := end.Add(-lookback)
		kl, err := e.HTTP.Candles(ctx, meta.MarketID, res, start.UnixMilli(), end.UnixMilli(), countBack)
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
			e.HandleClosed(ctx, sym, b, nextOpen, source)
		}
	}
	if expectedClosed <= 0 {
		return true
	}
	return e.haveClosedBar(expectedClosed)
}

func (e *Engine) haveClosedBar(barTime int64) bool {
	e.mu.Lock()
	defer e.mu.Unlock()
	for _, sym := range e.Cfg.Symbols {
		if e.lastBar[sym] < barTime {
			return false
		}
	}
	return len(e.Cfg.Symbols) > 0
}

// After each UTC bar boundary, poll REST every 200ms until the closed bar is seen
// (or timeout). Complements trade-driven WS so quiet markets do not fall through
// to the 15s reconcile ticker (~8s median lag).
const (
	boundaryPollInterval = 200 * time.Millisecond
	boundaryPollTimeout  = 5 * time.Second
)

// RunBoundaryPoll waits for each bar boundary, then polls until the prior bar is
// ingested for all symbols. HandleClosed is idempotent if WS wins the race.
func (e *Engine) RunBoundaryPoll(ctx context.Context) {
	tf := e.Cfg.Timeframe
	if tf <= 0 {
		tf = time.Hour
	}
	for {
		if ctx.Err() != nil {
			return
		}
		now := time.Now().UTC()
		boundary := now.Truncate(tf).Add(tf)
		if wait := time.Until(boundary); wait > 0 {
			select {
			case <-ctx.Done():
				return
			case <-time.After(wait):
			}
		}
		expectedClosed := boundary.Add(-tf).Unix()
		deadline := boundary.Add(boundaryPollTimeout)
		attempt := 0
		for {
			if ctx.Err() != nil {
				return
			}
			attempt++
			offsetMs := time.Since(boundary).Milliseconds()
			allSeen := e.pollClosedBars(ctx, expectedClosed, "boundary")
			if e.Flog != nil {
				e.Flog.Event("boundary_poll", map[string]any{
					"attempt":         attempt,
					"offset_ms":       offsetMs,
					"boundary":        boundary.UTC().Format(time.RFC3339),
					"expected_closed": expectedClosed,
					"all_seen":        allSeen,
				})
			}
			if allSeen || time.Now().UTC().After(deadline) {
				break
			}
			select {
			case <-ctx.Done():
				return
			case <-time.After(boundaryPollInterval):
			}
		}
		// Align to the following boundary.
		next := time.Now().UTC().Truncate(tf).Add(tf)
		if wait := time.Until(next); wait > 0 {
			select {
			case <-ctx.Done():
				return
			case <-time.After(wait):
			}
		}
	}
}

type Snapshot struct {
	Network              string           `json:"network"`
	Strategy             string           `json:"strategy"`
	InstanceID           string           `json:"instance_id"`
	InstanceName         string           `json:"instance_name"`
	KillSwitch           bool             `json:"kill_switch"`
	DryRun               bool             `json:"dry_run"`
	WSConnected          bool             `json:"ws_connected"`
	WSError              string           `json:"ws_error"`
	UptimeSec            int64            `json:"uptime_sec"`
	DailyPnL             float64          `json:"daily_pnl"`
	Equity               float64          `json:"equity"`
	EquityShadowLS5      float64          `json:"equity_shadow_ls5"`
	EquityShadowTwin     float64          `json:"equity_shadow_twin"`
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
	ShadowTwin      string  `json:"shadow_twin"`
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
		Strategy:             e.Cfg.Tag,
		InstanceID:           e.Cfg.InstanceID,
		InstanceName:         e.Cfg.InstanceName,
		KillSwitch:           e.Risk.Kill(),
		DryRun:               e.Cfg.DryRun,
		WSConnected:          e.wsOK,
		WSError:              e.wsErr,
		UptimeSec:            int64(time.Since(e.started).Seconds()),
		DailyPnL:             daily,
		Equity:               eq,
		EquityShadowLS5:      rep.EquityShadowLS5,
		EquityShadowTwin:     rep.EquityShadowLS5,
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
		item := SymbolSnap{
			Symbol:          sym,
			MarketID:        e.IDBySym[sym],
			LastBarTime:     e.lastBar[sym],
			LastPrice:       e.lastPrice[sym],
			Bars:            len(e.bars[sym]),
			Position:        "FLAT",
			ConsecLosses:    0,
			PauseUntilTime:  0,
			Paused:          false,
			ShadowLS5:       posLabel(sh),
			ShadowTwin:      posLabel(sh),
			ShadowBaseline:  posLabel(base),
			ShadowConsecLS5: 0,
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

func (e *Engine) lockStrategyFingerprint(ctx context.Context) error {
	fp := e.Cfg.Fingerprint()
	old, err := e.Store.LoadKV(ctx, config.FingerprintKey)
	if err != nil {
		return err
	}
	if old == fp {
		return nil
	}
	if old != "" {
		return fmt.Errorf("sqlite is frozen as %q but this process is %q — archive data/bot.db and start a new run", old, fp)
	}
	has, err := e.Store.HasMarketHistory(ctx)
	if err != nil {
		return err
	}
	if has {
		return fmt.Errorf("sqlite already has candles/trades but no strategy fingerprint (old 1h run). Archive data/bot.db before starting %s", e.Cfg.Tag)
	}
	return e.Store.SaveKV(ctx, config.FingerprintKey, fp)
}
