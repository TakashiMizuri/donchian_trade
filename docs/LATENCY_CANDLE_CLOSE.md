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

### B. Boundary REST poll-until-seen

Файл: `backend/internal/engine/engine.go` → `RunBoundaryPoll`  
Старт: `backend/cmd/bot/main.go`

После каждой UTC-границы TF:

1. Сразу начинаем опрос REST.
2. Повторяем **каждые 200 ms**, пока у **всех** символов не появится закрытый бар (`lastBar >= expected`), либо до таймаута **5 s**.
3. Если WS успел раньше — `all_seen=true` на первой же попытке, опрос останавливается (`HandleClosed` идемпотентен).

В jsonl:

- `kind=boundary_poll` — `attempt`, `offset_ms`, `all_seen`
- `kind=bar_closed` / `entry_intent` — поле `source`: `ws` | `boundary` | `reconcile`

Старый тикер reconcile **15 s** остаётся страховкой + account sync (не основной путь детекта).

## Ожидание

- `detect_lag_ms` стабильно в зоне **&lt;1–2 s**, когда REST уже отдаёт закрытый бар; без провала на ~8 s из-за 15s ticker.
- Если биржа отдаёт свечу поздно — лаг ≈ задержка API (не больше timeout 5 s на этом пути).
- Полный ноль не обещаем: сеть, REST RTT, impact на IOC остаются.
- Signing/IOC path не меняли — он уже был миллисекундный.

## Как проверить на проде

```bash
# после деплоя, на закрытии бара с входом:
grep '"kind":"entry_intent"' /data/logs/bot-*.jsonl | tail
# смотреть detect_lag_ms и source

grep '"kind":"bar_closed"' /data/logs/bot-*.jsonl | tail
grep '"kind":"boundary_poll"' /data/logs/bot-*.jsonl | tail
```

Сравнить median/p90 `detect_lag_ms` с дампом `prod_5m_tstop20` (до poll-until-seen: часто ~8 s).

## Вне скоупа

- Сборка OHLC из raw trade tape (максимально рано, но риск расхождения с «официальной» свечой research).
- Смена Lighter order logic / colocation.
