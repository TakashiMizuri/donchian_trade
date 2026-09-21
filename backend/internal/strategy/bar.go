package strategy

// Bar is a closed candle of the configured timeframe. Time is Unix seconds of the bar open.
type Bar struct {
	Time   int64
	Open   float64
	High   float64
	Low    float64
	Close  float64
	Volume float64
}

type Direction string

const (
	DirBuy  Direction = "BUY"
	DirSell Direction = "SELL"
)

type Outcome string

const (
	OutcomeSL    Outcome = "sl"
	OutcomeTime  Outcome = "time" // channel exit (historical name)
	OutcomeOpen  Outcome = "open"
	OutcomeKill  Outcome = "kill"
	OutcomeWatch Outcome = "watchdog"
)

type Trade struct {
	ID           int
	Direction    Direction
	EntryTime    int64
	EntryPrice   float64
	StopLoss     float64
	ExitTime     int64
	ExitPrice    float64
	Outcome      Outcome
	RiskDistance float64
	SignalTime   int64
	Quantity     float64
	Gross        float64
	EntryFee     float64
	ExitFee      float64
	Net          float64
}

type Config struct {
	ChannelN    int
	ExitM       int
	ATRPeriod   int
	ATRStopMult float64
	RiskPct     float64
	MaxRiskUSD  float64
	FeeRate     float64
	LS5         LS5Config
}

type LS5Config struct {
	Enabled   bool
	StreakN   int
	PauseBars int
	ResumeATR float64
}

// DefaultConfig is the research 1h fixture used by parity tests.
// Live values come from DONCHIAN_* env (see config.applyStrategyEnv).
func DefaultConfig() Config {
	return Config{
		ChannelN:    30,
		ExitM:       15,
		ATRPeriod:   20,
		ATRStopMult: 1.5,
		RiskPct:     1.0,
		MaxRiskUSD:  1000,
		FeeRate:     0, // Lighter Standard: 0 maker / 0 taker. Not Binance 5 bps.
		LS5: LS5Config{
			Enabled:   true,
			StreakN:   5,
			PauseBars: 24,
			ResumeATR: 2.0,
		},
	}
}

func Warmup(cfg Config) int {
	w := cfg.ChannelN
	if cfg.ExitM > w {
		w = cfg.ExitM
	}
	if cfg.ATRPeriod > w {
		w = cfg.ATRPeriod
	}
	return w
}
