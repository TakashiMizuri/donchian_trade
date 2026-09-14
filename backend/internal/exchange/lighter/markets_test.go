package lighter

import (
	"context"
	"net/http"
	"net/http/httptest"
	"testing"
)

func TestResolveMarketsTestnetIDs(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.Write([]byte(`{"code":200,"order_book_details":[
			{"symbol":"BTC","market_id":4096,"supported_size_decimals":5,"supported_price_decimals":1},
			{"symbol":"ETH","market_id":4095,"supported_size_decimals":4,"supported_price_decimals":2}
		]}`))
	}))
	defer srv.Close()

	got, err := NewHTTP(srv.URL).ResolveMarkets(context.Background(), []string{"BTC", "ETH"})
	if err != nil {
		t.Fatal(err)
	}
	if got["BTC"].MarketID != 4096 {
		t.Fatalf("BTC id %d", got["BTC"].MarketID)
	}
	if got["ETH"].MarketID != 4095 {
		t.Fatalf("ETH id %d", got["ETH"].MarketID)
	}
}

func TestResolveMarketsMainnetSmallIDs(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.Write([]byte(`{"code":200,"order_book_details":[{"symbol":"ETH","market_id":0},{"symbol":"BTC","market_id":1}]}`))
	}))
	defer srv.Close()

	got, err := NewHTTP(srv.URL).ResolveMarkets(context.Background(), []string{"BTC", "ETH"})
	if err != nil {
		t.Fatal(err)
	}
	if got["ETH"].MarketID != 0 || got["BTC"].MarketID != 1 {
		t.Fatalf("got ETH=%d BTC=%d", got["ETH"].MarketID, got["BTC"].MarketID)
	}
}

func TestParseMarketFromChannelWideIDs(t *testing.T) {
	if parseMarketFromChannel("candle/4096/1h") != 4096 {
		t.Fatal("slash form")
	}
	if parseMarketFromChannel("candle:4095:1h") != 4095 {
		t.Fatal("colon form")
	}
	if parseMarketFromChannel("candle:0:1h") != 0 {
		t.Fatal("mainnet eth")
	}
}
