# Ускорение детекта закрытия свечи (candle-close latency)

**Дата:** 2026-10-03  
**Контекст:** live 15m `brk0.5+vol_rank` на Lighter. На проде `detect_lag_ms` на входах был **4–8 с** при том, что от `entry_intent` до `entry_sent` уходило **~3–4 мс**.

## Проблема

Бот не «думал» 8 секунд. Он **ждал уведомление о закрытом баре**.

У Lighter канал `candle/{market}/{resolution}`:

- апдейты батчатся ~**500 ms**, но приходят **когда есть сделки**;
- в тихом рынке после границы бара (`:00/:15/:30/:45`) следующий trade может прийти через несколько секунд → close «молчит»;
- наш WS раньше эмитил close **только** при dual-candle payload (`candles[0]` = closed, `candles[1]` = live). Если биржа слала один новый бар с новым `t`, close не срабатывал, пока не придёт dual или REST раз в 15 с.

На 15m это даёт лишние bps side/entry slip относительно shadow fill на open.

## Что внедрили

### A. WS: close при `t`↑ (как Nautilus)

Файл: `backend/internal/exchange/lighter/ws.go`

Кешируем последний live-бар по `market_id`. Если приходит **одна** свеча с `t` больше предыдущего — эмитим закешированный бар как **closed**, новый как live. Dual-candle rollover по-прежнему поддерживается. `HandleClosed` идемпотентен (повтор того же `bar.Time` no-op).

### B. Boundary REST poll

Файл: `backend/internal/engine/engine.go` → `RunBoundaryPoll`  
Старт: `backend/cmd/bot/main.go`

После каждой UTC-границы TF дергаем `PollClosedBars` на смещениях:

- +200 ms  
- +500 ms  
- +1 s  
- +2 s  

Не ждём trade-driven WS. В jsonl пишется `kind=boundary_poll`. Старый тикер reconcile **15 s** остаётся страховкой + account sync.

## Ожидание

- Типичный `detect_lag_ms`: с **4–8 s** → часто **&lt;1–2 s** (зависит от того, когда REST отдаст закрытый бар / когда придёт первый trade нового бара).
- Полный ноль не обещаем: сеть, REST RTT, impact на IOC остаются.
- Signing/IOC path не меняли — он уже был миллисекундный.

## Как проверить на проде

```bash
# после деплоя, на закрытии бара с входом:
grep '"kind":"entry_intent"' /data/logs/bot-*.jsonl | tail
# смотреть detect_lag_ms

grep '"kind":"boundary_poll"' /data/logs/bot-*.jsonl | tail
```

Сравнить median/p90 `detect_lag_ms` и `entry_slip_bps` с дампом `prod_15m_brk_vol` (до патча).

## Вне скоупа

- Сборка OHLC из raw trade tape (максимально рано, но риск расхождения с «официальной» свечой research).
- Смена Lighter order logic / colocation.
