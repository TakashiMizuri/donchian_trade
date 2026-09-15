package lighter

import "testing"

func TestVWAP(t *testing.T) {
	px, qty, fee := VWAP([]Fill{
		{Price: 100, Size: 1, Fee: 0.05},
		{Price: 110, Size: 1, Fee: 0.05},
	})
	if qty != 2 {
		t.Fatalf("qty %v", qty)
	}
	if px < 104.9 || px > 105.1 {
		t.Fatalf("vwap %v", px)
	}
	if fee < 0.09 || fee > 0.11 {
		t.Fatalf("fee %v", fee)
	}
}

func TestVWAPSkipsBadPrints(t *testing.T) {
	px, qty, _ := VWAP([]Fill{
		{Price: 0, Size: 9},
		{Price: 50, Size: 2},
	})
	if qty != 2 || px != 50 {
		t.Fatalf("got px=%v qty=%v", px, qty)
	}
}

func TestFillsForClient(t *testing.T) {
	all := []Fill{
		{AskClient: 7, Price: 1, Size: 1},
		{BidClient: 8, Price: 2, Size: 1},
		{AskClient: 7, Price: 3, Size: 1},
	}
	got := FillsForClient(all, 7)
	if len(got) != 2 {
		t.Fatalf("len %d", len(got))
	}
	if FillsForClient(all, 0) != nil {
		t.Fatal("zero client must not match")
	}
}

func TestTakerFeeUSD(t *testing.T) {
	if takerFeeUSD(map[string]any{"usd_amount": 10000.0}) != 0 {
		t.Fatal("omitted taker_fee is Standard $0")
	}
	if takerFeeUSD(map[string]any{"taker_fee": 0.0, "usd_amount": 10000.0}) != 0 {
		t.Fatal("zero taker_fee is Standard $0")
	}
	// Premium 500k LIT: 196 ppm = 0.0196% of $10k → $1.96
	if g := takerFeeUSD(map[string]any{"taker_fee": 196.0, "usd_amount": 10000.0}); g < 1.95 || g > 1.97 {
		t.Fatalf("ppm fee %v", g)
	}
	// Plus: 50 ppm = 0.5 bps of $10k → $0.50
	if g := takerFeeUSD(map[string]any{"taker_fee": 50.0, "usd_amount": 10000.0}); g < 0.49 || g > 0.51 {
		t.Fatalf("plus fee %v", g)
	}
	// USDC micro-units: 2.80 on $10k
	if g := takerFeeUSD(map[string]any{"taker_fee": 2_800_000.0, "usd_amount": 10000.0}); g < 2.79 || g > 2.81 {
		t.Fatalf("micro-USDC %v", g)
	}
	if g := takerFeeUSD(map[string]any{"taker_fee": 1.5, "usd_amount": 10000.0}); g != 1.5 {
		t.Fatalf("already-dollar fee %v", g)
	}
}
