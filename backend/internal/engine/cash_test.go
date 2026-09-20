package engine

import "testing"

func TestResidualCashDeposit(t *testing.T) {
	// wallet +1000, no trading
	if g := ResidualCash(10000, 11000, 100, 100, 2, 2, 0, 0); g < 999 || g > 1001 {
		t.Fatalf("deposit %v", g)
	}
}

func TestResidualCashWithdraw(t *testing.T) {
	if g := ResidualCash(10000, 7000, 50, 50, 0, 0, 0, 0); g > -2999 || g < -3001 {
		t.Fatalf("withdraw %v", g)
	}
}

func TestResidualCashTradeClose(t *testing.T) {
	// after open wallet already paid $5 fee; close net +80 ⇒ wallet +85 (= net + entry fee)
	if g := ResidualCash(9995, 10080, 0, 80, 5, 0, 0, 0); abs(g) > 0.01 {
		t.Fatalf("close should be 0, got %v", g)
	}
}

func TestResidualCashNewEntryFee(t *testing.T) {
	// entry fee 4 hits wallet, tracked as open fee
	if g := ResidualCash(10000, 9996, 20, 20, 0, 4, 0, 0); abs(g) > 0.01 {
		t.Fatalf("entry fee should be 0, got %v", g)
	}
}

func TestResidualCashLighterEntrySettled(t *testing.T) {
	// Lighter Collateral often stays put; settled equity drops by the taker fee.
	if g := ResidualCash(10000, 9993.31, 0, 0, 0, 6.69, 0, 0); abs(g) > 0.01 {
		t.Fatalf("settled+openFees should cancel, got %v", g)
	}
}

func TestResidualCashCloseAfterJournal(t *testing.T) {
	// Open already took $6.69 fee out of settled; close net −112.70 includes that fee.
	// Settled therefore moves by net + entry fee = −106.01.
	if g := ResidualCash(9993.31, 9887.30, 0, -112.70, 6.69, 0, 0, 0); abs(g) > 0.02 {
		t.Fatalf("close after journal should be 0, got %v", g)
	}
}

func TestSettledWalletIgnoresMark(t *testing.T) {
	if g := SettledWallet(10100, 100); abs(g-10000) > 1e-9 {
		t.Fatalf("settled %v", g)
	}
}

func TestResidualCashMarkMoveIgnored(t *testing.T) {
	// wallet unchanged when only uPnL moves
	if g := ResidualCash(9900, 9900, 10, 10, 0, 0, 0, 0); abs(g) > 0.01 {
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

func TestResidualCashFundingAccrualNotDeposit(t *testing.T) {
	// funding posts to wallet during the hold; journal funding on the open trade matches it
	if g := ResidualCash(10000, 10008.78, 0, 0, 0, 0, 0, 8.78); abs(g) > 0.01 {
		t.Fatalf("accrual %v", g)
	}
}

func TestResidualCashCloseDoesNotCountFundingTwice(t *testing.T) {
	// LastWallet already contains +8.78 funding. Close books net +781.26 of which +8.78 is funding.
	// realizedXF +772.48, funding unchanged (moved open → closed).
	if g := ResidualCash(10008.78, 10781.26, 0, 772.48, 0, 0, 8.78, 8.78); abs(g) > 0.02 {
		t.Fatalf("close %v", g)
	}
}

func TestCloseDustCash(t *testing.T) {
	if !closeDustCash(-8.74, 781.26, 30) {
		t.Fatal("ETH close residual")
	}
	if !closeDustCash(-15.51, 803.93, 30) {
		t.Fatal("BTC close residual")
	}
	if closeDustCash(-200, 0, 30) {
		t.Fatal("real withdraw")
	}
	if closeDustCash(-8, 10, 30) {
		t.Fatal("explained not large enough")
	}
}

func abs(x float64) float64 {
	if x < 0 {
		return -x
	}
	return x
}
