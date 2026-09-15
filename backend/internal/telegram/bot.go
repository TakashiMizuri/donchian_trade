package telegram

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"log/slog"
	"mime/multipart"
	"net/http"
	"strconv"
	"strings"
	"sync"
	"time"

	"donchian.trade/bot/internal/engine"
	"donchian.trade/bot/internal/notify"
	"donchian.trade/bot/internal/plot"
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
		HTTP: &http.Client{Timeout: 45 * time.Second}, hbEvery: hb,
	}
}

func (b *Bot) Alert(_ context.Context, level, kind, message string) {
	if b.Token == "" || kind == "daily" {
		return
	}
	text := formatAlert(level, kind, message)
	b.broadcastHTML(text)
}

func (b *Bot) Report(ctx context.Context, mail notify.ReportMail) {
	if b.Token == "" {
		return
	}
	date := mail.Date
	if date == "" {
		date = time.Now().UTC().Format("2006-01-02")
	}
	sn := b.Engine.Snapshot(ctx)
	rows, _ := b.Engine.Store.ListTrades(ctx, 80)
	html := mail.HTML
	if html == "" {
		html = formatDailyHTML(date, sn, rows)
	}
	caption := dailyCaption(date, sn.Verdict)
	png := mail.PNG
	if len(png) == 0 {
		png = b.equityPNG(ctx)
	}
	for id := range b.Allowed {
		if len(png) > 0 {
			if err := b.sendPhoto(id, png, caption); err != nil {
				b.Log.Warn("telegram photo", "err", err)
			}
		}
		if html == "" {
			continue
		}
		if err := b.sendHTML(id, html); err != nil {
			b.Log.Warn("telegram report", "err", err)
		}
	}
}

func (b *Bot) broadcastHTML(text string) {
	for id := range b.Allowed {
		if err := b.sendHTML(id, text); err != nil {
			b.Log.Warn("telegram send", "err", err)
		}
	}
}

func (b *Bot) Run(ctx context.Context) {
	if b.Token == "" {
		b.Log.Info("telegram disabled (no token)")
		return
	}
	b.Alert(ctx, "info", "boot", "Donchian онлайн")
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
				b.Alert(ctx, "info", "heartbeat", fmt.Sprintf("жив, здоровье=%v (%s)", ok, detail))
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
			_ = b.sendHTML(chat, "Этот чат не в списке. Напишите свой chat id в TELEGRAM_CHAT_IDS.")
			continue
		}
		b.handle(ctx, chat, strings.TrimSpace(up.Message.Text))
	}
	return nil
}

func (b *Bot) handle(ctx context.Context, chat int64, text string) {
	cmd := strings.ToLower(strings.TrimSpace(strings.Split(text, "@")[0]))
	switch cmd {
	case "/start", "/help":
		_ = b.sendHTML(chat, formatHelp())
	case "/health":
		ok, detail := b.Engine.Health()
		_ = b.sendHTML(chat, formatHealth(ok, detail))
	case "/status":
		_ = b.sendHTML(chat, formatStatus(b.Engine.Snapshot(ctx)))
	case "/trades":
		rows, err := b.Engine.Store.ListTrades(ctx, 40)
		if err != nil {
			_ = b.sendHTML(chat, "Не удалось прочитать сделки: "+htmlEsc(err.Error()))
			return
		}
		_ = b.sendHTML(chat, formatTrades(rows, 10))
	case "/report":
		b.sendDaily(ctx, chat, time.Now().UTC().Format("2006-01-02"))
	case "/kill":
		if err := b.Engine.KillAll(ctx); err != nil {
			_ = b.sendHTML(chat, "<b>Kill-switch</b> · ошибка\n"+htmlEsc(err.Error()))
			return
		}
		_ = b.sendHTML(chat, "<b>Kill-switch</b>\nLive закрыт, новые входы запрещены. Тень считает дальше.")
	case "/resume":
		b.Engine.Resume(ctx)
		_ = b.sendHTML(chat, "<b>Kill-switch снят</b>\nНовые входы снова разрешены.")
	default:
		_ = b.sendHTML(chat, "Не знаю эту команду. /help")
	}
}

func (b *Bot) sendDaily(ctx context.Context, chat int64, date string) {
	sn := b.Engine.Snapshot(ctx)
	rows, _ := b.Engine.Store.ListTrades(ctx, 80)
	html := formatDailyHTML(date, sn, rows)
	caption := dailyCaption(date, sn.Verdict)
	if png := b.equityPNG(ctx); len(png) > 0 {
		if err := b.sendPhoto(chat, png, caption); err != nil {
			b.Log.Warn("telegram photo", "err", err)
		}
	}
	if err := b.sendHTML(chat, html); err != nil {
		b.Log.Warn("telegram report", "err", err)
	}
}

func (b *Bot) equityPNG(ctx context.Context) []byte {
	curve, err := b.Engine.Store.ListCurve(ctx, 2000)
	if err != nil || len(curve) < 2 {
		return nil
	}
	live := make([]float64, len(curve))
	sh := make([]float64, len(curve))
	for i, p := range curve {
		live[i] = p.EquityLive
		sh[i] = p.EquityShadowLS5
	}
	png, err := plot.EquityPNG(live, sh)
	if err != nil {
		return nil
	}
	return png
}

func (b *Bot) sendHTML(chat int64, text string) error {
	for _, chunk := range splitTelegram(text, 3500) {
		if err := b.sendChunk(chat, chunk); err != nil {
			return err
		}
	}
	return nil
}

func (b *Bot) sendChunk(chat int64, text string) error {
	payload, _ := json.Marshal(map[string]any{
		"chat_id":                  chat,
		"text":                     text,
		"parse_mode":               "HTML",
		"disable_web_page_preview": true,
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
	b.touch()
	return nil
}

func (b *Bot) sendPhoto(chat int64, png []byte, caption string) error {
	if len(caption) > 1000 {
		caption = caption[:1000]
	}
	var buf bytes.Buffer
	w := multipart.NewWriter(&buf)
	_ = w.WriteField("chat_id", strconv.FormatInt(chat, 10))
	_ = w.WriteField("caption", caption)
	_ = w.WriteField("parse_mode", "HTML")
	fw, err := w.CreateFormFile("photo", "equity.png")
	if err != nil {
		return err
	}
	if _, err := fw.Write(png); err != nil {
		return err
	}
	if err := w.Close(); err != nil {
		return err
	}
	u := fmt.Sprintf("https://api.telegram.org/bot%s/sendPhoto", b.Token)
	req, err := http.NewRequest(http.MethodPost, u, &buf)
	if err != nil {
		return err
	}
	req.Header.Set("Content-Type", w.FormDataContentType())
	resp, err := b.HTTP.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()
	body, _ := io.ReadAll(resp.Body)
	var wrap tgResp
	_ = json.Unmarshal(body, &wrap)
	if !wrap.OK {
		return fmt.Errorf("sendPhoto: %s", wrap.Desc)
	}
	b.touch()
	return nil
}

func (b *Bot) touch() {
	b.mu.Lock()
	b.lastSend = time.Now()
	b.mu.Unlock()
}

func splitTelegram(s string, max int) []string {
	s = strings.TrimSpace(s)
	if len(s) <= max {
		return []string{s}
	}
	var out []string
	for len(s) > max {
		cut := strings.LastIndex(s[:max], "\n")
		if cut < max/2 {
			cut = max
		}
		out = append(out, strings.TrimSpace(s[:cut]))
		s = strings.TrimSpace(s[cut:])
	}
	if s != "" {
		out = append(out, s)
	}
	return out
}

func htmlEsc(s string) string {
	return strings.NewReplacer("&", "&amp;", "<", "&lt;", ">", "&gt;").Replace(s)
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
