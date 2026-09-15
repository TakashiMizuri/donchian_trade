package telegram

import (
	"fmt"
	"html"
	"strings"
	"time"

	"donchian.trade/bot/internal/engine"
	"donchian.trade/bot/internal/store"
	"donchian.trade/bot/internal/telemetry"
)

var kindTitle = map[string]string{
	"boot":      "Запуск",
	"heartbeat": "Пульс",
	"entry":     "Вход",
	"exit":      "Выход",
	"sl":        "Стоп по ATR",
	"stop":      "Стоп-ордер",
	"kill":      "Kill-switch",
	"verdict":   "Вердикт",
	"ws":        "Стрим",
	"cash":      "Касса",
	"daily":     "Суточный отчёт",
	"replay":    "Сверка тени",
	"risk":      "Риск",
	"reconcile": "Сверка с биржей",
	"flatten":   "Flatten",
	"watchdog":  "Сторож стопа",
	"db":        "База",
}

func formatAlert(level, kind, message string) string {
	title := kindTitle[kind]
	if title == "" {
		title = kind
	}
	head := "<b>" + html.EscapeString(title) + "</b>"
	switch level {
	case "error":
		head += " · ошибка"
	case "warn":
		head += " · внимание"
	}
	msg := strings.TrimSpace(message)
	if msg == "" {
		return head
	}
	return head + "\n" + html.EscapeString(msg)
}

func formatHelp() string {
	return strings.Join([]string{
		"<b>Donchian</b> — команды",
		"",
		"/status — снимок счёта и позиций",
		"/trades — последние 10 Live (открытые и закрытые)",
		"/report — тот же суточный отчёт, что приходит сам",
		"/health — жив ли бот",
		"/kill — закрыть Live и запретить входы",
		"/resume — снять kill-switch",
		"",
		"Раз в сутки после закрытия 23:00 UTC приходит отчёт с графиком.",
	}, "\n")
}

func formatHealth(ok bool, detail string) string {
	state := "ок"
	if !ok {
		state = "не готов"
	}
	return "<b>Здоровье</b> · " + state + "\n" + html.EscapeString(detail)
}

func formatStatus(sn engine.Snapshot) string {
	var b strings.Builder
	v := verdictRU(sn.Verdict)
	if sn.KillSwitch {
		v = "стоп (kill-switch)"
	}
	fmt.Fprintf(&b, "<b>Снимок</b> · %s\n", html.EscapeString(v))
	fmt.Fprintf(&b, "%s · %s\n", html.EscapeString(netRU(sn.Network, sn.DryRun)), html.EscapeString(sn.Strategy))
	fmt.Fprintf(&b, "стрим: %s\n", html.EscapeString(wsRU(sn.WSConnected, sn.WSError)))
	b.WriteString("\n")
	fmt.Fprintf(&b, "<b>Live</b>  %s\n", usd(sn.Equity))
	fmt.Fprintf(&b, "<b>Тень</b>  %s\n", usd(sn.EquityShadowLS5))
	fmt.Fprintf(&b, "разрыв %s (%s)\n", usd(sn.GapUSD), pct(sn.GapPct))
	fmt.Fprintf(&b, "совпало %d из %d · неделя %s\n", sn.Report.NMatched, sn.Report.NUnion, pct(sn.MatchRate7d))
	fmt.Fprintf(&b, "входы %s · исполнение %s · PnL %s\n", checkRU(sn.StatusA), checkRU(sn.StatusB), checkRU(sn.StatusC))
	fmt.Fprintf(&b, "slip %s · день %s (не стоп)\n", bps(sn.SideSlipMedianBps), usd(sn.DailyPnL))
	b.WriteString("\n")
	for _, s := range sn.Symbols {
		fmt.Fprintf(&b, "<b>%s</b>  %s  %s\n", html.EscapeString(s.Symbol), sideRU(s.Position), usd(s.LastPrice))
		if s.Position != "FLAT" && s.Position != "" {
			fmt.Fprintf(&b, "  qty %s · вход %s · стоп %s\n", trimFloat(s.Qty), usd(s.Entry), usd(s.Stop))
		}
		fmt.Fprintf(&b, "  тень %s · убытки %d/%d", sideRU(s.ShadowLS5), s.ConsecLosses, s.ShadowConsecLS5)
		if s.Paused {
			b.WriteString(" · пауза")
		}
		b.WriteString("\n")
	}
	return strings.TrimSpace(b.String())
}

func formatTrades(rows []store.TradeRow, limit int) string {
	live := make([]store.TradeRow, 0, limit)
	for _, t := range rows {
		if t.Profile != telemetry.BookLive {
			continue
		}
		live = append(live, t)
		if len(live) >= limit {
			break
		}
	}
	if len(live) == 0 {
		return "<b>Сделки Live</b>\nПока пусто. Donchian на часе редко входит."
	}
	var b strings.Builder
	fmt.Fprintf(&b, "<b>Последние %d Live</b>\n", len(live))
	for i, t := range live {
		if i > 0 {
			b.WriteString("\n")
		}
		b.WriteString(formatTradeLine(t))
	}
	return b.String()
}

func formatTradeLine(t store.TradeRow) string {
	entry := t.EntryPxLive
	if entry == 0 {
		entry = t.EntryPrice.Float64
	}
	exit := t.ExitPxLive
	if exit == 0 {
		exit = t.ExitPrice.Float64
	}
	when := unixRU(t.EntryTime.Int64)
	if when == "—" {
		when = unixRU(t.SignalTime)
	}
	out := outcomeRU(t.Outcome.String)
	line := fmt.Sprintf("<b>%s</b> %s · %s\n%s → %s",
		html.EscapeString(t.Symbol), sideRU(t.Direction), html.EscapeString(out),
		usd(entry), dashIf(exit))
	if t.Outcome.String != "open" && t.Outcome.String != "" {
		line += "\nnet " + usd(t.Net.Float64)
	}
	line += "\n" + when
	return line
}

func formatDailyHTML(date string, sn engine.Snapshot, trades []store.TradeRow) string {
	r := sn.Report
	var b strings.Builder
	fmt.Fprintf(&b, "<b>Суточный отчёт</b> · %s UTC\n", html.EscapeString(date))
	fmt.Fprintf(&b, "%s\n", html.EscapeString(verdictRU(sn.Verdict)))
	fmt.Fprintf(&b, "входы %s · исполнение %s · PnL %s\n\n", checkRU(r.StatusA), checkRU(r.StatusB), checkRU(r.StatusC))
	fmt.Fprintf(&b, "<b>Live</b>  %s  (без funding %s)\n", usd(r.EquityLive), usd(r.EquityLiveExFunding))
	fmt.Fprintf(&b, "<b>Тень ls5</b>  %s\n", usd(r.EquityShadowLS5))
	fmt.Fprintf(&b, "без паузы  %s\n", usd(r.EquityShadowBaseline))
	fmt.Fprintf(&b, "разрыв %s (%s)\n", usd(r.GapUSD), pct(r.GapPct))
	fmt.Fprintf(&b, "просадка Live %s · тень %s\n\n", pct(r.DDLive), pct(r.DDShadowLS5))
	fmt.Fprintf(&b, "совпало %d из %d за всё · неделя %s\n", r.NMatched, r.NUnion, pct(r.MatchRate7d))
	fmt.Fprintf(&b, "только Live/тень за неделю %d/%d\n", r.LiveOnly7d, r.ShadowOnly7d)
	fmt.Fprintf(&b, "slip медиана %s · p90 %s · выход по стопу %s\n", bps(r.SideSlipMedianBps), bps(r.SideSlipP90Bps), bps(r.SLExitSlipMedianBps))
	ratio := "рано"
	if r.PnLRatioOK {
		ratio = fmt.Sprintf("%.2f", r.PnLRatioXF)
	}
	fmt.Fprintf(&b, "Live net %s · без funding %s · тень %s\n", usd(r.LiveNet), usd(r.LiveNetXF), usd(r.ShadowNet))
	fmt.Fprintf(&b, "funding %s · Live/тень %s\n", usd(r.Funding), ratio)
	fmt.Fprintf(&b, "день %s — не стоп-сигнал\n\n", usd(sn.DailyPnL))

	b.WriteString("<b>Позиции</b>\n")
	if len(sn.Symbols) == 0 {
		b.WriteString("нет рынков\n")
	}
	for _, s := range sn.Symbols {
		fmt.Fprintf(&b, "%s Live %s · тень %s · %s\n", html.EscapeString(s.Symbol), sideRU(s.Position), sideRU(s.ShadowLS5), usd(s.LastPrice))
		if s.Qty > 0 {
			fmt.Fprintf(&b, "  qty %s · вход %s · стоп %s\n", trimFloat(s.Qty), usd(s.Entry), usd(s.Stop))
		}
	}

	today := tradesOnUTCDate(trades, date)
	b.WriteString("\n<b>Сделки Live за день</b>\n")
	if len(today) == 0 {
		b.WriteString("закрытых и новых входов не было\n")
	} else {
		for _, t := range today {
			b.WriteString(formatTradeLine(t))
			b.WriteString("\n")
		}
	}
	b.WriteString("\nДень в минусе сам по себе ничего не стопает.")
	return strings.TrimSpace(b.String())
}

func dailyCaption(date, verdict string) string {
	return "Суточный отчёт " + date + " UTC · " + verdictRU(verdict)
}

func tradesOnUTCDate(rows []store.TradeRow, date string) []store.TradeRow {
	var out []store.TradeRow
	for _, t := range rows {
		if t.Profile != telemetry.BookLive {
			continue
		}
		ts := t.ExitTime.Int64
		if ts == 0 {
			ts = t.EntryTime.Int64
		}
		if ts == 0 {
			ts = t.SignalTime
		}
		if time.Unix(ts, 0).UTC().Format("2006-01-02") == date {
			out = append(out, t)
		}
	}
	return out
}

func verdictRU(v string) string {
	switch v {
	case telemetry.VerdictSTOP:
		return "стоп"
	case telemetry.VerdictInvestigate:
		return "нужно разобрать"
	default:
		return "всё сходится"
	}
}

func checkRU(v string) string {
	switch v {
	case "green":
		return "норма"
	case "yellow":
		return "смотреть"
	case "orange":
		return "плохо"
	case "red":
		return "стоп"
	default:
		return "рано"
	}
}

func sideRU(v string) string {
	switch v {
	case "BUY", "LONG":
		return "лонг"
	case "SELL", "SHORT":
		return "шорт"
	case "FLAT", "":
		return "нет позиции"
	default:
		return v
	}
}

func outcomeRU(v string) string {
	switch v {
	case "", "open":
		return "открыта"
	case "sl":
		return "стоп"
	case "time":
		return "выход по каналу"
	case "kill":
		return "kill-switch"
	case "watchdog":
		return "сторож"
	case "rejected":
		return "биржа отказала"
	default:
		return v
	}
}

func netRU(network string, dry bool) string {
	n := "тестнет"
	if network == "mainnet" {
		n = "mainnet"
	}
	if dry {
		return n + " · без ордеров"
	}
	return n
}

func wsRU(ok bool, err string) string {
	if ok {
		return "идёт"
	}
	if err != "" {
		return "молчит, REST ещё торгует (" + err + ")"
	}
	return "молчит, REST ещё торгует"
}

func usd(n float64) string {
	sign := ""
	if n < 0 {
		sign = "−"
		n = -n
	}
	s := fmt.Sprintf("$%.2f", n)
	return sign + s
}

func pct(n float64) string {
	return fmt.Sprintf("%.1f%%", n*100)
}

func bps(n float64) string {
	return fmt.Sprintf("%.1f б.п.", n)
}

func dashIf(n float64) string {
	if n == 0 {
		return "…"
	}
	return usd(n)
}

func unixRU(sec int64) string {
	if sec == 0 {
		return "—"
	}
	return time.Unix(sec, 0).UTC().Format("02.01 15:04 UTC")
}

func trimFloat(n float64) string {
	s := fmt.Sprintf("%.6f", n)
	s = strings.TrimRight(s, "0")
	s = strings.TrimRight(s, ".")
	return s
}
