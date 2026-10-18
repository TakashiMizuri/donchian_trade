package engine

import (
	"context"
	"log/slog"
	"testing"

	"donchian.trade/bot/internal/config"
)

func TestLivePnLUsesActualFillNotMinusR(t *testing.T) {
	// BUY 2 BTC, entry 100, exit 99 → gross −2, not forced −riskUSD
	gross, net := livePnL("BUY", 2, 100, 99, 0.1, 0.1, -0.5)
	if gross < -2.01 || gross > -1.99 {
		t.Fatalf("gross %v", gross)
	}
	// net = -2 - 0.1 - 0.1 + (-0.5) = -2.7
	if net < -2.71 || net > -2.69 {
		t.Fatalf("net %v", net)
	}
}

func TestLivePnLShort(t *testing.T) {
	gross, _ := livePnL("SELL", 1, 100, 90, 0, 0, 0)
	if gross < 9.99 || gross > 10.01 {
		t.Fatalf("short gross %v", gross)
	}
}

func TestFundingPnLSign(t *testing.T) {
	// paid_out rose by 10 → we paid → PnL −10
	if g := fundingPnL(0, 10); g != -10 {
		t.Fatalf("paid %v", g)
	}
	// paid_out fell (negative) → we received
	if g := fundingPnL(0, -4); g != 4 {
		t.Fatalf("received %v", g)
	}
}

func TestLiveDoesNotInventBinanceFee(t *testing.T) {
	gross, net := livePnL("BUY", 2, 100, 101, 0, 0, 0)
	if gross < 1.99 || gross > 2.01 {
		t.Fatalf("gross %v", gross)
	}
	if net != gross {
		t.Fatalf("net %v should equal gross when Lighter charged $0", net)
	}
}

// Regression: booking exit≈entry (open positionAvg) made ghost ~0 PnL closes
// while the exchange was still in the trade.
func TestGhostExitNearEntryLooksLikeZeroPnL(t *testing.T) {
	gross, net := livePnL("BUY", 0.4128, 3890.5, 3890.5, 0, 0, 0)
	if gross != 0 || net != 0 {
		t.Fatalf("avg-as-exit ghost PnL gross=%v net=%v", gross, net)
	}
}

func TestResolveLiveExitFillUsesLastWhenExchangeFlat(t *testing.T) {
	e := &Engine{
		Cfg:       &config.Config{DryRun: true},
		Log:       slog.Default(),
		lastPrice: map[string]float64{"ETH": 2501.25},
	}
	px, fee := e.resolveLiveExitFill(context.Background(), "ETH", 0, "", 2490)
	if px != 2501.25 || fee != 0 {
		t.Fatalf("got px=%v fee=%v", px, fee)
	}
}

func TestResolveLiveExitFillShadowWhenNoLast(t *testing.T) {
	e := &Engine{
		Cfg:       &config.Config{DryRun: true},
		Log:       slog.Default(),
		lastPrice: map[string]float64{},
	}
	px, _ := e.resolveLiveExitFill(context.Background(), "ETH", 0, "", 2490)
	if px != 2490 {
		t.Fatalf("got px=%v want shadow", px)
	}
}

func TestResolveLiveExitFillUnresolvedWhenNoPrices(t *testing.T) {
	e := &Engine{
		Cfg:       &config.Config{DryRun: true},
		Log:       slog.Default(),
		lastPrice: map[string]float64{},
	}
	px, fee := e.resolveLiveExitFill(context.Background(), "ETH", 0, "", 0)
	if px != 0 || fee != 0 {
		t.Fatalf("got px=%v fee=%v want unresolved", px, fee)
	}
}
