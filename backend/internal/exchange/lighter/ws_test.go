package lighter

import (
	"testing"

	"donchian.trade/bot/internal/strategy"
)

func TestHandleDualCandleRollover(t *testing.T) {
	w := NewWS("", map[uint16]string{1: "BTC"}, "15m")
	var closed *strategy.Bar
	var live strategy.Bar
	var n int
	w.OnCandle = func(id uint16, c *strategy.Bar, l strategy.Bar) {
		n++
		closed, live = c, l
	}
	w.handle([]byte(`{"type":"update/candle","channel":"candle:1:15m","candles":[
		{"t":1000,"o":1,"h":2,"l":0.5,"c":1.5,"v":10},
		{"t":1900,"o":1.5,"h":1.6,"l":1.4,"c":1.55,"v":1}
	]}`))
	if n != 1 || closed == nil || closed.Time != 1000 || live.Time != 1900 {
		t.Fatalf("dual: n=%d closed=%v live=%v", n, closed, live)
	}
}

func TestHandleTAdvanceEmitsCachedClose(t *testing.T) {
	w := NewWS("", map[uint16]string{1: "BTC"}, "15m")
	var closes []int64
	var lives []int64
	w.OnCandle = func(id uint16, c *strategy.Bar, l strategy.Bar) {
		lives = append(lives, l.Time)
		if c != nil {
			closes = append(closes, c.Time)
		}
	}
	w.handle([]byte(`{"type":"update/candle","channel":"candle:1:15m","candles":[
		{"t":1000,"o":1,"h":2,"l":0.5,"c":1.8,"v":10}
	]}`))
	w.handle([]byte(`{"type":"update/candle","channel":"candle:1:15m","candles":[
		{"t":1000,"o":1,"h":2.1,"l":0.5,"c":1.9,"v":11}
	]}`))
	if len(closes) != 0 {
		t.Fatalf("same-t updates should not close: %v", closes)
	}
	w.handle([]byte(`{"type":"update/candle","channel":"candle:1:15m","candles":[
		{"t":1900,"o":1.9,"h":1.95,"l":1.85,"c":1.92,"v":1}
	]}`))
	if len(closes) != 1 || closes[0] != 1000 {
		t.Fatalf("want close at 1000, got %v", closes)
	}
	if lives[len(lives)-1] != 1900 {
		t.Fatalf("live=%v", lives)
	}
}

func TestHandleSameTDoesNotDoubleClose(t *testing.T) {
	w := NewWS("", map[uint16]string{0: "ETH"}, "15m")
	var closes int
	w.OnCandle = func(id uint16, c *strategy.Bar, l strategy.Bar) {
		if c != nil {
			closes++
		}
	}
	w.handle([]byte(`{"type":"update/candle","channel":"candle:0:15m","candles":[{"t":1000,"o":1,"h":1,"l":1,"c":1,"v":1}]}`))
	w.handle([]byte(`{"type":"update/candle","channel":"candle:0:15m","candles":[{"t":1900,"o":1,"h":1,"l":1,"c":1,"v":1}]}`))
	w.handle([]byte(`{"type":"update/candle","channel":"candle:0:15m","candles":[{"t":1900,"o":1,"h":1.1,"l":1,"c":1.05,"v":2}]}`))
	if closes != 1 {
		t.Fatalf("closes=%d want 1", closes)
	}
}
