package lighter

import (
	"context"
	"io"
	"net/http"
	"net/http/httptest"
	"testing"
)

func TestNormalizeTriggerUnscalesTicks(t *testing.T) {
	got := NormalizeTrigger(218270, 2182.70, 2)
	if got < 2182.69 || got > 2182.71 {
		t.Fatalf("got %v", got)
	}
	same := NormalizeTrigger(2182.70, 2182.69, 2)
	if same < 2182.69 || same > 2182.71 {
		t.Fatalf("already in price units: %v", same)
	}
	trail := NormalizeTrigger(2063, 2142, 2)
	if trail != 2063 {
		t.Fatalf("nearby price must stay %v", trail)
	}
}

func TestActiveOrdersParsesTriggerAliases(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/api/v1/accountActiveOrders" {
			w.WriteHeader(http.StatusNotFound)
			return
		}
		w.Header().Set("Content-Type", "application/json")
		_, _ = io.WriteString(w, `{"orders":[{
			"order_index": 11,
			"client_order_index": 22,
			"market_id": 0,
			"remaining_base_amount": "0.1",
			"price": "2100.00",
			"trigger_px": "2182.70",
			"is_ask": false,
			"reduce_only": true,
			"order_type": "stop-loss",
			"status": "open"
		}]}`)
	}))
	defer srv.Close()

	got, err := NewHTTP(srv.URL).ActiveOrders(context.Background(), 1, 0)
	if err != nil {
		t.Fatal(err)
	}
	if len(got) != 1 {
		t.Fatalf("len %d", len(got))
	}
	o := got[0]
	if o.ClientOrderIndex != 22 || o.Trigger < 2182.69 || o.Trigger > 2182.71 || !o.ReduceOnly || o.Type != "stop-loss" {
		t.Fatalf("%+v", o)
	}
}
