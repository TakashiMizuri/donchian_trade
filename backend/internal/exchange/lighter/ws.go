package lighter

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"strings"
	"sync"
	"time"

	"donchian.trade/bot/internal/strategy"

	"github.com/gorilla/websocket"
)

type CandleHandler func(marketID uint8, closed *strategy.Bar, live strategy.Bar)

type WS struct {
	URL       string
	Markets   map[uint8]string // id → symbol
	OnCandle  CandleHandler
	OnState   func(connected bool, err string)
	mu        sync.Mutex
	conn      *websocket.Conn
	closed    map[string]int64 // symbol → last closed bar time
}

func NewWS(url string, markets map[uint8]string) *WS {
	return &WS{URL: url, Markets: markets, closed: map[string]int64{}}
}

func (w *WS) Run(ctx context.Context) error {
	backoff := time.Second
	for {
		if ctx.Err() != nil {
			return ctx.Err()
		}
		err := w.once(ctx)
		if w.OnState != nil {
			w.OnState(false, errString(err))
		}
		select {
		case <-ctx.Done():
			return ctx.Err()
		case <-time.After(backoff):
		}
		if backoff < 30*time.Second {
			backoff *= 2
		}
	}
}

func errString(err error) string {
	if err == nil {
		return ""
	}
	return err.Error()
}

func (w *WS) once(ctx context.Context) error {
	d := websocket.Dialer{HandshakeTimeout: 15 * time.Second}
	conn, _, err := d.DialContext(ctx, w.URL, http.Header{})
	if err != nil {
		return err
	}
	w.mu.Lock()
	w.conn = conn
	w.mu.Unlock()
	defer conn.Close()
	if w.OnState != nil {
		w.OnState(true, "")
	}
	for id := range w.Markets {
		msg := fmt.Sprintf(`{"type":"subscribe","channel":"candle/%d/1h"}`, id)
		if err := conn.WriteMessage(websocket.TextMessage, []byte(msg)); err != nil {
			return err
		}
	}
	_ = conn.SetReadDeadline(time.Now().Add(90 * time.Second))
	conn.SetPongHandler(func(string) error {
		_ = conn.SetReadDeadline(time.Now().Add(90 * time.Second))
		return nil
	})
	go func() {
		t := time.NewTicker(20 * time.Second)
		defer t.Stop()
		for {
			select {
			case <-ctx.Done():
				return
			case <-t.C:
				w.mu.Lock()
				c := w.conn
				w.mu.Unlock()
				if c != nil {
					_ = c.WriteControl(websocket.PingMessage, []byte("ping"), time.Now().Add(5*time.Second))
				}
			}
		}
	}()
	for {
		select {
		case <-ctx.Done():
			return ctx.Err()
		default:
		}
		_, data, err := conn.ReadMessage()
		if err != nil {
			return err
		}
		_ = conn.SetReadDeadline(time.Now().Add(90 * time.Second))
		w.handle(data)
	}
}

type wsMsg struct {
	Type     string      `json:"type"`
	Channel  string      `json:"channel"`
	Candles  []CandleRaw `json:"candles"`
	Error    json.RawMessage
}

func (w *WS) handle(data []byte) {
	var msg wsMsg
	if err := json.Unmarshal(data, &msg); err != nil {
		return
	}
	if !strings.Contains(msg.Type, "candle") && !strings.Contains(msg.Channel, "candle") {
		return
	}
	if len(msg.Candles) == 0 {
		return
	}
	marketID := parseMarketFromChannel(msg.Channel)
	symbol := w.Markets[marketID]
	// Rollover: two candles, first is the just-closed bar.
	if len(msg.Candles) >= 2 {
		closed := rawToBar(msg.Candles[0])
		live := rawToBar(msg.Candles[1])
		w.noteClosed(symbol, closed.Time)
		if w.OnCandle != nil {
			w.OnCandle(marketID, &closed, live)
		}
		return
	}
	live := rawToBar(msg.Candles[0])
	if w.OnCandle != nil {
		w.OnCandle(marketID, nil, live)
	}
}

func (w *WS) noteClosed(symbol string, t int64) {
	w.mu.Lock()
	defer w.mu.Unlock()
	if t > w.closed[symbol] {
		w.closed[symbol] = t
	}
}

func rawToBar(c CandleRaw) strategy.Bar {
	return strategy.Bar{
		Time:   candleTimeSec(c.T),
		Open:   c.O,
		High:   c.H,
		Low:    c.L,
		Close:  c.C,
		Volume: c.V,
	}
}

func parseMarketFromChannel(ch string) uint8 {
	// candle:0:1h  or candle/0/1h
	ch = strings.ReplaceAll(ch, "/", ":")
	parts := strings.Split(ch, ":")
	if len(parts) >= 2 {
		var id int
		fmt.Sscanf(parts[1], "%d", &id)
		if id >= 0 && id <= 255 {
			return uint8(id)
		}
	}
	return 0
}
