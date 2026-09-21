package config

import (
	"fmt"
	"os"
	"strconv"
	"strings"
	"time"

	"donchian.trade/bot/internal/strategy"
)

// Lighter candle resolutions we allow. Strings match REST `resolution` and WS `candle/{id}/{res}`.
var allowedTF = map[string]time.Duration{
	"1h":  time.Hour,
	"30m": 30 * time.Minute,
	"15m": 15 * time.Minute,
	"5m":  5 * time.Minute,
}

const (
	defaultTF          = "30m"
	defaultChannelN    = 60
	defaultExitM       = 30
	defaultATRPeriod   = 40
	defaultATRStopMult = 1.5
	defaultRiskPct     = 1.0
	defaultMaxRiskUSD  = 1000.0
	defaultPauseHours  = 24
	defaultLiveProfile = "ls5_cond_brk2.0"
	FingerprintKey     = "strategy_fingerprint"
)

func applyStrategyEnv(cfg *Config) error {
	tf := strings.ToLower(strings.TrimSpace(getenv("DONCHIAN_TIMEFRAME", defaultTF)))
	bar, ok := allowedTF[tf]
	if !ok {
		return fmt.Errorf("DONCHIAN_TIMEFRAME %q not in 1h,30m,15m,5m", tf)
	}
	wantSec := int(bar / time.Second)
	if raw := strings.TrimSpace(os.Getenv("DONCHIAN_BAR_SECONDS")); raw != "" {
		got, err := strconv.Atoi(raw)
		if err != nil || got != wantSec {
			return fmt.Errorf("DONCHIAN_BAR_SECONDS must be %d for %s, got %q", wantSec, tf, raw)
		}
	}

	method := strings.ToLower(strings.TrimSpace(getenv("DONCHIAN_ATR_METHOD", "sma_tr")))
	if method != "sma_tr" {
		return fmt.Errorf("DONCHIAN_ATR_METHOD must be sma_tr, got %q", method)
	}

	n := getenvInt("DONCHIAN_CHANNEL_N", defaultChannelN)
	m := getenvInt("DONCHIAN_EXIT_M", defaultExitM)
	atrN := getenvInt("DONCHIAN_ATR_PERIOD", defaultATRPeriod)
	mult := getenvFloat("DONCHIAN_ATR_STOP_MULT", defaultATRStopMult)
	risk := getenvFloat("DONCHIAN_RISK_PCT", defaultRiskPct)
	if n <= 0 || m <= 0 || atrN <= 0 {
		return fmt.Errorf("DONCHIAN_CHANNEL_N / EXIT_M / ATR_PERIOD must be > 0")
	}
	if m >= n {
		return fmt.Errorf("DONCHIAN_EXIT_M (%d) must be < DONCHIAN_CHANNEL_N (%d)", m, n)
	}
	if mult <= 0 || risk <= 0 {
		return fmt.Errorf("DONCHIAN_ATR_STOP_MULT and DONCHIAN_RISK_PCT must be > 0")
	}

	capOn := getenvBool("DONCHIAN_MAX_RISK_ENABLED", true)
	capUSD := getenvFloat("DONCHIAN_MAX_RISK_USD", defaultMaxRiskUSD)
	if !capOn || capUSD < 0 {
		capUSD = 0
	}

	profile := strings.ToLower(strings.TrimSpace(getenv("DONCHIAN_LIVE_PROFILE", defaultLiveProfile)))
	ls5On := true
	switch profile {
	case "ls5", "ls5_cond_brk2.0":
		profile = "ls5_cond_brk2.0"
		ls5On = true
	case "baseline":
		ls5On = false
	default:
		return fmt.Errorf("DONCHIAN_LIVE_PROFILE %q (use ls5_cond_brk2.0 or baseline)", profile)
	}

	pauseHours := getenvFloat("DONCHIAN_LS5_PAUSE_HOURS", defaultPauseHours)
	if pauseHours <= 0 {
		return fmt.Errorf("DONCHIAN_LS5_PAUSE_HOURS must be > 0")
	}
	derivedBars := int(pauseHours*3600) / wantSec
	if int(pauseHours*3600)%wantSec != 0 {
		return fmt.Errorf("DONCHIAN_LS5_PAUSE_HOURS=%.4g is not a whole number of %s bars", pauseHours, tf)
	}
	pauseBars := derivedBars
	if raw := strings.TrimSpace(os.Getenv("DONCHIAN_LS5_PAUSE_BARS")); raw != "" {
		got, err := strconv.Atoi(raw)
		if err != nil || got <= 0 {
			return fmt.Errorf("DONCHIAN_LS5_PAUSE_BARS must be a positive int, got %q", raw)
		}
		if got != derivedBars {
			return fmt.Errorf("DONCHIAN_LS5_PAUSE_BARS=%d but %g hours on %s is %d bars", got, pauseHours, tf, derivedBars)
		}
		pauseBars = got
	}

	streak := getenvInt("DONCHIAN_LS5_LOSS_STREAK_N", 5)
	resume := getenvFloat("DONCHIAN_LS5_RESUME_MIN_BREAKOUT_ATR", 2.0)
	if streak <= 0 || resume < 0 {
		return fmt.Errorf("ls5 streak / resume ATR invalid")
	}

	cfg.Resolution = tf
	cfg.Timeframe = bar
	cfg.BarSeconds = wantSec
	cfg.ChannelN = n
	cfg.ExitM = m
	cfg.ATRPeriod = atrN
	cfg.ATRStopMult = mult
	cfg.RiskPct = risk
	cfg.MaxRiskUSD = capUSD
	cfg.FeeRate = getenvFloat("DONCHIAN_FEE_RATE_PER_SIDE", 0)
	cfg.LiveProfile = profile
	cfg.LS5Enabled = ls5On
	cfg.LossStreakN = streak
	cfg.PauseHours = pauseHours
	cfg.PauseBars = pauseBars
	cfg.ResumeATR = resume
	cfg.Tag = strings.TrimSpace(getenv("DONCHIAN_TAG", autoTag(cfg)))
	if cfg.Tag == "" {
		cfg.Tag = autoTag(cfg)
	}
	return nil
}

func autoTag(cfg *Config) string {
	tag := fmt.Sprintf("%s_N%d_M%d_ATR%d_R%g", cfg.Resolution, cfg.ChannelN, cfg.ExitM, cfg.ATRPeriod, cfg.RiskPct)
	if cfg.LS5Enabled {
		return tag + "+ls5_cond_brk2.0"
	}
	return tag + "+baseline"
}

func (c *Config) coreStrategy() strategy.Config {
	return strategy.Config{
		ChannelN:    c.ChannelN,
		ExitM:       c.ExitM,
		ATRPeriod:   c.ATRPeriod,
		ATRStopMult: c.ATRStopMult,
		RiskPct:     c.RiskPct,
		MaxRiskUSD:  c.MaxRiskUSD,
		FeeRate:     c.FeeRate,
		LS5: strategy.LS5Config{
			Enabled:   c.LS5Enabled,
			StreakN:   c.LossStreakN,
			PauseBars: c.PauseBars,
			ResumeATR: c.ResumeATR,
		},
	}
}

func (c *Config) LiveConfig() strategy.Config {
	return c.coreStrategy()
}

func (c *Config) ShadowLS5Config() strategy.Config {
	cfg := c.coreStrategy()
	cfg.LS5.Enabled = true
	return cfg
}

func (c *Config) BaselineConfig() strategy.Config {
	cfg := c.coreStrategy()
	cfg.LS5.Enabled = false
	return cfg
}

// Fingerprint is stored in SQLite so a 30m process cannot eat a 1h journal.
func (c *Config) Fingerprint() string {
	return fmt.Sprintf("tf=%s n=%d m=%d atr=%d x%.4g risk=%.4g cap=%.4g fee=%.6g ls5=%t pause=%d resume=%.4g",
		c.Resolution, c.ChannelN, c.ExitM, c.ATRPeriod, c.ATRStopMult, c.RiskPct, c.MaxRiskUSD, c.FeeRate,
		c.LS5Enabled, c.PauseBars, c.ResumeATR)
}

func (c *Config) ProfileLog() []any {
	return []any{
		"tag", c.Tag,
		"tf", c.Resolution,
		"n", c.ChannelN,
		"m", c.ExitM,
		"atr", c.ATRPeriod,
		"stop_mult", c.ATRStopMult,
		"risk_pct", c.RiskPct,
		"max_risk_usd", c.MaxRiskUSD,
		"fee", c.FeeRate,
		"live_profile", c.LiveProfile,
		"ls5_pause_bars", c.PauseBars,
		"ls5_pause_hours", c.PauseHours,
		"resume_atr", c.ResumeATR,
		"symbols", strings.Join(c.Symbols, ","),
	}
}
