package lighter

import (
	"context"
	"encoding/json"
	"fmt"
	"net/url"
	"strconv"
	"time"
)

// Fill is one Lighter trade print against our account.
type Fill struct {
	MarketID  uint8
	Price     float64
	Size      float64
	USD       float64
	Fee       float64
	Timestamp int64
	TxHash    string
	AskClient int64
	BidClient int64
	AskAcct   int64
	BidAcct   int64
}

func (c *HTTPClient) AccountTrades(ctx context.Context, accountIndex int64, marketID uint8, limit int) ([]Fill, error) {
	if limit <= 0 {
		limit = 50
	}
	if limit > 100 {
		limit = 100
	}
	q := url.Values{}
	q.Set("account_index", strconv.FormatInt(accountIndex, 10))
	q.Set("sort_by", "timestamp")
	q.Set("sort_dir", "desc")
	q.Set("limit", strconv.Itoa(limit))
	if marketID != 255 {
		q.Set("market_id", strconv.Itoa(int(marketID)))
	}
	b, err := c.get(ctx, "/api/v1/trades", q, true)
	if err != nil {
		return nil, err
	}
	var wrap struct {
		Code    int               `json:"code"`
		Message string            `json:"message"`
		Trades  []json.RawMessage `json:"trades"`
	}
	if err := json.Unmarshal(b, &wrap); err != nil {
		return nil, err
	}
	if wrap.Code != 0 && wrap.Code != 200 {
		return nil, fmt.Errorf("trades code=%d msg=%s", wrap.Code, wrap.Message)
	}
	out := make([]Fill, 0, len(wrap.Trades))
	for _, raw := range wrap.Trades {
		var m map[string]any
		if err := json.Unmarshal(raw, &m); err != nil {
			continue
		}
		mid := intField(m, "market_id")
		if mid < 0 || mid > 255 {
			continue
		}
		out = append(out, Fill{
			MarketID:  uint8(mid),
			Price:     floatField(m, "price"),
			Size:      floatField(m, "size"),
			USD:       floatField(m, "usd_amount"),
			Fee:       takerFeeUSD(m),
			Timestamp: int64(intField(m, "timestamp")),
			TxHash:    strField(m, "tx_hash"),
			AskClient: int64(intField(m, "ask_client_id")),
			BidClient: int64(intField(m, "bid_client_id")),
			AskAcct:   int64(intField(m, "ask_account_id")),
			BidAcct:   int64(intField(m, "bid_account_id")),
		})
	}
	return out, nil
}

// takerFeeUSD: Lighter's taker_fee is an integer of unclear scale. Only keep
// values that look like a dollar fee (positive and < 2% of notional).
func takerFeeUSD(m map[string]any) float64 {
	fee := floatField(m, "taker_fee")
	notional := floatField(m, "usd_amount")
	if notional <= 0 {
		notional = floatField(m, "price") * floatField(m, "size")
	}
	if fee > 0 && notional > 0 && fee < notional*0.02 {
		return fee
	}
	return 0
}

func FillsForClient(fills []Fill, clientIdx int64) []Fill {
	if clientIdx == 0 {
		return nil
	}
	var out []Fill
	for _, f := range fills {
		if f.AskClient == clientIdx || f.BidClient == clientIdx {
			out = append(out, f)
		}
	}
	return out
}

func FillsForTx(fills []Fill, txHash string) []Fill {
	if txHash == "" {
		return nil
	}
	var out []Fill
	for _, f := range fills {
		if f.TxHash == txHash {
			out = append(out, f)
		}
	}
	return out
}

// VWAP is size-weighted average price. fee is the sum of plausible dollar fees.
func VWAP(fills []Fill) (px, qty, fee float64) {
	var quote float64
	for _, f := range fills {
		if f.Size <= 0 || f.Price <= 0 {
			continue
		}
		qty += f.Size
		quote += f.Size * f.Price
		fee += f.Fee
	}
	if qty <= 0 {
		return 0, 0, 0
	}
	return quote / qty, qty, fee
}

func (c *HTTPClient) WaitFill(ctx context.Context, accountIndex int64, marketID uint8, clientIdx int64, txHash string, fallback float64) (px, fee float64) {
	deadline := time.Now().Add(2 * time.Second)
	for {
		fills, err := c.AccountTrades(ctx, accountIndex, marketID, 50)
		if err == nil {
			subset := FillsForClient(fills, clientIdx)
			if len(subset) == 0 {
				subset = FillsForTx(fills, txHash)
			}
			if p, _, f := VWAP(subset); p > 0 {
				return p, f
			}
		}
		if time.Now().After(deadline) {
			return fallback, 0
		}
		select {
		case <-ctx.Done():
			return fallback, 0
		case <-time.After(250 * time.Millisecond):
		}
	}
}
