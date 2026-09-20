package store

import (
	"context"
	"path/filepath"
	"testing"

	"donchian.trade/bot/internal/strategy"
)

func TestStoreRoundTrip(t *testing.T) {
	dir := t.TempDir()
	s, err := Open(filepath.Join(dir, "t.db"))
	if err != nil {
		t.Fatal(err)
	}
	defer s.Close()
	ctx := context.Background()
	bars := []strategy.Bar{{Time: 100, Open: 1, High: 2, Low: 0.5, Close: 1.5, Volume: 9}}
	if err := s.UpsertCandles(ctx, "BTC", bars); err != nil {
		t.Fatal(err)
	}
	got, err := s.LoadCandles(ctx, "BTC")
	if err != nil || len(got) != 1 || got[0].Close != 1.5 {
		t.Fatalf("candles %+v err=%v", got, err)
	}
	id, ok, err := s.TryOpenTrade(ctx, "BTC", "live", "BUY", 100, 200, 10, 9, 1, 0.5, 42)
	if err != nil || !ok || id == 0 {
		t.Fatalf("open %v %v %v", id, ok, err)
	}
	_, ok, err = s.TryOpenTrade(ctx, "BTC", "live", "BUY", 100, 200, 10, 9, 1, 0.5, 43)
	if err != nil || ok {
		t.Fatal("expected idempotent ignore")
	}
	if err := s.CloseTrade(ctx, id, 300, 11, "time", 1, 0.01, 0.01, 0, 0.98, 0, -1); err != nil {
		t.Fatal(err)
	}
	if err := s.SetEntryLive(ctx, id, 10, 10.05, 0.02); err != nil {
		t.Fatal(err)
	}
	if err := s.SetExitShadow(ctx, id, 11); err != nil {
		t.Fatal(err)
	}
	gotT, err := s.TradeBySignal(ctx, "BTC", "live", 100)
	if err != nil || gotT == nil {
		t.Fatal(err)
	}
	if gotT.EntryPxLive < 10.04 || gotT.EntryPxLive > 10.06 {
		t.Fatalf("entry live %v", gotT.EntryPxLive)
	}
	if gotT.ExitPxShadow != 11 {
		t.Fatalf("exit shadow %v", gotT.ExitPxShadow)
	}
	if err := s.UpsertBarLog(ctx, BarLog{Symbol: "BTC", BarTime: 100, Close: 1.5, WantLS5: "BUY"}); err != nil {
		t.Fatal(err)
	}
	if err := s.InsertCurve(ctx, CurvePoint{TS: 100, Reason: "bar", EquityLive: 10000, EquityShadowLS5: 10000}); err != nil {
		t.Fatal(err)
	}
	for i := 0; i < 5; i++ {
		if err := s.InsertCurve(ctx, CurvePoint{TS: int64(200 + i), Reason: "fill", EquityLive: 10001}); err != nil {
			t.Fatal(err)
		}
	}
	if err := s.InsertCurve(ctx, CurvePoint{TS: 300, Reason: "bar", EquityLive: 10100, EquityShadowLS5: 10100}); err != nil {
		t.Fatal(err)
	}
	pts, err := s.ListCurve(ctx, 10)
	if err != nil || len(pts) != 2 {
		t.Fatalf("curve want 2 bars, got %d err=%v", len(pts), err)
	}
	if pts[0].Reason != "bar" || pts[1].EquityLive != 10100 {
		t.Fatalf("curve %+v", pts)
	}
	if err := s.SaveCashWatch(ctx, CashWatch{Inited: true, LastWallet: 10000}); err != nil {
		t.Fatal(err)
	}
	if _, err := s.InsertCashFlow(ctx, CashFlow{Amount: 10000, Kind: "seed", LiveEquity: 10000, Note: "t"}); err != nil {
		t.Fatal(err)
	}
	if _, err := s.InsertCashFlow(ctx, CashFlow{Amount: 2500, Kind: "deposit", LiveEquity: 12500}); err != nil {
		t.Fatal(err)
	}
	sum, err := s.SumCash(ctx)
	if err != nil || sum < 12499 || sum > 12501 {
		t.Fatalf("sum cash %v %v", sum, err)
	}
}
