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
	defaultTF          = "15m"
	defaultChannelN    = 120
	defaultExitM       = 60
	defaultATRPeriod   = 80
	defaultATRStopMult = 1.5
	defaultRiskPct     = 1.0
	defaultMaxRiskUSD  = 625.0
	defaultLiveProfile = "brk0.5+vol_rank"
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

	defVolBars := (20 * 3600) / wantSec
	defVolLookback := (42 * 24 * 3600) / wantSec

	profile := strings.ToLower(strings.TrimSpace(getenv("DONCHIAN_LIVE_PROFILE", defaultLiveProfile)))
	profile = strings.ReplaceAll(profile, " ", "")
	minBrk := getenvFloat("DONCHIAN_MIN_BREAKOUT_ATR", -1)
	maxVR := getenvFloat("DONCHIAN_MAX_VOL_RANK", -1)

	switch profile {
	case "baseline":
		if minBrk < 0 {
			minBrk = 0
		}
		if maxVR < 0 {
			maxVR = 0
		}
	case "brk0.5":
		if minBrk < 0 {
			minBrk = 0.5
		}
		if maxVR < 0 {
			maxVR = 0
		}
	case "vol_rank", "vol_rank0.64":
		profile = "vol_rank"
		if minBrk < 0 {
			minBrk = 0
		}
		if maxVR < 0 {
			maxVR = 0.64
		}
	case "brk0.5+vol_rank", "brk0.5_vol_rank", "brk_vol":
		profile = "brk0.5+vol_rank"
		if minBrk < 0 {
			minBrk = 0.5
		}
		if maxVR < 0 {
			maxVR = 0.64
		}
	default:
		return fmt.Errorf("DONCHIAN_LIVE_PROFILE %q (use brk0.5+vol_rank, brk0.5, vol_rank, baseline)", profile)
	}
	if minBrk < 0 {
		minBrk = 0
	}
	if maxVR < 0 {
		maxVR = 0
	}
	if maxVR > 1 {
		return fmt.Errorf("DONCHIAN_MAX_VOL_RANK must be in [0,1], got %g", maxVR)
	}

	volBars := getenvInt("DONCHIAN_VOL_RANK_BARS", defVolBars)
	volLookback := getenvInt("DONCHIAN_VOL_RANK_LOOKBACK", defVolLookback)
	if maxVR > 0 {
		if volBars < 2 || volLookback < 2 {
			return fmt.Errorf("DONCHIAN_VOL_RANK_BARS / LOOKBACK must be ≥ 2 when MAX_VOL_RANK > 0")
		}
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
	cfg.MinBreakoutATR = minBrk
	cfg.MaxVolRank = maxVR
	cfg.VolRankBars = volBars
	cfg.VolRankLookback = volLookback

	maxBars := getenvInt("DONCHIAN_MAX_BARS_IN_TRADE", 0)
	if maxBars < 0 {
		return fmt.Errorf("DONCHIAN_MAX_BARS_IN_TRADE must be ≥ 0")
	}
	shadowMaxBars := getenvInt("DONCHIAN_SHADOW_MAX_BARS_IN_TRADE", 0)
	if shadowMaxBars < 0 {
		return fmt.Errorf("DONCHIAN_SHADOW_MAX_BARS_IN_TRADE must be ≥ 0")
	}
	cfg.MaxBarsInTrade = maxBars
	cfg.ShadowMaxBarsInTrade = shadowMaxBars

	if raw := strings.TrimSpace(os.Getenv("DONCHIAN_SHADOW_BRK_VOL")); raw != "" {
		cfg.ShadowBrkVol = getenvBool("DONCHIAN_SHADOW_BRK_VOL", false)
	} else {
		cfg.ShadowBrkVol = profile == "brk0.5+vol_rank" || profile == "brk0.5" || profile == "vol_rank"
	}

	cfg.Tag = strings.TrimSpace(getenv("DONCHIAN_TAG", autoTag(cfg)))
	if cfg.Tag == "" {
		cfg.Tag = autoTag(cfg)
	}
	return nil
}

func autoTag(cfg *Config) string {
	tag := fmt.Sprintf("%s_N%d_M%d_ATR%d_R%g", cfg.Resolution, cfg.ChannelN, cfg.ExitM, cfg.ATRPeriod, cfg.RiskPct)
	switch {
	case cfg.MinBreakoutATR > 0 && cfg.MaxVolRank > 0:
		tag += "_brk0.5_vol0.64"
	case cfg.MinBreakoutATR > 0:
		tag += "_brk0.5"
	case cfg.MaxVolRank > 0:
		tag += "_vol0.64"
	default:
		tag += "+baseline"
	}
	if cfg.MaxBarsInTrade > 0 {
		hours := float64(cfg.MaxBarsInTrade) * float64(cfg.BarSeconds) / 3600
		if hours == float64(int(hours)) {
			tag += fmt.Sprintf("_tstop%dh", int(hours))
		} else {
			tag += fmt.Sprintf("_tstop%db", cfg.MaxBarsInTrade)
		}
	}
	return tag
}

func (c *Config) coreStrategy() strategy.Config {
	return strategy.Config{
		ChannelN:        c.ChannelN,
		ExitM:           c.ExitM,
		ATRPeriod:       c.ATRPeriod,
		ATRStopMult:     c.ATRStopMult,
		RiskPct:         c.RiskPct,
		MaxRiskUSD:      c.MaxRiskUSD,
		FeeRate:         c.FeeRate,
		MinBreakoutATR:  c.MinBreakoutATR,
		MaxVolRank:      c.MaxVolRank,
		VolRankBars:     c.VolRankBars,
		VolRankLookback: c.VolRankLookback,
	}
}

func (c *Config) LiveConfig() strategy.Config {
	cfg := c.coreStrategy()
	cfg.MaxBarsInTrade = c.MaxBarsInTrade
	return cfg
}

// ShadowTwinConfig matches live entry filters (stored under profile shadow_ls5).
// Time-stop is independent (usually 0 so twin matches research fills without tstop).
func (c *Config) ShadowTwinConfig() strategy.Config {
	cfg := c.coreStrategy()
	cfg.MaxBarsInTrade = c.ShadowMaxBarsInTrade
	return cfg
}

func (c *Config) BaselineConfig() strategy.Config {
	cfg := c.coreStrategy()
	cfg.MinBreakoutATR = 0
	cfg.MaxVolRank = 0
	cfg.MaxBarsInTrade = 0
	return cfg
}

func (c *Config) Fingerprint() string {
	return fmt.Sprintf("tf=%s n=%d m=%d atr=%d x%.4g risk=%.4g cap=%.4g fee=%.6g brk=%.4g vol=%.4g vb=%d vl=%d tstop=%d ststop=%d",
		c.Resolution, c.ChannelN, c.ExitM, c.ATRPeriod, c.ATRStopMult, c.RiskPct, c.MaxRiskUSD, c.FeeRate,
		c.MinBreakoutATR, c.MaxVolRank, c.VolRankBars, c.VolRankLookback, c.MaxBarsInTrade, c.ShadowMaxBarsInTrade)
}

func (c *Config) ProfileLog() []any {
	return []any{
		"instance_id", c.InstanceID,
		"instance_name", c.InstanceName,
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
		"min_breakout_atr", c.MinBreakoutATR,
		"max_vol_rank", c.MaxVolRank,
		"vol_rank_bars", c.VolRankBars,
		"vol_rank_lookback", c.VolRankLookback,
		"tstop_bars", c.MaxBarsInTrade,
		"shadow_tstop_bars", c.ShadowMaxBarsInTrade,
		"shadow_twin", c.ShadowBrkVol,
		"shadow_baseline", c.ShadowBaseline,
		"log_dir", c.LogDir,
		"symbols", strings.Join(c.Symbols, ","),
	}
}
