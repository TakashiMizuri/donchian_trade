package strategy

import "math"

type Position struct {
	ID           int
	Direction    Direction
	EntryPrice   float64
	EntryIdx     int
	Stop         float64
	RiskDistance float64
	SignalTime   int64
	Quantity     float64
}

type State struct {
	Position      *Position
	ConsecLosses  int
	PauseUntilIdx int
	Equity        float64
	NextID        int
}

func NewState(equity float64) State {
	return State{PauseUntilIdx: -1, Equity: equity, NextID: 1}
}

type ActionKind string

const (
	ActionNone        ActionKind = "none"
	ActionEnter       ActionKind = "enter"
	ActionExitStop    ActionKind = "exit_stop"
	ActionExitChannel ActionKind = "exit_channel"
)

type Action struct {
	Kind       ActionKind
	Direction  Direction
	Price      float64
	Stop       float64
	Risk       float64
	SignalTime int64
	SignalIdx  int
	FillIdx    int
	FillTime   int64
}

func ChannelAt(bars []Bar, i, lookback int) (upper, lower float64) {
	from := i - lookback
	if from < 0 {
		from = 0
	}
	return channelMaxMin(bars, from, i)
}

func SignalWant(close, upper, lower float64) Direction {
	if close > upper {
		return DirBuy
	}
	if close < lower {
		return DirSell
	}
	return ""
}

func channelMaxMin(bars []Bar, from, to int) (upper, lower float64) {
	if to <= from {
		return 0, 0
	}
	upper = bars[from].High
	lower = bars[from].Low
	for j := from + 1; j < to; j++ {
		if bars[j].High > upper {
			upper = bars[j].High
		}
		if bars[j].Low < lower {
			lower = bars[j].Low
		}
	}
	return
}

func resumeReady(st State, cfg Config, i int, upperN, lowerN, c, atrI float64) bool {
	if !cfg.LS5.Enabled || cfg.LS5.StreakN <= 0 {
		return true
	}
	if i >= st.PauseUntilIdx {
		return true
	}
	if atrI <= 0 {
		return false
	}
	if cfg.LS5.ResumeATR > 0 && (c > upperN+cfg.LS5.ResumeATR*atrI || c < lowerN-cfg.LS5.ResumeATR*atrI) {
		return true
	}
	return false
}

func onTradeClose(st *State, cfg Config, dir Direction, entry, exit float64, exitBarI int) {
	if !cfg.LS5.Enabled || cfg.LS5.StreakN <= 0 {
		return
	}
	if GrossMove(dir, entry, exit) > 0 {
		st.ConsecLosses = 0
		return
	}
	st.ConsecLosses++
	if st.ConsecLosses >= cfg.LS5.StreakN {
		pause := exitBarI + cfg.LS5.PauseBars
		if pause > st.PauseUntilIdx {
			st.PauseUntilIdx = pause
		}
		st.ConsecLosses = 0
	}
}

// Decide inspects closed bar i. nextOpen/nextTime stand in for bar i+1 open
// (research fill). Pass the live mark when the next bar has just opened.
func Decide(bars []Bar, atr []float64, i int, nextOpen float64, nextTime int64, st State, cfg Config) Action {
	if i < 0 || i >= len(bars) {
		return Action{Kind: ActionNone}
	}
	if st.Position != nil {
		pos := st.Position
		hi := bars[i].High
		lo := bars[i].Low
		c := bars[i].Close
		slHit := (pos.Direction == DirBuy && lo <= pos.Stop) || (pos.Direction == DirSell && hi >= pos.Stop)
		if slHit {
			return Action{
				Kind:       ActionExitStop,
				Direction:  pos.Direction,
				Price:      pos.Stop,
				Stop:       pos.Stop,
				Risk:       pos.RiskDistance,
				SignalTime: pos.SignalTime,
				SignalIdx:  i,
				FillIdx:    i,
				FillTime:   bars[i].Time,
			}
		}
		from := i - cfg.ExitM
		if from < 0 {
			from = 0
		}
		upperM, lowerM := channelMaxMin(bars, from, i)
		ch := (pos.Direction == DirBuy && c < lowerM) || (pos.Direction == DirSell && c > upperM)
		if ch {
			return Action{
				Kind:       ActionExitChannel,
				Direction:  pos.Direction,
				Price:      nextOpen,
				Stop:       pos.Stop,
				Risk:       pos.RiskDistance,
				SignalTime: pos.SignalTime,
				SignalIdx:  i,
				FillIdx:    i + 1,
				FillTime:   nextTime,
			}
		}
		return Action{Kind: ActionNone}
	}

	warm := Warmup(cfg)
	if i < warm {
		return Action{Kind: ActionNone}
	}
	atrI := 0.0
	if i < len(atr) {
		atrI = atr[i]
	}
	if atrI <= 0 {
		return Action{Kind: ActionNone}
	}
	from := i - cfg.ChannelN
	if from < 0 {
		from = 0
	}
	upperN, lowerN := channelMaxMin(bars, from, i)
	c := bars[i].Close
	if !resumeReady(st, cfg, i, upperN, lowerN, c, atrI) {
		return Action{Kind: ActionNone}
	}
	var want Direction
	switch {
	case c > upperN:
		want = DirBuy
	case c < lowerN:
		want = DirSell
	default:
		return Action{Kind: ActionNone}
	}
	var stop, risk float64
	if want == DirBuy {
		stop = nextOpen - cfg.ATRStopMult*atrI
		risk = nextOpen - stop
	} else {
		stop = nextOpen + cfg.ATRStopMult*atrI
		risk = stop - nextOpen
	}
	if risk <= 0 {
		return Action{Kind: ActionNone}
	}
	return Action{
		Kind:       ActionEnter,
		Direction:  want,
		Price:      nextOpen,
		Stop:       stop,
		Risk:       risk,
		SignalTime: bars[i].Time,
		SignalIdx:  i,
		FillIdx:    i + 1,
		FillTime:   nextTime,
	}
}

func Apply(st *State, cfg Config, bars []Bar, act Action) *Trade {
	switch act.Kind {
	case ActionEnter:
		qty := Quantity(st.Equity, cfg.RiskPct, cfg.MaxRiskUSD, act.Risk)
		st.Position = &Position{
			ID:           st.NextID,
			Direction:    act.Direction,
			EntryPrice:   act.Price,
			EntryIdx:     act.FillIdx,
			Stop:         act.Stop,
			RiskDistance: act.Risk,
			SignalTime:   act.SignalTime,
			Quantity:     qty,
		}
		st.NextID++
		return nil
	case ActionExitStop, ActionExitChannel:
		if st.Position == nil {
			return nil
		}
		pos := st.Position
		outcome := OutcomeSL
		if act.Kind == ActionExitChannel {
			outcome = OutcomeTime
		}
		riskUSD := RiskUSD(st.Equity, cfg.RiskPct, cfg.MaxRiskUSD)
		// Use the risk dollars frozen at entry (qty * risk distance), matching
		// live sizing. Replay also uses current equity at entry time.
		if pos.Quantity > 0 && pos.RiskDistance > 0 {
			riskUSD = pos.Quantity * pos.RiskDistance
		}
		gross, entryFee, exitFee, net, newEq := ApplyFillCosts(
			st.Equity, pos.Quantity, pos.EntryPrice, act.Price, pos.Direction, outcome, riskUSD, cfg.FeeRate,
		)
		t := &Trade{
			ID:           pos.ID,
			Direction:    pos.Direction,
			EntryTime:    0,
			EntryPrice:   pos.EntryPrice,
			StopLoss:     pos.Stop,
			ExitTime:     act.FillTime,
			ExitPrice:    act.Price,
			Outcome:      outcome,
			RiskDistance: pos.RiskDistance,
			SignalTime:   pos.SignalTime,
			Quantity:     pos.Quantity,
			Gross:        gross,
			EntryFee:     entryFee,
			ExitFee:      exitFee,
			Net:          net,
		}
		if pos.EntryIdx >= 0 && pos.EntryIdx < len(bars) {
			t.EntryTime = bars[pos.EntryIdx].Time
		}
		onTradeClose(st, cfg, pos.Direction, pos.EntryPrice, act.Price, act.FillIdx)
		st.Equity = newEq
		st.Position = nil
		return t
	}
	return nil
}

// Replay is a 1:1 port of strategies/donchian/trades.py build_donchian_trades
// plus research fee/sizing accounting. evalFrom/evalTo filter by entry_time
// (Unix seconds); pass 0, math.MaxInt64 to keep all trades.
func Replay(bars []Bar, cfg Config, startEquity float64, evalFrom, evalTo int64) (trades []Trade, final State) {
	st := NewState(startEquity)
	if evalTo == 0 {
		evalTo = math.MaxInt64
	}
	atr := ATR(bars, cfg.ATRPeriod)
	n := len(bars)
	warm := Warmup(cfg)
	i := warm
	for i < n-1 {
		nextOpen := bars[i+1].Open
		nextTime := bars[i+1].Time
		if st.Position != nil {
			act := Decide(bars, atr, i, nextOpen, nextTime, st, cfg)
			if act.Kind == ActionExitStop || act.Kind == ActionExitChannel {
				if t := Apply(&st, cfg, bars, act); t != nil {
					trades = append(trades, *t)
				}
				i++
				continue
			}
		}
		if st.Position == nil {
			act := Decide(bars, atr, i, nextOpen, nextTime, st, cfg)
			if act.Kind == ActionEnter {
				if evalFrom <= nextTime && nextTime < evalTo {
					Apply(&st, cfg, bars, act)
					i = st.Position.EntryIdx
					continue
				}
			}
		}
		i++
	}
	if st.Position != nil {
		entryIdx := st.Position.EntryIdx
		endIdx := n - 1
		for j := n - 1; j > entryIdx; j-- {
			if bars[j].Time < evalTo {
				endIdx = j
				break
			}
		}
		if endIdx <= entryIdx {
			trades = append(trades, Trade{
				ID:           st.Position.ID,
				Direction:    st.Position.Direction,
				EntryTime:    bars[entryIdx].Time,
				EntryPrice:   st.Position.EntryPrice,
				StopLoss:     st.Position.Stop,
				Outcome:      OutcomeOpen,
				RiskDistance: st.Position.RiskDistance,
				SignalTime:   st.Position.SignalTime,
				Quantity:     st.Position.Quantity,
			})
		} else {
			act := Action{
				Kind:       ActionExitChannel,
				Direction:  st.Position.Direction,
				Price:      bars[endIdx].Open,
				Stop:       st.Position.Stop,
				Risk:       st.Position.RiskDistance,
				SignalTime: st.Position.SignalTime,
				FillIdx:    endIdx,
				FillTime:   bars[endIdx].Time,
			}
			if t := Apply(&st, cfg, bars, act); t != nil {
				trades = append(trades, *t)
			}
		}
	}
	out := trades[:0]
	for _, t := range trades {
		if t.Outcome == OutcomeOpen || (evalFrom <= t.EntryTime && t.EntryTime < evalTo) {
			out = append(out, t)
		}
	}
	return out, st
}
