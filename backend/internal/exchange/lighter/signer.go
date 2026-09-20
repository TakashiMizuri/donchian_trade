package lighter

import (
	"context"
	"encoding/json"
	"fmt"
	"math"
	"net/url"
	"strconv"
	"sync"
	"time"

	lighterclient "github.com/elliottech/lighter-go/client"
	"github.com/elliottech/lighter-go/types"
	"github.com/elliottech/lighter-go/types/txtypes"
)

const (
	authTokenTTL    = 7 * time.Hour
	authRefreshSkew = 10 * time.Minute
)

type Signer struct {
	Tx   *lighterclient.TxClient
	HTTP *HTTPClient
	Meta map[string]MarketMeta

	tokMu  sync.Mutex
	token  string
	tokExp time.Time
}

func NewSigner(httpClient *HTTPClient, privKey string, accountIndex int64, apiKeyIndex uint8, chainID uint32) (*Signer, error) {
	tx, err := lighterclient.NewTxClient(httpClient, privKey, accountIndex, apiKeyIndex, chainID)
	if err != nil {
		return nil, err
	}
	return &Signer{Tx: tx, HTTP: httpClient, Meta: map[string]MarketMeta{}}, nil
}

func (s *Signer) AuthToken() (string, error) {
	now := time.Now()
	s.tokMu.Lock()
	defer s.tokMu.Unlock()
	if s.token != "" && tokenFresh(now, s.tokExp) {
		return s.token, nil
	}
	deadline := now.Add(authTokenTTL)
	tok, err := s.Tx.GetAuthToken(deadline)
	if err != nil {
		return "", err
	}
	s.token = tok
	s.tokExp = deadline
	return tok, nil
}

func tokenFresh(now, exp time.Time) bool {
	return now.Add(authRefreshSkew).Before(exp)
}

type OrderResult struct {
	TxHash           string
	ClientOrderIndex int64
	TxInfo           string
}

func txJSON(v interface{ GetTxInfo() (string, error) }) (string, error) {
	return v.GetTxInfo()
}

func (s *Signer) meta(symbol string) (MarketMeta, error) {
	m, ok := s.Meta[symbol]
	if !ok {
		return MarketMeta{}, fmt.Errorf("unknown market %s", symbol)
	}
	return m, nil
}

func (s *Signer) MarketIOC(symbol string, buy bool, qty, refPrice, slippage float64, clientIdx int64, reduceOnly bool) (*OrderResult, error) {
	m, err := s.meta(symbol)
	if err != nil {
		return nil, err
	}
	if slippage <= 0 {
		slippage = 0.02
	}
	worst := refPrice * (1 + slippage)
	if !buy {
		worst = refPrice * (1 - slippage)
		if worst <= 0 {
			worst = refPrice * 0.5
		}
	}
	isAsk := uint8(0)
	if !buy {
		isAsk = 1
	}
	ro := uint8(0)
	if reduceOnly {
		ro = 1
	}
	amt := ScaleAmount(qty, m.SizeDecimals)
	if amt <= 0 {
		return nil, fmt.Errorf("qty scales to 0 (qty=%v decimals=%d)", qty, m.SizeDecimals)
	}
	px := ScalePrice(worst, m.PriceDecimals)
	if px < 1 {
		px = 1
	}
	req := &types.CreateOrderTxReq{
		MarketIndex:      int16(m.MarketID),
		ClientOrderIndex: clientIdx,
		BaseAmount:       amt,
		Price:            px,
		IsAsk:            isAsk,
		Type:             txtypes.MarketOrder,
		TimeInForce:      txtypes.ImmediateOrCancel,
		ReduceOnly:       ro,
		TriggerPrice:     txtypes.NilOrderTriggerPrice,
		OrderExpiry:      txtypes.NilOrderExpiry,
	}
	info, err := s.Tx.GetCreateOrderTransaction(req, nil)
	if err != nil {
		return nil, err
	}
	js, err := txJSON(info)
	if err != nil {
		return nil, err
	}
	resp, err := s.HTTP.SendTx(context.Background(), txtypes.TxTypeL2CreateOrder, js)
	if err != nil {
		return nil, err
	}
	return &OrderResult{TxHash: resp.TxHash, ClientOrderIndex: clientIdx, TxInfo: js}, nil
}

func (s *Signer) StopLoss(symbol string, buyPosition bool, qty, stop, slippage float64, clientIdx int64) (*OrderResult, error) {
	m, err := s.meta(symbol)
	if err != nil {
		return nil, err
	}
	if slippage <= 0 {
		slippage = 0.02
	}
	isAsk := uint8(1)
	worst := stop * (1 - slippage)
	if !buyPosition {
		isAsk = 0
		worst = stop * (1 + slippage)
	}
	if worst <= 0 {
		worst = stop
	}
	amt := ScaleAmount(qty, m.SizeDecimals)
	if amt <= 0 {
		return nil, fmt.Errorf("SL qty scales to 0")
	}
	px := ScalePrice(worst, m.PriceDecimals)
	trig := ScalePrice(stop, m.PriceDecimals)
	if px < 1 {
		px = 1
	}
	if trig < 1 {
		trig = 1
	}
	req := &types.CreateOrderTxReq{
		MarketIndex:      int16(m.MarketID),
		ClientOrderIndex: clientIdx,
		BaseAmount:       amt,
		Price:            px,
		IsAsk:            isAsk,
		Type:             txtypes.StopLossOrder,
		TimeInForce:      txtypes.ImmediateOrCancel, // required by Lighter for SL market
		ReduceOnly:       1,
		TriggerPrice:     trig,
		OrderExpiry:      time.Now().Add(28 * 24 * time.Hour).UnixMilli(),
	}
	info, err := s.Tx.GetCreateOrderTransaction(req, nil)
	if err != nil {
		return nil, err
	}
	js, err := txJSON(info)
	if err != nil {
		return nil, err
	}
	resp, err := s.HTTP.SendTx(context.Background(), txtypes.TxTypeL2CreateOrder, js)
	if err != nil {
		return nil, err
	}
	return &OrderResult{TxHash: resp.TxHash, ClientOrderIndex: clientIdx, TxInfo: js}, nil
}

func (s *Signer) Cancel(symbol string, orderIndex int64) (*OrderResult, error) {
	m, err := s.meta(symbol)
	if err != nil {
		return nil, err
	}
	info, err := s.Tx.GetCancelOrderTransaction(&types.CancelOrderTxReq{
		MarketIndex: int16(m.MarketID),
		Index:       orderIndex,
	}, nil)
	if err != nil {
		return nil, err
	}
	js, err := txJSON(info)
	if err != nil {
		return nil, err
	}
	resp, err := s.HTTP.SendTx(context.Background(), txtypes.TxTypeL2CancelOrder, js)
	if err != nil {
		return nil, err
	}
	return &OrderResult{TxHash: resp.TxHash}, nil
}

func (s *Signer) CancelAll(symbol string) error {
	info, err := s.Tx.GetCancelAllOrdersTransaction(&types.CancelAllOrdersTxReq{
		TimeInForce: txtypes.ImmediateCancelAll,
		Time:        0,
	}, nil)
	if err != nil {
		return err
	}
	js, err := txJSON(info)
	if err != nil {
		return err
	}
	_, err = s.HTTP.SendTx(context.Background(), txtypes.TxTypeL2CancelAllOrders, js)
	_ = symbol
	return err
}

func RoundQty(qty float64, decimals int) float64 {
	p := math.Pow10(decimals)
	return math.Floor(qty*p) / p
}

func (c *HTTPClient) GetApiKey(accountIndex int64, apiKeyIndex uint8) (string, error) {
	q := url.Values{}
	q.Set("account_index", strconv.FormatInt(accountIndex, 10))
	b, err := c.get(context.Background(), "/api/v1/apikeys", q, false)
	if err != nil {
		return "", err
	}
	var wrap struct {
		ApiKeys []struct {
			ApiKeyIndex uint8  `json:"api_key_index"`
			PublicKey   string `json:"public_key"`
		} `json:"api_keys"`
	}
	if err := json.Unmarshal(b, &wrap); err != nil {
		return "", err
	}
	for _, k := range wrap.ApiKeys {
		if k.ApiKeyIndex == apiKeyIndex {
			return k.PublicKey, nil
		}
	}
	return "", fmt.Errorf("no api key returned for index %d", apiKeyIndex)
}

func (c *HTTPClient) InvalidateApiKeys(accountIndex int64) {}
