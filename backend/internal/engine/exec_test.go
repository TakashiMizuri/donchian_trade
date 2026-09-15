package engine

import "testing"

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
