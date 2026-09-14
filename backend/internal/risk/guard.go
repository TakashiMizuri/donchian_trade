package risk

import (
	"sync"
	"time"

	"donchian.trade/bot/internal/config"
)

type Guard struct {
	mu           sync.Mutex
	kill         bool
	maxNotional  float64
	dailyLimit   float64
	maxLeverage  float64
	dailyPnL     float64
	dailyDate    string
}

func New(cfg *config.Config, kill bool, dailyPnL float64, dailyDate string) *Guard {
	return &Guard{
		kill:        kill || cfg.KillSwitch,
		maxNotional: cfg.MaxNotionalUSD,
		dailyLimit:  cfg.DailyLossLimitUSD,
		maxLeverage: cfg.MaxLeverage,
		dailyPnL:    dailyPnL,
		dailyDate:   dailyDate,
	}
}

func todayUTC() string { return time.Now().UTC().Format("2006-01-02") }

func (g *Guard) snapshotDay() {
	d := todayUTC()
	if g.dailyDate != d {
		g.dailyDate = d
		g.dailyPnL = 0
	}
}

func (g *Guard) Kill() bool {
	g.mu.Lock()
	defer g.mu.Unlock()
	return g.kill
}

func (g *Guard) SetKill(v bool) {
	g.mu.Lock()
	g.kill = v
	g.mu.Unlock()
}

func (g *Guard) AddRealized(net float64) {
	g.mu.Lock()
	defer g.mu.Unlock()
	g.snapshotDay()
	g.dailyPnL += net
}

func (g *Guard) DailyPnL() (float64, string) {
	g.mu.Lock()
	defer g.mu.Unlock()
	g.snapshotDay()
	return g.dailyPnL, g.dailyDate
}

func (g *Guard) AllowEntry(notional, equity float64) (bool, string) {
	g.mu.Lock()
	defer g.mu.Unlock()
	g.snapshotDay()
	if g.kill {
		return false, "kill-switch is on"
	}
	if g.dailyLimit > 0 && g.dailyPnL <= -g.dailyLimit {
		return false, "daily loss limit reached"
	}
	if g.maxNotional > 0 && notional > g.maxNotional {
		return false, "max notional exceeded"
	}
	if g.maxLeverage > 0 && equity > 0 && notional/equity > g.maxLeverage {
		return false, "max leverage exceeded"
	}
	return true, ""
}
