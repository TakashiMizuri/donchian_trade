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
	"1m":  time.Minute,
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
		return fmt.Errorf("DONCHIAN_TIMEFRAME %q not in 1h,30m,15m,5m,1m", tf)
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
	case "core", "brk0", "brk0_voloff":
		profile = "core"
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
		return fmt.Errorf("DONCHIAN_LIVE_PROFILE %q (use core, brk0.5+vol_rank, brk0.5, vol_rank, baseline)", profile)
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
	if profile == "core" && !envSet("DONCHIAN_SHADOW_MAX_BARS_IN_TRADE") {
		cfg.ShadowMaxBarsInTrade = maxBars
	}

	if raw := strings.TrimSpace(os.Getenv("DONCHIAN_SHADOW_BRK_VOL")); raw != "" {
		cfg.ShadowBrkVol = getenvBool("DONCHIAN_SHADOW_BRK_VOL", false)
	} else {
		cfg.ShadowBrkVol = profile == "core" || profile == "brk0.5+vol_rank" || profile == "brk0.5" || profile == "vol_rank"
	}

	onBars := getenvInt("DONCHIAN_LATE_TRAIL_ON_BARS", 0)
	trailR := getenvFloat("DONCHIAN_LATE_TRAIL_R", 0)
	trailMFE := getenvFloat("DONCHIAN_LATE_TRAIL_MIN_MFE", 0)
	if onBars < 0 || trailR < 0 || trailMFE < 0 {
		return fmt.Errorf("DONCHIAN_LATE_TRAIL_* must be ≥ 0")
	}
	if onBars > 0 {
		if trailR <= 0 || trailMFE <= 0 {
			return fmt.Errorf("DONCHIAN_LATE_TRAIL_R and MIN_MFE must be > 0 when ON_BARS > 0")
		}
		if maxBars > 0 && onBars >= maxBars {
			return fmt.Errorf("DONCHIAN_LATE_TRAIL_ON_BARS (%d) must be < MAX_BARS_IN_TRADE (%d)", onBars, maxBars)
		}
	}
	cfg.LateTrailOnBars = onBars
	cfg.LateTrailR = trailR
	cfg.LateTrailMinMFE = trailMFE

	// Twin defaults to the live book (ideal paper fills). SHADOW_* overrides only.
	cfg.ShadowMinBreakoutATR = cfg.MinBreakoutATR
	cfg.ShadowMaxVolRank = cfg.MaxVolRank
	cfg.ShadowLateTrailOnBars = onBars
	cfg.ShadowLateTrailR = trailR
	cfg.ShadowLateTrailMinMFE = trailMFE
	if envSet("DONCHIAN_SHADOW_MIN_BREAKOUT_ATR") {
		cfg.ShadowMinBreakoutATR = getenvFloat("DONCHIAN_SHADOW_MIN_BREAKOUT_ATR", cfg.ShadowMinBreakoutATR)
	}
	if envSet("DONCHIAN_SHADOW_MAX_VOL_RANK") {
		cfg.ShadowMaxVolRank = getenvFloat("DONCHIAN_SHADOW_MAX_VOL_RANK", cfg.ShadowMaxVolRank)
	}
	if envSet("DONCHIAN_SHADOW_LATE_TRAIL_ON_BARS") {
		cfg.ShadowLateTrailOnBars = getenvInt("DONCHIAN_SHADOW_LATE_TRAIL_ON_BARS", cfg.ShadowLateTrailOnBars)
	}
	if envSet("DONCHIAN_SHADOW_LATE_TRAIL_R") {
		cfg.ShadowLateTrailR = getenvFloat("DONCHIAN_SHADOW_LATE_TRAIL_R", cfg.ShadowLateTrailR)
	}
	if envSet("DONCHIAN_SHADOW_LATE_TRAIL_MIN_MFE") {
		cfg.ShadowLateTrailMinMFE = getenvFloat("DONCHIAN_SHADOW_LATE_TRAIL_MIN_MFE", cfg.ShadowLateTrailMinMFE)
	}
	if cfg.ShadowMaxVolRank < 0 || cfg.ShadowMaxVolRank > 1 {
		return fmt.Errorf("DONCHIAN_SHADOW_MAX_VOL_RANK must be in [0,1], got %g", cfg.ShadowMaxVolRank)
	}
	if cfg.ShadowLateTrailOnBars < 0 || cfg.ShadowLateTrailR < 0 || cfg.ShadowLateTrailMinMFE < 0 {
		return fmt.Errorf("DONCHIAN_SHADOW_LATE_TRAIL_* must be ≥ 0")
	}

	warnCoreProfile(cfg)

	cfg.Tag = strings.TrimSpace(getenv("DONCHIAN_TAG", autoTag(cfg)))
	if cfg.Tag == "" {
		cfg.Tag = autoTag(cfg)
	}
	if cfg.Resolution == "1m" && cfg.LiveProfile == "core" && !strings.Contains(strings.ToLower(cfg.Tag), "satellite") {
		cfg.StrategyWarnings = append(cfg.StrategyWarnings, "1m core tag should contain satellite")
	}
	return nil
}

func warnCoreProfile(cfg *Config) {
	if cfg.LiveProfile != "core" {
		return
	}
	if cfg.MinBreakoutATR != 0 || cfg.MaxVolRank != 0 {
		cfg.StrategyWarnings = append(cfg.StrategyWarnings,
			fmt.Sprintf("core profile expects MIN_BREAKOUT_ATR=0 and MAX_VOL_RANK=0, got %g / %g", cfg.MinBreakoutATR, cfg.MaxVolRank))
	}
	switch cfg.Resolution {
	case "5m":
		if cfg.LateTrailOnBars != 216 || cfg.LateTrailR != 2 || cfg.LateTrailMinMFE != 6 {
			cfg.StrategyWarnings = append(cfg.StrategyWarnings,
				fmt.Sprintf("5m R-026 expects late trail 216/2/6, got %d/%g/%g", cfg.LateTrailOnBars, cfg.LateTrailR, cfg.LateTrailMinMFE))
		}
		if cfg.LateTrailR == 3 {
			cfg.StrategyWarnings = append(cfg.StrategyWarnings, "LATE_TRAIL_R=3 is R-024, not R-026 live")
		}
		if cfg.LateTrailOnBars == 720 {
			cfg.StrategyWarnings = append(cfg.StrategyWarnings, "LATE_TRAIL_ON_BARS=720 is the 1m clock, not 5m")
		}
	case "1m":
		if cfg.LateTrailOnBars != 720 || cfg.LateTrailR != 6 || cfg.LateTrailMinMFE != 15 {
			cfg.StrategyWarnings = append(cfg.StrategyWarnings,
				fmt.Sprintf("1m R-025 expects late trail 720/6/15, got %d/%g/%g", cfg.LateTrailOnBars, cfg.LateTrailR, cfg.LateTrailMinMFE))
		}
		if cfg.LateTrailR == 8 {
			cfg.StrategyWarnings = append(cfg.StrategyWarnings, "LATE_TRAIL_R=8 is R-023, not R-025 live")
		}
	}
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
		if cfg.LateTrailOnBars > 0 {
			tag += "_brk0_voloff"
		} else {
			tag += "+baseline"
		}
	}
	if cfg.MaxBarsInTrade > 0 {
		hours := float64(cfg.MaxBarsInTrade) * float64(cfg.BarSeconds) / 3600
		if hours == float64(int(hours)) {
			tag += fmt.Sprintf("_tstop%dh", int(hours))
		} else {
			tag += fmt.Sprintf("_tstop%db", cfg.MaxBarsInTrade)
		}
	}
	if cfg.LateTrailOnBars > 0 {
		hours := float64(cfg.LateTrailOnBars) * float64(cfg.BarSeconds) / 3600
		if hours == float64(int(hours)) {
			tag += fmt.Sprintf("_late%dh_%gR", int(hours), cfg.LateTrailR)
		} else {
			tag += fmt.Sprintf("_late%db_%gR", cfg.LateTrailOnBars, cfg.LateTrailR)
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
		LateTrailOnBars: c.LateTrailOnBars,
		LateTrailR:      c.LateTrailR,
		LateTrailMinMFE: c.LateTrailMinMFE,
	}
}

func (c *Config) LiveConfig() strategy.Config {
	cfg := c.coreStrategy()
	cfg.MaxBarsInTrade = c.MaxBarsInTrade
	return cfg
}

// ShadowTwinConfig is the paper twin of live (DB shadow_ls5): same knobs,
// research fills. Gap vs live is execution, not a second strategy.
func (c *Config) ShadowTwinConfig() strategy.Config {
	cfg := c.coreStrategy()
	cfg.MinBreakoutATR = c.ShadowMinBreakoutATR
	cfg.MaxVolRank = c.ShadowMaxVolRank
	cfg.MaxBarsInTrade = c.ShadowMaxBarsInTrade
	cfg.LateTrailOnBars = c.ShadowLateTrailOnBars
	cfg.LateTrailR = c.ShadowLateTrailR
	cfg.LateTrailMinMFE = c.ShadowLateTrailMinMFE
	return cfg
}

func (c *Config) BaselineConfig() strategy.Config {
	cfg := c.coreStrategy()
	cfg.MinBreakoutATR = 0
	cfg.MaxVolRank = 0
	cfg.MaxBarsInTrade = 0
	cfg.LateTrailOnBars = 0
	cfg.LateTrailR = 0
	cfg.LateTrailMinMFE = 0
	return cfg
}

func (c *Config) Fingerprint() string {
	return fmt.Sprintf("tf=%s n=%d m=%d atr=%d x%.4g risk=%.4g cap=%.4g fee=%.6g brk=%.4g vol=%.4g vb=%d vl=%d tstop=%d ststop=%d trail=%d/%.4g/%.4g strail=%d/%.4g/%.4g sbrk=%.4g svol=%.4g",
		c.Resolution, c.ChannelN, c.ExitM, c.ATRPeriod, c.ATRStopMult, c.RiskPct, c.MaxRiskUSD, c.FeeRate,
		c.MinBreakoutATR, c.MaxVolRank, c.VolRankBars, c.VolRankLookback, c.MaxBarsInTrade, c.ShadowMaxBarsInTrade,
		c.LateTrailOnBars, c.LateTrailR, c.LateTrailMinMFE,
		c.ShadowLateTrailOnBars, c.ShadowLateTrailR, c.ShadowLateTrailMinMFE,
		c.ShadowMinBreakoutATR, c.ShadowMaxVolRank)
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
		"late_trail_on", c.LateTrailOnBars,
		"late_trail_r", c.LateTrailR,
		"late_trail_min_mfe", c.LateTrailMinMFE,
		"shadow_brk", c.ShadowMinBreakoutATR,
		"shadow_vol", c.ShadowMaxVolRank,
		"shadow_trail_on", c.ShadowLateTrailOnBars,
		"shadow_trail_r", c.ShadowLateTrailR,
		"shadow_twin", c.ShadowBrkVol,
		"shadow_baseline", c.ShadowBaseline,
		"log_dir", c.LogDir,
		"symbols", strings.Join(c.Symbols, ","),
	}
}
