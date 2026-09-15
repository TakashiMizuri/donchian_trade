package telegram

import (
	"database/sql"
	"strings"
	"testing"

	"donchian.trade/bot/internal/engine"
	"donchian.trade/bot/internal/store"
	"donchian.trade/bot/internal/telemetry"
)

func TestFormatAlertEscapesHTML(t *testing.T) {
	got := formatAlert("error", "entry", "ETH <fail> & boom")
	if strings.Contains(got, "<fail>") {
		t.Fatalf("unescaped: %s", got)
	}
	if !strings.Contains(got, "Вход") || !strings.Contains(got, "ошибка") {
		t.Fatalf("title missing: %s", got)
	}
	if !strings.Contains(got, "&amp;") {
		t.Fatalf("amp missing: %s", got)
	}
}

func TestFormatTradesTakesLiveOnly(t *testing.T) {
	rows := []store.TradeRow{
		{Symbol: "ETH", Profile: "shadow_ls5", Direction: "BUY", Outcome: ns("sl")},
		{Symbol: "BTC", Profile: "live", Direction: "SELL", Outcome: ns("open"), EntryPxLive: 100},
		{Symbol: "ETH", Profile: "live", Direction: "BUY", Outcome: ns("sl"), Net: nf(-12), EntryPxLive: 10, ExitPxLive: 9},
	}
	got := formatTrades(rows, 10)
	if strings.Contains(got, "shadow") {
		t.Fatalf("shadow leaked: %s", got)
	}
	if !strings.Contains(got, "BTC") || !strings.Contains(got, "ETH") {
		t.Fatalf("live missing: %s", got)
	}
	if !strings.Contains(got, "открыта") || !strings.Contains(got, "стоп") {
		t.Fatalf("outcomes: %s", got)
	}
}

func TestFormatDailyHTML(t *testing.T) {
	sn := engine.Snapshot{
		Network: "testnet",
		Verdict: telemetry.VerdictWAIT,
		Report: telemetry.Report{
			EquityLive: 100, EquityShadowLS5: 101, GapUSD: -1, NMatched: 3, NUnion: 5,
			StatusA: "na", StatusB: "na", StatusC: "na",
		},
		Symbols: []engine.SymbolSnap{{
			Symbol: "BTC", Position: "SELL", LastPrice: 108000, Qty: 0.01,
			Entry: 109000, Stop: 110000, ShadowLS5: "SELL",
		}},
	}
	got := formatDailyHTML("2026-09-14", sn, nil)
	for _, need := range []string{"Суточный отчёт", "всё сходится", "BTC", "шорт", "Live"} {
		if !strings.Contains(got, need) {
			t.Fatalf("missing %q in %s", need, got)
		}
	}
}

func TestSplitTelegram(t *testing.T) {
	s := strings.Repeat("a\n", 2000)
	parts := splitTelegram(s, 100)
	if len(parts) < 2 {
		t.Fatalf("want split, got %d", len(parts))
	}
}

func ns(s string) sql.NullString { return sql.NullString{String: s, Valid: true} }
func nf(n float64) sql.NullFloat64 {
	return sql.NullFloat64{Float64: n, Valid: true}
}
