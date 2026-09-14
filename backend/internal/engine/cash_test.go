package engine

import "testing"

func TestResidualCashDeposit(t *testing.T) {
	// wallet +1000, no trading
	if g := ResidualCash(10000, 11000, 100, 100, 2, 2); g < 999 || g > 1001 {
		t.Fatalf("deposit %v", g)
	}
}

func TestResidualCashWithdraw(t *testing.T) {
	if g := ResidualCash(10000, 7000, 50, 50, 0, 0); g > -2999 || g < -3001 {
		t.Fatalf("withdraw %v", g)
	}
}

func TestResidualCashTradeClose(t *testing.T) {
	// after open wallet already paid $5 fee; close net +80 ⇒ wallet +85 (= net + entry fee)
	if g := ResidualCash(9995, 10080, 0, 80, 5, 0); abs(g) > 0.01 {
		t.Fatalf("close should be 0, got %v", g)
	}
}

func TestResidualCashNewEntryFee(t *testing.T) {
	// entry fee 4 hits wallet, tracked as open fee
	if g := ResidualCash(10000, 9996, 20, 20, 0, 4); abs(g) > 0.01 {
		t.Fatalf("entry fee should be 0, got %v", g)
	}
}

func TestResidualCashMarkMoveIgnored(t *testing.T) {
	// wallet unchanged when only uPnL moves
	if g := ResidualCash(9900, 9900, 10, 10, 0, 0); abs(g) > 0.01 {
		t.Fatalf("mark %v", g)
	}
}

func TestCashThreshold(t *testing.T) {
	if cashThreshold(5, 10000) != 5 {
		t.Fatal("small equity uses min")
	}
	if cashThreshold(5, 200_000) < 99 {
		t.Fatal("large equity uses 5bps floor")
	}
}

func abs(x float64) float64 {
	if x < 0 {
		return -x
	}
	return x
}
