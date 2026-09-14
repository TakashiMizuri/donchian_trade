package main

import (
	"context"
	"fmt"
	"log/slog"
	"os"
	"os/signal"
	"syscall"
	"time"

	"donchian.trade/bot/internal/api"
	"donchian.trade/bot/internal/config"
	"donchian.trade/bot/internal/engine"
	"donchian.trade/bot/internal/exchange/lighter"
	"donchian.trade/bot/internal/notify"
	"donchian.trade/bot/internal/risk"
	"donchian.trade/bot/internal/store"
	"donchian.trade/bot/internal/telegram"
)

func main() {
	log := slog.New(slog.NewJSONHandler(os.Stdout, &slog.HandlerOptions{Level: slog.LevelInfo}))
	slog.SetDefault(log)

	cfg, err := config.Load()
	if err != nil {
		log.Error("config", "err", err)
		os.Exit(1)
	}
	if cfg.APIPrivateKey == "" && !cfg.DryRun {
		log.Error("LIGHTER_API_PRIVATE_KEY is required (or set DRY_RUN=true)")
		os.Exit(1)
	}

	ctx, cancel := signal.NotifyContext(context.Background(), syscall.SIGINT, syscall.SIGTERM)
	defer cancel()

	st, err := store.Open(cfg.SQLitePath)
	if err != nil {
		log.Error("sqlite", "err", err)
		os.Exit(1)
	}
	defer st.Close()

	g, _ := st.LoadGlobal(ctx)
	if cfg.KillSwitch {
		g.KillSwitch = true
	}
	rg := risk.New(cfg, g.KillSwitch, g.DailyPnL, g.DailyPnLDate)

	httpClient := lighter.NewHTTP(cfg.BaseURL)
	markets := map[string]lighter.MarketMeta{}
	if err := pingMarkets(ctx, httpClient, cfg, markets, log); err != nil {
		log.Error("markets", "err", err)
		os.Exit(1)
	}

	var signer *lighter.Signer
	if cfg.APIPrivateKey != "" {
		signer, err = lighter.NewSigner(httpClient, cfg.APIPrivateKey, cfg.AccountIndex, cfg.APIKeyIndex, cfg.ChainID)
		if err != nil {
			log.Error("signer", "err", err)
			os.Exit(1)
		}
		signer.Meta = markets
		httpClient.SetTokenSource(func(context.Context) (string, error) {
			return signer.AuthToken()
		})
	} else {
		log.Warn("running without signer (DRY_RUN)")
	}

	eng := engine.New(cfg, st, httpClient, signer, markets, notify.Nop{}, rg, log)

	var tg *telegram.Bot
	if cfg.TelegramToken != "" {
		tg = telegram.New(cfg.TelegramToken, cfg.TelegramChatIDs, eng, cfg.TelegramHeartbeat, log)
		eng.Notify = notify.Multi{List: []notify.Notifier{tg, notify.FuncNotifier{F: func(level, kind, message string) {
			log.Info("alert", "level", level, "kind", kind, "msg", message)
		}}}}
	} else {
		eng.Notify = notify.FuncNotifier{F: func(level, kind, message string) {
			log.Info("alert", "level", level, "kind", kind, "msg", message)
		}}
	}

	if err := eng.Bootstrap(ctx); err != nil {
		log.Error("bootstrap", "err", err)
		os.Exit(1)
	}

	idToSym := map[uint8]string{}
	for s, m := range markets {
		idToSym[m.MarketID] = s
	}
	ws := lighter.NewWS(cfg.WSURL, idToSym)
	ws.OnCandle = eng.OnLiveCandle
	ws.OnState = eng.SetWS

	go func() {
		if err := ws.Run(ctx); err != nil && ctx.Err() == nil {
			log.Error("ws", "err", err)
		}
	}()
	go func() {
		t := time.NewTicker(cfg.ReconcileEvery)
		defer t.Stop()
		for {
			select {
			case <-ctx.Done():
				return
			case <-t.C:
				eng.PollClosedBars(ctx)
				eng.Reconcile(ctx)
			}
		}
	}()
	if tg != nil {
		go tg.Run(ctx)
	}

	srv := api.New(cfg.HTTPAddr, cfg.DashboardPassword, cfg.CookieSecure, eng, st)
	go func() {
		log.Info("http listen", "addr", cfg.HTTPAddr, "network", cfg.Network)
		if err := srv.ListenAndServe(); err != nil {
			log.Error("http", "err", err)
			cancel()
		}
	}()

	<-ctx.Done()
	log.Info("shutdown")
}

func pingMarkets(ctx context.Context, httpClient *lighter.HTTPClient, cfg *config.Config, into map[string]lighter.MarketMeta, log *slog.Logger) error {
	resolved, err := httpClient.ResolveMarkets(ctx, cfg.Symbols)
	if err != nil {
		return fmt.Errorf("resolve markets: %w", err)
	}
	for k, v := range resolved {
		into[k] = v
		log.Info("market", "symbol", k, "id", v.MarketID, "size_decimals", v.SizeDecimals, "price_decimals", v.PriceDecimals)
	}
	return nil
}
