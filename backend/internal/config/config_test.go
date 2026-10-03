package config

import (
	"strings"
	"testing"
	"time"

	"donchian.trade/bot/internal/strategy"
)

func loadWithPass(t *testing.T) *Config {
	t.Helper()
	t.Setenv("DASHBOARD_PASSWORD", "test-pass")
	t.Setenv("LIGHTER_NETWORK", "testnet")
	cfg, err := Load()
	if err != nil {
		t.Fatalf("Load: %v", err)
	}
	return cfg
}

func TestDefaultEnvIs15mBrkVol(t *testing.T) {
	cfg := loadWithPass(t)
	if cfg.Resolution != "15m" || cfg.Timeframe != 15*time.Minute || cfg.BarSeconds != 900 {
		t.Fatalf("tf %+v %s %d", cfg.Timeframe, cfg.Resolution, cfg.BarSeconds)
	}
	if cfg.ChannelN != 120 || cfg.ExitM != 60 || cfg.ATRPeriod != 80 {
		t.Fatalf("channel n=%d m=%d atr=%d", cfg.ChannelN, cfg.ExitM, cfg.ATRPeriod)
	}
	if cfg.LiveProfile != "brk0.5+vol_rank" || cfg.MinBreakoutATR != 0.5 || cfg.MaxVolRank != 0.64 {
		t.Fatalf("profile %q brk=%v vol=%v", cfg.LiveProfile, cfg.MinBreakoutATR, cfg.MaxVolRank)
	}
	if !cfg.ShadowBrkVol || !cfg.ShadowBaseline {
		t.Fatalf("shadows twin=%v base=%v", cfg.ShadowBrkVol, cfg.ShadowBaseline)
	}
	if cfg.LogDir == "" {
		t.Fatal("log dir empty")
	}
	live := cfg.LiveConfig()
	base := cfg.BaselineConfig()
	if live.MinBreakoutATR != 0.5 || base.MinBreakoutATR != 0 {
		t.Fatalf("filters live=%+v base=%+v", live, base)
	}
	if live.MaxBarsInTrade != 0 || base.MaxBarsInTrade != 0 {
		t.Fatalf("default tstop should be off, live=%d base=%d", live.MaxBarsInTrade, base.MaxBarsInTrade)
	}
}

func TestFiveMTstop20hPreset(t *testing.T) {
	t.Setenv("DONCHIAN_TIMEFRAME", "5m")
	t.Setenv("DONCHIAN_BAR_SECONDS", "300")
	t.Setenv("DONCHIAN_CHANNEL_N", "360")
	t.Setenv("DONCHIAN_EXIT_M", "180")
	t.Setenv("DONCHIAN_ATR_PERIOD", "240")
	t.Setenv("DONCHIAN_LIVE_PROFILE", "brk0.5+vol_rank")
	t.Setenv("DONCHIAN_MIN_BREAKOUT_ATR", "0.5")
	t.Setenv("DONCHIAN_MAX_VOL_RANK", "0.64")
	t.Setenv("DONCHIAN_VOL_RANK_BARS", "240")
	t.Setenv("DONCHIAN_VOL_RANK_LOOKBACK", "12000")
	t.Setenv("DONCHIAN_MAX_BARS_IN_TRADE", "240")
	t.Setenv("DONCHIAN_SHADOW_MAX_BARS_IN_TRADE", "0")
	cfg := loadWithPass(t)
	if cfg.Resolution != "5m" || cfg.BarSeconds != 300 {
		t.Fatalf("tf %s %d", cfg.Resolution, cfg.BarSeconds)
	}
	if cfg.ChannelN != 360 || cfg.ExitM != 180 || cfg.ATRPeriod != 240 {
		t.Fatalf("n/m/atr %d/%d/%d", cfg.ChannelN, cfg.ExitM, cfg.ATRPeriod)
	}
	if cfg.VolRankBars != 240 || cfg.VolRankLookback != 12000 {
		t.Fatalf("vol windows %d/%d", cfg.VolRankBars, cfg.VolRankLookback)
	}
	live := cfg.LiveConfig()
	twin := cfg.ShadowTwinConfig()
	if live.MaxBarsInTrade != 240 || twin.MaxBarsInTrade != 0 {
		t.Fatalf("tstop live=%d twin=%d", live.MaxBarsInTrade, twin.MaxBarsInTrade)
	}
	if !strings.Contains(cfg.Tag, "tstop20h") && !strings.Contains(cfg.Fingerprint(), "tstop=240") {
		t.Fatalf("tag/fp missing tstop: tag=%s fp=%s", cfg.Tag, cfg.Fingerprint())
	}
}

func TestOneHourBaselinePreset(t *testing.T) {
	t.Setenv("DONCHIAN_TIMEFRAME", "1h")
	t.Setenv("DONCHIAN_CHANNEL_N", "30")
	t.Setenv("DONCHIAN_EXIT_M", "15")
	t.Setenv("DONCHIAN_ATR_PERIOD", "20")
	t.Setenv("DONCHIAN_LIVE_PROFILE", "baseline")
	t.Setenv("DONCHIAN_MAX_RISK_USD", "1000")
	t.Setenv("DONCHIAN_MIN_BREAKOUT_ATR", "0")
	t.Setenv("DONCHIAN_MAX_VOL_RANK", "0")
	cfg := loadWithPass(t)
	got := cfg.LiveConfig()
	want := strategy.DefaultConfig()
	if got.ChannelN != want.ChannelN || got.ExitM != want.ExitM || got.ATRPeriod != want.ATRPeriod {
		t.Fatalf("n/m/atr got %+v want %+v", got, want)
	}
	if cfg.ShadowBrkVol {
		t.Fatal("baseline should not auto twin")
	}
}

func TestBarSecondsMustMatchTimeframe(t *testing.T) {
	t.Setenv("DASHBOARD_PASSWORD", "x")
	t.Setenv("DONCHIAN_TIMEFRAME", "15m")
	t.Setenv("DONCHIAN_BAR_SECONDS", "1800")
	if _, err := Load(); err == nil {
		t.Fatal("expected BAR_SECONDS mismatch")
	}
}

func TestExitMMustBeBelowN(t *testing.T) {
	t.Setenv("DASHBOARD_PASSWORD", "x")
	t.Setenv("DONCHIAN_CHANNEL_N", "30")
	t.Setenv("DONCHIAN_EXIT_M", "30")
	if _, err := Load(); err == nil {
		t.Fatal("expected M < N")
	}
}

func TestRejectUnknownTimeframe(t *testing.T) {
	t.Setenv("DASHBOARD_PASSWORD", "x")
	t.Setenv("DONCHIAN_TIMEFRAME", "2h")
	if _, err := Load(); err == nil {
		t.Fatal("expected reject 2h")
	}
}

func TestRejectLS5Profile(t *testing.T) {
	t.Setenv("DASHBOARD_PASSWORD", "x")
	t.Setenv("DONCHIAN_LIVE_PROFILE", "ls5_cond_brk2.0")
	if _, err := Load(); err == nil {
		t.Fatal("ls5 profile should be rejected")
	}
}
