package telegram

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"log/slog"
	"net/http"
	"strconv"
	"strings"
	"sync"
	"time"

	"donchian.trade/bot/internal/engine"
	"donchian.trade/bot/internal/telemetry"
)

type Bot struct {
	Token    string
	Allowed  map[int64]bool
	Engine   *engine.Engine
	Log      *slog.Logger
	HTTP     *http.Client
	offset   int
	mu       sync.Mutex
	lastSend time.Time
	hbEvery  time.Duration
}

func New(token string, chatIDs []int64, eng *engine.Engine, hb time.Duration, log *slog.Logger) *Bot {
	allow := map[int64]bool{}
	for _, id := range chatIDs {
		allow[id] = true
	}
	if log == nil {
		log = slog.Default()
	}
	return &Bot{
		Token: token, Allowed: allow, Engine: eng, Log: log,
		HTTP: &http.Client{Timeout: 35 * time.Second}, hbEvery: hb,
	}
}

func (b *Bot) Alert(_ context.Context, level, kind, message string) {
	if b.Token == "" {
		return
	}
	text := fmt.Sprintf("[%s/%s]\n%s", strings.ToUpper(level), kind, message)
	for id := range b.Allowed {
		if err := b.send(id, text); err != nil {
			b.Log.Warn("telegram send", "err", err)
		}
	}
}

func (b *Bot) Run(ctx context.Context) {
	if b.Token == "" {
		b.Log.Info("telegram disabled (no token)")
		return
	}
	b.Alert(ctx, "info", "boot", "Donchian bot online")
	go b.heartbeat(ctx)
	for {
		if ctx.Err() != nil {
			return
		}
		if err := b.poll(ctx); err != nil && ctx.Err() == nil {
			b.Log.Warn("telegram poll", "err", err)
			time.Sleep(3 * time.Second)
		}
	}
}

func (b *Bot) heartbeat(ctx context.Context) {
	if b.hbEvery <= 0 {
		return
	}
	t := time.NewTicker(time.Minute)
	defer t.Stop()
	for {
		select {
		case <-ctx.Done():
			return
		case <-t.C:
			b.mu.Lock()
			silent := time.Since(b.lastSend)
			b.mu.Unlock()
			if silent >= b.hbEvery {
				ok, detail := b.Engine.Health()
				b.Alert(ctx, "info", "heartbeat", fmt.Sprintf("alive health=%v (%s)", ok, detail))
			}
		}
	}
}

type tgResp struct {
	OK     bool            `json:"ok"`
	Result json.RawMessage `json:"result"`
	Desc   string          `json:"description"`
}

type update struct {
	UpdateID int `json:"update_id"`
	Message  *struct {
		Chat struct {
			ID int64 `json:"id"`
		} `json:"chat"`
		Text string `json:"text"`
		From struct {
			ID int64 `json:"id"`
		} `json:"from"`
	} `json:"message"`
}

func (b *Bot) poll(ctx context.Context) error {
	u := fmt.Sprintf("https://api.telegram.org/bot%s/getUpdates?timeout=25&offset=%d", b.Token, b.offset)
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, u, nil)
	if err != nil {
		return err
	}
	resp, err := b.HTTP.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()
	body, _ := io.ReadAll(resp.Body)
	var wrap tgResp
	if err := json.Unmarshal(body, &wrap); err != nil {
		return err
	}
	if !wrap.OK {
		return fmt.Errorf("telegram: %s", wrap.Desc)
	}
	var ups []update
	if err := json.Unmarshal(wrap.Result, &ups); err != nil {
		return err
	}
	for _, up := range ups {
		b.offset = up.UpdateID + 1
		if up.Message == nil {
			continue
		}
		chat := up.Message.Chat.ID
		if !b.Allowed[chat] && !b.Allowed[up.Message.From.ID] {
			_ = b.send(chat, "unauthorized")
			continue
		}
		b.handle(ctx, chat, strings.TrimSpace(up.Message.Text))
	}
	return nil
}

func (b *Bot) handle(ctx context.Context, chat int64, text string) {
	cmd := strings.Split(text, "@")[0]
	switch strings.ToLower(strings.TrimSpace(cmd)) {
	case "/start", "/help":
		_ = b.send(chat, "Команды:\n/health — проверка\n/status — снимок\n/report — Live vs Shadow §17\n/kill — аварийно закрыть всё\n/resume — снять kill-switch")
	case "/health":
		ok, detail := b.Engine.Health()
		_ = b.send(chat, fmt.Sprintf("health=%v\n%s", ok, detail))
	case "/status":
		sn := b.Engine.Snapshot(ctx)
		var sb strings.Builder
		ratio := "NA"
		if sn.PnLRatioOK {
			ratio = fmt.Sprintf("%.2f", sn.PnLRatioXF)
		}
		fmt.Fprintf(&sb, "net=%s kill=%v dry=%v ws=%v\nverdict %s  A=%s B=%s C=%s\nlive=%.2f  shadow_ls5=%.2f  gap=%+.2f\ncash_base=%.2f last_cash=%s %+.2f\nratio_xf=%s  match=%.3f  slip_med=%.1fbps\n",
			sn.Network, sn.KillSwitch, sn.DryRun, sn.WSConnected,
			sn.Verdict, sn.StatusA, sn.StatusB, sn.StatusC,
			sn.Equity, sn.EquityShadowLS5, sn.GapUSD,
			sn.CashBase, sn.LastCashKind, sn.LastCashAmount,
			ratio, sn.MatchRateLTD, sn.SideSlipMedianBps)
		for _, s := range sn.Symbols {
			fmt.Fprintf(&sb, "%s live=%s sh=%s px=%.2f losses=%d/%d\n", s.Symbol, s.Position, s.ShadowLS5, s.LastPrice, s.ConsecLosses, s.ShadowConsecLS5)
		}
		_ = b.send(chat, sb.String())
	case "/report":
		rep := b.Engine.ComputeReport(ctx)
		_ = b.send(chat, telemetry.FormatDaily(time.Now().UTC().Format("2006-01-02"), rep, ""))
	case "/kill":
		if err := b.Engine.KillAll(ctx); err != nil {
			_ = b.send(chat, "kill error: "+err.Error())
			return
		}
		_ = b.send(chat, "kill-switch ON, positions flattened")
	case "/resume":
		b.Engine.Resume(ctx)
		_ = b.send(chat, "kill-switch OFF")
	default:
		_ = b.send(chat, "unknown. /help")
	}
}

func (b *Bot) send(chat int64, text string) error {
	payload, _ := json.Marshal(map[string]any{
		"chat_id": chat,
		"text":    text,
	})
	u := fmt.Sprintf("https://api.telegram.org/bot%s/sendMessage", b.Token)
	resp, err := b.HTTP.Post(u, "application/json", bytes.NewReader(payload))
	if err != nil {
		return err
	}
	defer resp.Body.Close()
	body, _ := io.ReadAll(resp.Body)
	var wrap tgResp
	_ = json.Unmarshal(body, &wrap)
	if !wrap.OK {
		return fmt.Errorf("sendMessage: %s", wrap.Desc)
	}
	b.mu.Lock()
	b.lastSend = time.Now()
	b.mu.Unlock()
	return nil
}

func ParseIDs(s string) []int64 {
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
