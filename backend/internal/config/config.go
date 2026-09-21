package config

import (
	"fmt"
	"os"
	"strconv"
	"strings"
	"time"
)

type Network string

const (
	NetworkTestnet Network = "testnet"
	NetworkMainnet Network = "mainnet"
)

type Config struct {
	Network           Network
	BaseURL           string
	WSURL             string
	ChainID           uint32
	APIPrivateKey     string
	AccountIndex      int64
	APIKeyIndex       uint8
	L1Address         string
	Symbols           []string
	ShadowBaseline    bool
	ShadowLS5         bool
	StartEquity       float64 // ignored for shadow; first live balance / later cash-flows are the source of truth
	CashFlowMinUSD    float64
	MaxNotionalUSD    float64
	DailyLossLimitUSD float64
	MaxLeverage       float64
	MarketSlippage    float64
	KillSwitch        bool
	SQLitePath        string
	HTTPAddr          string
	DashboardPassword string
	CookieSecure      bool
	TelegramToken     string
	TelegramChatIDs   []int64
	TelegramHeartbeat time.Duration
	DryRun            bool
	CandleWarmup      time.Duration
	ReconcileEvery    time.Duration
	WatchdogEvery     time.Duration

	Tag         string
	Resolution  string // Lighter candle interval: 1h, 30m, 15m, 5m
	BarSeconds  int
	ChannelN    int
	ExitM       int
	ATRPeriod   int
	ATRStopMult float64
	RiskPct     float64
	MaxRiskUSD  float64
	FeeRate     float64
	LiveProfile string
	LS5Enabled  bool
	LossStreakN int
	PauseHours  float64
	PauseBars   int
	ResumeATR   float64
	Timeframe   time.Duration
}

func Load() (*Config, error) {
	net := Network(strings.ToLower(getenv("LIGHTER_NETWORK", "testnet")))
	cfg := &Config{
		Network:           net,
		APIPrivateKey:     strings.TrimSpace(os.Getenv("LIGHTER_API_PRIVATE_KEY")),
		AccountIndex:      int64(getenvInt("LIGHTER_ACCOUNT_INDEX", 0)),
		APIKeyIndex:       uint8(getenvInt("LIGHTER_API_KEY_INDEX", 2)),
		L1Address:         strings.TrimSpace(os.Getenv("LIGHTER_L1_ADDRESS")),
		Symbols:           splitCSV(getenv("SYMBOLS", "BTC,ETH")),
		ShadowBaseline:    getenvBool("SHADOW_BASELINE", true),
		ShadowLS5:         getenvBool("SHADOW_LS5", true),
		StartEquity:       getenvFloat("START_EQUITY", 0),
		CashFlowMinUSD:    getenvFloat("CASH_FLOW_MIN_USD", 5),
		MaxNotionalUSD:    getenvFloat("MAX_NOTIONAL_USD", 50000),
		DailyLossLimitUSD: getenvFloat("DAILY_LOSS_LIMIT_USD", 500),
		MaxLeverage:       getenvFloat("MAX_LEVERAGE", 10),
		MarketSlippage:    getenvFloat("MARKET_SLIPPAGE", 0.02),
		KillSwitch:        getenvBool("KILL_SWITCH", false),
		SQLitePath:        getenv("SQLITE_PATH", "./data/bot.db"),
		HTTPAddr:          getenv("HTTP_ADDR", ":8080"),
		DashboardPassword: os.Getenv("DASHBOARD_PASSWORD"),
		CookieSecure:      getenvBool("COOKIE_SECURE", false),
		TelegramToken:     strings.TrimSpace(os.Getenv("TELEGRAM_BOT_TOKEN")),
		TelegramChatIDs:   parseChatIDs(os.Getenv("TELEGRAM_CHAT_IDS")),
		TelegramHeartbeat: time.Duration(getenvInt("TELEGRAM_HEARTBEAT_MINUTES", 60)) * time.Minute,
		DryRun:            getenvBool("DRY_RUN", false),
		CandleWarmup:      120 * 24 * time.Hour,
		ReconcileEvery:    15 * time.Second,
		WatchdogEvery:     20 * time.Second,
	}
	if err := applyStrategyEnv(cfg); err != nil {
		return nil, err
	}
	switch net {
	case NetworkMainnet:
		cfg.BaseURL = "https://mainnet.zklighter.elliot.ai"
		cfg.WSURL = "wss://mainnet.zklighter.elliot.ai/stream"
		cfg.ChainID = 304
	case NetworkTestnet, "":
		cfg.Network = NetworkTestnet
		cfg.BaseURL = "https://testnet.zklighter.elliot.ai"
		cfg.WSURL = "wss://testnet.zklighter.elliot.ai/stream"
		cfg.ChainID = 300
	default:
		return nil, fmt.Errorf("unknown LIGHTER_NETWORK %q (use testnet or mainnet)", net)
	}
	if cfg.APIKeyIndex < 2 {
		return nil, fmt.Errorf("LIGHTER_API_KEY_INDEX must be 2-254 (0-1 are reserved for the Lighter UI)")
	}
	if len(cfg.Symbols) == 0 {
		return nil, fmt.Errorf("SYMBOLS is empty")
	}
	if cfg.DashboardPassword == "" {
		return nil, fmt.Errorf("DASHBOARD_PASSWORD is required")
	}
	return cfg, nil
}

func getenv(k, def string) string {
	if v := strings.TrimSpace(os.Getenv(k)); v != "" {
		return v
	}
	return def
}

func getenvInt(k string, def int) int {
	if v := strings.TrimSpace(os.Getenv(k)); v != "" {
		n, err := strconv.Atoi(v)
		if err == nil {
			return n
		}
	}
	return def
}

func getenvFloat(k string, def float64) float64 {
	if v := strings.TrimSpace(os.Getenv(k)); v != "" {
		n, err := strconv.ParseFloat(v, 64)
		if err == nil {
			return n
		}
	}
	return def
}

func getenvBool(k string, def bool) bool {
	if v := strings.TrimSpace(os.Getenv(k)); v != "" {
		b, err := strconv.ParseBool(v)
		if err == nil {
			return b
		}
	}
	return def
}

func splitCSV(s string) []string {
	parts := strings.Split(s, ",")
	out := make([]string, 0, len(parts))
	for _, p := range parts {
		p = strings.TrimSpace(strings.ToUpper(p))
		if p != "" {
			out = append(out, p)
		}
	}
	return out
}

func parseChatIDs(s string) []int64 {
	var out []int64
	for _, p := range strings.Split(s, ",") {
		p = strings.TrimSpace(p)
		if p == "" {
			continue
		}
		n, err := strconv.ParseInt(p, 10, 64)
		if err == nil {
			out = append(out, n)
		}
	}
	return out
}
