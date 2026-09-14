package lighter

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"math"
	"net/http"
	"net/url"
	"sort"
	"strconv"
	"strings"
	"sync"
	"time"

	"donchian.trade/bot/internal/strategy"
)

type HTTPClient struct {
	BaseURL    string
	HTTP       *http.Client
	AuthToken  string
	mu         sync.Mutex
	tokenFn    func(context.Context) (string, error)
}

func NewHTTP(baseURL string) *HTTPClient {
	return &HTTPClient{
		BaseURL: strings.TrimRight(baseURL, "/"),
		HTTP:    &http.Client{Timeout: 20 * time.Second},
	}
}

func (c *HTTPClient) SetTokenSource(fn func(context.Context) (string, error)) {
	c.tokenFn = fn
}

func (c *HTTPClient) get(ctx context.Context, path string, q url.Values, auth bool) ([]byte, error) {
	u := c.BaseURL + path
	if len(q) > 0 {
		u += "?" + q.Encode()
	}
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, u, nil)
	if err != nil {
		return nil, err
	}
	if auth {
		tok := c.AuthToken
		if c.tokenFn != nil {
			t, err := c.tokenFn(ctx)
			if err != nil {
				return nil, err
			}
			tok = t
		}
		if tok != "" {
			req.Header.Set("Authorization", tok)
		}
	}
	resp, err := c.HTTP.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()
	b, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, err
	}
	if resp.StatusCode >= 400 {
		return nil, fmt.Errorf("GET %s: HTTP %d: %s", path, resp.StatusCode, truncate(string(b), 400))
	}
	return b, nil
}

func (c *HTTPClient) postForm(ctx context.Context, path string, form url.Values) ([]byte, error) {
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, c.BaseURL+path, strings.NewReader(form.Encode()))
	if err != nil {
		return nil, err
	}
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	resp, err := c.HTTP.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()
	b, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, err
	}
	if resp.StatusCode >= 400 {
		return nil, fmt.Errorf("POST %s: HTTP %d: %s", path, resp.StatusCode, truncate(string(b), 400))
	}
	return b, nil
}

func truncate(s string, n int) string {
	if len(s) <= n {
		return s
	}
	return s[:n] + "..."
}

type MarketMeta struct {
	Symbol        string
	MarketID      uint8
	SizeDecimals  int
	PriceDecimals int
	MinBaseAmount float64
}

type OrderBookDetailsResp struct {
	Code             int               `json:"code"`
	Message          string            `json:"message"`
	OrderBookDetails []json.RawMessage `json:"order_book_details"`
	OrderBooks       []json.RawMessage `json:"order_books"`
}

func (c *HTTPClient) Markets(ctx context.Context) ([]MarketMeta, error) {
	b, err := c.get(ctx, "/api/v1/orderBookDetails", nil, false)
	if err != nil {
		return nil, err
	}
	var wrap OrderBookDetailsResp
	if err := json.Unmarshal(b, &wrap); err != nil {
		return nil, err
	}
	raw := wrap.OrderBookDetails
	if len(raw) == 0 {
		raw = wrap.OrderBooks
	}
	var out []MarketMeta
	for _, r := range raw {
		var m map[string]any
		if err := json.Unmarshal(r, &m); err != nil {
			continue
		}
		sym := strField(m, "symbol", "market_symbol")
		id := intField(m, "market_id", "market_index", "index")
		if id < 0 || id > 255 {
			continue
		}
		out = append(out, MarketMeta{
			Symbol:        strings.ToUpper(sym),
			MarketID:      uint8(id),
			SizeDecimals:  intField(m, "supported_size_decimals", "size_decimals"),
			PriceDecimals: intField(m, "supported_price_decimals", "price_decimals"),
			MinBaseAmount: floatField(m, "min_base_amount"),
		})
	}
	return out, nil
}

func (c *HTTPClient) ResolveMarkets(ctx context.Context, symbols []string) (map[string]MarketMeta, error) {
	all, err := c.Markets(ctx)
	if err != nil {
		return nil, err
	}
	bySym := map[string]MarketMeta{}
	for _, m := range all {
		bySym[m.Symbol] = m
		// BTCUSDT → BTC
		if strings.HasSuffix(m.Symbol, "USDT") {
			bySym[strings.TrimSuffix(m.Symbol, "USDT")] = m
		}
		if strings.HasSuffix(m.Symbol, "USD") {
			bySym[strings.TrimSuffix(m.Symbol, "USD")] = m
		}
	}
	out := map[string]MarketMeta{}
	for _, s := range symbols {
		m, ok := bySym[strings.ToUpper(s)]
		if !ok {
			return nil, fmt.Errorf("market %s not found on Lighter (have %d books)", s, len(all))
		}
		out[strings.ToUpper(s)] = m
	}
	return out, nil
}

type CandleRaw struct {
	T int64   `json:"t"`
	O float64 `json:"o"`
	H float64 `json:"h"`
	L float64 `json:"l"`
	C float64 `json:"c"`
	V float64 `json:"v"`
}

type CandlesResp struct {
	Code    int         `json:"code"`
	Message string      `json:"message"`
	R       string      `json:"r"`
	C       []CandleRaw `json:"c"`
}

func candleTimeSec(t int64) int64 {
	if t > 1_000_000_000_000 {
		return t / 1000
	}
	return t
}

func (c *HTTPClient) Candles(ctx context.Context, marketID uint8, resolution string, startMs, endMs int64, countBack int) ([]strategy.Bar, error) {
	q := url.Values{}
	q.Set("market_id", strconv.Itoa(int(marketID)))
	q.Set("resolution", resolution)
	q.Set("start_timestamp", strconv.FormatInt(startMs, 10))
	q.Set("end_timestamp", strconv.FormatInt(endMs, 10))
	q.Set("count_back", strconv.Itoa(countBack))
	b, err := c.get(ctx, "/api/v1/candles", q, false)
	if err != nil {
		return nil, err
	}
	var resp CandlesResp
	if err := json.Unmarshal(b, &resp); err != nil {
		return nil, err
	}
	out := make([]strategy.Bar, 0, len(resp.C))
	for _, k := range resp.C {
		out = append(out, strategy.Bar{
			Time:   candleTimeSec(k.T),
			Open:   k.O,
			High:   k.H,
			Low:    k.L,
			Close:  k.C,
			Volume: k.V,
		})
	}
	return out, nil
}

func (c *HTTPClient) Backfill1h(ctx context.Context, marketID uint8, from time.Time) ([]strategy.Bar, error) {
	end := time.Now().UTC()
	var all []strategy.Bar
	seen := map[int64]struct{}{}
	curEnd := end
	for curEnd.After(from) {
		start := curEnd.Add(-20 * 24 * time.Hour)
		if start.Before(from) {
			start = from
		}
		batch, err := c.Candles(ctx, marketID, "1h", start.UnixMilli(), curEnd.UnixMilli(), 500)
		if err != nil {
			return nil, err
		}
		if len(batch) == 0 {
			break
		}
		oldest := batch[0].Time
		for _, b := range batch {
			if _, ok := seen[b.Time]; ok {
				continue
			}
			seen[b.Time] = struct{}{}
			all = append(all, b)
		}
		curEnd = time.Unix(oldest, 0).UTC().Add(-time.Hour)
		if len(batch) < 2 {
			break
		}
	}
	sort.Slice(all, func(i, j int) bool { return all[i].Time < all[j].Time })
	return all, nil
}

type Account struct {
	Index            int64
	Collateral       float64
	Available        float64
	TotalAssetValue  float64
	Positions        []Position
}

type Position struct {
	MarketID       uint8
	Symbol         string
	Sign           int
	Size           float64
	AvgEntry       float64
	UnrealizedPnL  float64
	PositionValue  float64
	FundingPaid    float64
	IM             float64
}

func (c *HTTPClient) Account(ctx context.Context, accountIndex int64) (*Account, error) {
	q := url.Values{}
	q.Set("by", "index")
	q.Set("value", strconv.FormatInt(accountIndex, 10))
	b, err := c.get(ctx, "/api/v1/account", q, false)
	if err != nil {
		return nil, err
	}
	var wrap struct {
		Code     int               `json:"code"`
		Message  string            `json:"message"`
		Accounts []json.RawMessage `json:"accounts"`
	}
	if err := json.Unmarshal(b, &wrap); err != nil {
		return nil, err
	}
	if len(wrap.Accounts) == 0 {
		// some responses put the account at the top level
		return parseAccount(b)
	}
	return parseAccount(wrap.Accounts[0])
}

func parseAccount(b []byte) (*Account, error) {
	var m map[string]any
	if err := json.Unmarshal(b, &m); err != nil {
		return nil, err
	}
	acc := &Account{
		Index:           int64(intField(m, "account_index", "index")),
		Collateral:      floatField(m, "collateral"),
		Available:       floatField(m, "available_balance"),
		TotalAssetValue: floatField(m, "total_asset_value"),
	}
	if acc.TotalAssetValue == 0 {
		acc.TotalAssetValue = acc.Collateral
	}
	rawPos, _ := m["positions"].([]any)
	for _, p := range rawPos {
		pm, ok := p.(map[string]any)
		if !ok {
			continue
		}
		id := intField(pm, "market_id")
		if id < 0 || id > 255 {
			continue
		}
		acc.Positions = append(acc.Positions, Position{
			MarketID:      uint8(id),
			Symbol:        strings.ToUpper(strField(pm, "symbol")),
			Sign:          intField(pm, "sign"),
			Size:          math.Abs(floatField(pm, "position")),
			AvgEntry:      floatField(pm, "avg_entry_price"),
			UnrealizedPnL: floatField(pm, "unrealized_pnl"),
			PositionValue: floatField(pm, "position_value"),
			FundingPaid:   floatField(pm, "total_funding_paid_out"),
			IM:            floatField(pm, "initial_margin_fraction"),
		})
	}
	return acc, nil
}

type OpenOrder struct {
	OrderIndex       int64
	ClientOrderIndex int64
	MarketID         uint8
	Remaining        float64
	Price            float64
	Trigger          float64
	IsAsk            bool
	ReduceOnly       bool
	Type             string
	Status           string
}

func (c *HTTPClient) ActiveOrders(ctx context.Context, accountIndex int64, marketID uint8) ([]OpenOrder, error) {
	q := url.Values{}
	q.Set("account_index", strconv.FormatInt(accountIndex, 10))
	q.Set("market_id", strconv.Itoa(int(marketID)))
	b, err := c.get(ctx, "/api/v1/accountActiveOrders", q, true)
	if err != nil {
		// fallback name used in some SDK versions
		b2, err2 := c.get(ctx, "/api/v1/orders", q, true)
		if err2 != nil {
			return nil, fmt.Errorf("active orders: %v / %v", err, err2)
		}
		b = b2
	}
	var wrap map[string]any
	if err := json.Unmarshal(b, &wrap); err != nil {
		return nil, err
	}
	var raw []any
	for _, key := range []string{"orders", "account_orders", "data"} {
		if v, ok := wrap[key].([]any); ok {
			raw = v
			break
		}
	}
	var out []OpenOrder
	for _, item := range raw {
		m, ok := item.(map[string]any)
		if !ok {
			continue
		}
		mid := intField(m, "market_id", "market_index")
		if mid < 0 || mid > 255 {
			mid = int(marketID)
		}
		out = append(out, OpenOrder{
			OrderIndex:       int64(intField(m, "order_index", "order_id")),
			ClientOrderIndex: int64(intField(m, "client_order_index", "client_order_id")),
			MarketID:         uint8(mid),
			Remaining:        floatField(m, "remaining_base_amount", "remaining_base_size"),
			Price:            floatField(m, "price"),
			Trigger:          floatField(m, "trigger_price"),
			IsAsk:            boolField(m, "is_ask") || strings.EqualFold(strField(m, "side"), "sell"),
			ReduceOnly:       boolField(m, "reduce_only"),
			Type:             strField(m, "type"),
			Status:           strField(m, "status"),
		})
	}
	return out, nil
}

type SendTxResp struct {
	Code    int    `json:"code"`
	Message string `json:"message"`
	TxHash  string `json:"tx_hash"`
}

func (c *HTTPClient) SendTx(ctx context.Context, txType int, txInfo string) (*SendTxResp, error) {
	form := url.Values{}
	form.Set("tx_type", strconv.Itoa(txType))
	form.Set("tx_info", txInfo)
	b, err := c.postForm(ctx, "/api/v1/sendTx", form)
	if err != nil {
		return nil, err
	}
	var resp SendTxResp
	if err := json.Unmarshal(b, &resp); err != nil {
		return nil, err
	}
	if resp.Code != 200 && resp.Code != 0 {
		return &resp, fmt.Errorf("sendTx code=%d msg=%s hash=%s", resp.Code, resp.Message, resp.TxHash)
	}
	return &resp, nil
}

func (c *HTTPClient) GetNextNonce(accountIndex int64, apiKeyIndex uint8) (int64, error) {
	q := url.Values{}
	q.Set("account_index", strconv.FormatInt(accountIndex, 10))
	q.Set("api_key_index", strconv.Itoa(int(apiKeyIndex)))
	b, err := c.get(context.Background(), "/api/v1/nextNonce", q, false)
	if err != nil {
		return 0, err
	}
	var wrap struct {
		Code  int   `json:"code"`
		Nonce int64 `json:"nonce"`
	}
	if err := json.Unmarshal(b, &wrap); err != nil {
		return 0, err
	}
	return wrap.Nonce, nil
}

func strField(m map[string]any, keys ...string) string {
	for _, k := range keys {
		if v, ok := m[k]; ok {
			switch t := v.(type) {
			case string:
				return t
			case float64:
				return strconv.FormatInt(int64(t), 10)
			case json.Number:
				return t.String()
			}
		}
	}
	return ""
}

func intField(m map[string]any, keys ...string) int {
	for _, k := range keys {
		if v, ok := m[k]; ok {
			switch t := v.(type) {
			case float64:
				return int(t)
			case int:
				return t
			case int64:
				return int(t)
			case string:
				n, _ := strconv.Atoi(t)
				return n
			case json.Number:
				n, _ := t.Int64()
				return int(n)
			}
		}
	}
	return 0
}

func floatField(m map[string]any, keys ...string) float64 {
	for _, k := range keys {
		if v, ok := m[k]; ok {
			switch t := v.(type) {
			case float64:
				return t
			case string:
				n, _ := strconv.ParseFloat(t, 64)
				return n
			case json.Number:
				n, _ := t.Float64()
				return n
			case int:
				return float64(t)
			}
		}
	}
	return 0
}

func boolField(m map[string]any, keys ...string) bool {
	for _, k := range keys {
		if v, ok := m[k]; ok {
			switch t := v.(type) {
			case bool:
				return t
			case float64:
				return t != 0
			case string:
				return t == "true" || t == "1"
			}
		}
	}
	return false
}

func ScalePrice(px float64, decimals int) uint32 {
	if decimals < 0 {
		decimals = 0
	}
	v := px * math.Pow10(decimals)
	if v < 0 {
		v = 0
	}
	if v > float64(math.MaxUint32) {
		v = float64(math.MaxUint32)
	}
	return uint32(math.Round(v))
}

func ScaleAmount(qty float64, decimals int) int64 {
	if decimals < 0 {
		decimals = 0
	}
	v := qty * math.Pow10(decimals)
	if v < 0 {
		v = 0
	}
	return int64(math.Round(v))
}

func Unscale(v float64, decimals int) float64 {
	return v / math.Pow10(decimals)
}
