package config

import (
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

func TestDefaultEnvIs30mCalendar(t *testing.T) {
	cfg := loadWithPass(t)
	if cfg.Resolution != "30m" || cfg.Timeframe != 30*time.Minute || cfg.BarSeconds != 1800 {
		t.Fatalf("tf %+v %s %d", cfg.Timeframe, cfg.Resolution, cfg.BarSeconds)
	}
	if cfg.ChannelN != 60 || cfg.ExitM != 30 || cfg.ATRPeriod != 40 || cfg.ATRStopMult != 1.5 {
		t.Fatalf("channel n=%d m=%d atr=%d x=%v", cfg.ChannelN, cfg.ExitM, cfg.ATRPeriod, cfg.ATRStopMult)
	}
	if cfg.RiskPct != 1 || cfg.MaxRiskUSD != 1000 {
		t.Fatalf("risk %v cap %v", cfg.RiskPct, cfg.MaxRiskUSD)
	}
	if !cfg.LS5Enabled || cfg.PauseBars != 48 || cfg.PauseHours != 24 {
		t.Fatalf("ls5 on=%v pause bars=%d hours=%v", cfg.LS5Enabled, cfg.PauseBars, cfg.PauseHours)
	}
	if cfg.FeeRate != 0 {
		t.Fatalf("fee %v", cfg.FeeRate)
	}
	if cfg.Tag == "" || cfg.LiveProfile != "ls5_cond_brk2.0" {
		t.Fatalf("tag %q profile %q", cfg.Tag, cfg.LiveProfile)
	}
}

func TestOneHourPresetMatchesResearchFixture(t *testing.T) {
	t.Setenv("DONCHIAN_TIMEFRAME", "1h")
	t.Setenv("DONCHIAN_CHANNEL_N", "30")
	t.Setenv("DONCHIAN_EXIT_M", "15")
	t.Setenv("DONCHIAN_ATR_PERIOD", "20")
	t.Setenv("DONCHIAN_ATR_STOP_MULT", "1.5")
	t.Setenv("DONCHIAN_RISK_PCT", "1")
	t.Setenv("DONCHIAN_MAX_RISK_USD", "1000")
	t.Setenv("DONCHIAN_LIVE_PROFILE", "ls5_cond_brk2.0")
	t.Setenv("DONCHIAN_LS5_PAUSE_HOURS", "24")
	cfg := loadWithPass(t)
	got := cfg.LiveConfig()
	want := strategy.DefaultConfig()
	if got.ChannelN != want.ChannelN || got.ExitM != want.ExitM || got.ATRPeriod != want.ATRPeriod {
		t.Fatalf("n/m/atr got %+v want %+v", got, want)
	}
	if got.LS5.PauseBars != 24 || cfg.Resolution != "1h" || cfg.Timeframe != time.Hour {
		t.Fatalf("1h pause/tf %+v %s", got.LS5, cfg.Resolution)
	}
	if got.FeeRate != 0 || got.MaxRiskUSD != 1000 || got.ATRStopMult != 1.5 {
		t.Fatalf("sizing %+v", got)
	}
}

func TestBarSecondsMustMatchTimeframe(t *testing.T) {
	t.Setenv("DASHBOARD_PASSWORD", "x")
	t.Setenv("DONCHIAN_TIMEFRAME", "30m")
	t.Setenv("DONCHIAN_BAR_SECONDS", "3600")
	if _, err := Load(); err == nil {
		t.Fatal("expected BAR_SECONDS mismatch")
	}
}

func TestPauseBarsMustEqualHours(t *testing.T) {
	t.Setenv("DASHBOARD_PASSWORD", "x")
	t.Setenv("DONCHIAN_TIMEFRAME", "30m")
	t.Setenv("DONCHIAN_LS5_PAUSE_HOURS", "24")
	t.Setenv("DONCHIAN_LS5_PAUSE_BARS", "24")
	if _, err := Load(); err == nil {
		t.Fatal("expected pause bars mismatch")
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

func TestBaselineDisablesLiveLS5NotShadow(t *testing.T) {
	t.Setenv("DONCHIAN_LIVE_PROFILE", "baseline")
	cfg := loadWithPass(t)
	if cfg.LS5Enabled || cfg.LiveConfig().LS5.Enabled {
		t.Fatal("live ls5 should be off")
	}
	if !cfg.ShadowLS5Config().LS5.Enabled || cfg.BaselineConfig().LS5.Enabled {
		t.Fatal("shadow books")
	}
	if cfg.PauseBars != 48 {
		t.Fatalf("pause still calendar 24h: %d", cfg.PauseBars)
	}
}

func TestRejectUnknownTimeframe(t *testing.T) {
	t.Setenv("DASHBOARD_PASSWORD", "x")
	t.Setenv("DONCHIAN_TIMEFRAME", "2h")
	if _, err := Load(); err == nil {
		t.Fatal("expected reject 2h")
	}
}
