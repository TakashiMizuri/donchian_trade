package engine

import (
	"database/sql"
	"testing"

	"donchian.trade/bot/internal/store"
	"donchian.trade/bot/internal/strategy"
)

func TestOwnEntryInFlightSkipsUnexpectedFlatten(t *testing.T) {
	if !ownEntryInFlight(true, false) {
		t.Fatal("busy HandleClosed must not flatten")
	}
	if !ownEntryInFlight(false, true) {
		t.Fatal("sqlite open live trade must not flatten")
	}
	if ownEntryInFlight(false, false) {
		t.Fatal("true orphan exchange size should still flatten")
	}
}

func TestPositionFromOpenUsesLiveEntry(t *testing.T) {
	open := &store.TradeRow{
		ID:          17,
		Direction:   "BUY",
		EntryPrice:  sql.NullFloat64{Float64: 83564.4, Valid: true},
		EntryPxLive: 83554.4,
		Stop:        sql.NullFloat64{Float64: 83383.77, Valid: true},
		Quantity:    sql.NullFloat64{Float64: 0.00904, Valid: true},
		SignalTime:  1792572300,
		EntryTime:   sql.NullInt64{Int64: 1792572600, Valid: true},
	}
	p := positionFromOpen(open, nil)
	if p == nil || p.Direction != strategy.DirBuy || p.EntryPrice != 83554.4 || p.Quantity != 0.00904 {
		t.Fatalf("%+v", p)
	}
}
