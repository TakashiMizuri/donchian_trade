# Donchian live start-pack

**Назначение:** самодостаточная спецификация, чтобы с нуля поднять автоторговый бот в другом репозитории и повторить research-логику 1:1.

**Источник research:** репозиторий `new-algotrading-adventure` (заморозка 2026-09).  
**Канонический тег:** `1h_N30_M15_ATR1.5`  
**Рекомендуемый live-профиль:** core + alternate **`ls5_cond_brk2.0`** (см. §8).  
**Live vs backtest (статистика с дня 1, пороги WAIT/STOP):** **§17**.

---

## 0. Коротко: что это за стратегия

Классический **Donchian / Turtle-style breakout** на крипто-перпетуалах:

- Торгуем **пробой** N-периодного канала Дончиана.
- Выходим по **ATR-стопу** или по **обратному пробою** M-канала.
- Один слот позиции (без пирамидинга).
- Сайзинг: фиксированный **% equity** с долларовым **cap**.
- Ожидаемый профиль: **низкий winrate (~25–30%)**, прибыль от **редких крупных runners**; серии мелких −R — нормальная цена edge, не баг.

Это **не** mean-reversion и не MSS. Не пытайтесь «починить» луз-стрики фильтрами ATR/ADX на входе — research это отверг.

---

## 1. Рекомендуемая конфигурация для бота

### 1.1 Primary core (эталон сигналов)

| Параметр | Значение | Комментарий |
|----------|----------|-------------|
| Инструменты | `BTCUSDT`, `ETHUSDT` (USD-M perpetual) | Spot не подходит (нужен short) |
| Таймфрейм | **1h** | `bar_seconds = 3600` |
| Donchian entry `N` | **30** | lookback для канала входа |
| Donchian exit `M` | **15** | lookback для канала выхода; `M < N` |
| ATR period | **20** | см. §4 — это **не** Wilder RMA |
| ATR stop mult | **1.5** | стоп = `1.5 × ATR` на signal-баре |
| Max positions | **1** | нет пирамиды, нет хеджа в research |
| Risk % | **1.0** | от equity **на момент входа** |
| Max risk USD | **1000** | cap; при equity > ~$100k режет compounding |
| Fee model (research) | taker **0.05%** / сторону | Binance USD-M VIP0, без BNB |
| Slippage (research) | **0%** (опционально 0.01–0.02% live stress) | |
| Funding | **не моделировался** | в live учесть отдельно |
| Invert after loss | **OFF** | |
| Adaptive risk | **OFF** | |
| Min breakout ATR | **0** (off) | |
| Regime filters (ADX/ATR%/channel) | **OFF** | отвергнуты |

### 1.2 Live-профиль (рекомендация research)

| Режим | Когда использовать |
|-------|-------------------|
| **A. Shadow / paper baseline** | Всегда гонять параллельно как эталон сигналов |
| **B. Live default = `ls5_cond_brk2.0`** | Тот же core + пауза после 5 gross-лоссов (§8) |
| **C. Не включать** | Equity DD-pause, streak-sizing, ADX gates, invert+adaptive как default |

Параметры `ls5_cond_brk2.0`:

| Параметр | Значение |
|----------|----------|
| `loss_streak_n` | **5** |
| `loss_streak_pause_bars` | **24** (т.е. до 24 часов на 1h) |
| `cooldown_resume_min_breakout_atr` | **2.0** |

Смысл: после 5 подряд **gross**-убытков не входим до `exit_bar + 24`, **кроме** случая, когда на signal-баре close пробивает N-канал минимум на **2.0 × ATR** — тогда пауза снимается раньше.

---

## 2. Рынок и данные

### 2.1 Инструмент

- **Binance USD-M Futures** perpetual: `BTCUSDT`, `ETHUSDT`.
- Research klines исторически брались из **Binance Vision spot 1h** и агрегировались/использовались как OHLC для сигналов. Для live логичнее стримить **futures 1h klines** того же символа (цены близки, но не идентичны spot — заложите возможный drift при сверке с backtest).
- Листинг Binance: данные примерно с **2017-08-17**; **2016 недоступен**.

### 2.2 Свечи

На каждом закрытом баре `i` (0-based):

| Поле | Смысл |
|------|--------|
| `time[i]` | open time бара (Unix sec) |
| `open/high/low/close[i]` | OHLC |

**Критично — причинность (no lookahead):**

- Канал входа на баре `i` строится по **предыдущим** барам: `high/low` в диапазоне **`[i-N, i)`** (бар `i` **не** входит в канал).
- Канал выхода: **`[i-M, i)`**.
- Сигнал смотрится по **`close[i]`**.
- Исполнение входа — по **`open[i+1]`** (следующий бар).
- После входа на баре `entry_idx` тот же бар **не** используется для немедленного re-entry после выхода (см. цикл: после закрытия `i += 1`, вход ставит `i = entry_idx` и идёт дальше).

### 2.3 Warmup

Перед первым сигналом нужно минимум:

```text
warm = max(N, M, atr_period) = max(30, 15, 20) = 30
```

баров истории **до** начала торговли. На практике держите ≥ **120 дней** 1h истории в буфере бота (как в research warmup), чтобы ATR/канал стабилизировались после рестартов.

---

## 3. Алгоритм бара за баром (state machine)

Состояние бота:

```text
position: null | { direction, entry_price, entry_bar_index, stop, risk_distance, signal_time, id }
consec_gross_losses: int = 0          # только для ls5_cond
pause_until_bar_index: int = -1       # только для ls5_cond
equity: float
```

На **каждом закрытом** баре `i` (пока есть `i+1` для потенциального входа):

### 3.1 Если есть позиция — сначала проверить выход

Пусть `direction ∈ {BUY, SELL}`, `stop`, `entry_idx`.

1. **ATR stop (приоритет выше channel exit):**
   - BUY: если `low[i] <= stop` → выход по цене **`stop`**, outcome=`sl`, время = `time[i]` (open time текущего бара; в backtest fill моделируется ценой стопа на этом баре).
   - SELL: если `high[i] >= stop` → аналогично.
2. Иначе **channel exit** (и опциональный time-stop, в live core — off):
   - `upper_m = max(high[i-M : i])`
   - `lower_m = min(low[i-M : i])`
   - BUY выходит, если `close[i] < lower_m`
   - SELL выходит, если `close[i] > upper_m`
   - Fill: **`open[i+1]`**, время = `time[i+1]`, outcome=`time` (имя историческое; это channel/time exit, не wall-clock).
3. Если сработали и stop, и channel на одном баре — в research **побеждает stop**.

После закрытия:

- Обновить equity (fees + PnL).
- Обновить `consec_gross_losses` / паузу (§8), если включён ls5.
- **Не** открывать новую сделку на том же проходе выхода (следующий бар).

### 3.2 Если позиции нет — проверить вход

Условия:

1. `atr[i] > 0`
2. Если ls5: не в паузе **или** сработал early-resume (§8)
3. `entry_idx = i + 1` существует
4. (В backtest) `eval_from <= time[entry_idx] < eval_to`

Канал входа:

```text
upper_n = max(high[i-N : i])
lower_n = min(low[i-N : i])
c       = close[i]
atr_i   = ATR[i]
```

Сигнал (core, `min_breakout_atr = 0`):

```text
if c > upper_n:          want = BUY
elif c < lower_n:        want = SELL
else:                    want = None
```

Если `min_breakout_atr > 0` (не для default live):

```text
if c > upper_n + min_breakout_atr * atr_i: want = BUY
elif c < lower_n - min_breakout_atr * atr_i: want = SELL
```

Исполнение:

```text
entry_price = open[i+1]
if BUY:
    stop = entry_price - atr_stop_mult * atr_i
    risk_distance = entry_price - stop   # = atr_stop_mult * atr_i
else:
    stop = entry_price + atr_stop_mult * atr_i
    risk_distance = stop - entry_price
```

**Важно:** ATR для стопа берётся с **signal-бара `i`**, а не с entry-бара. Стоп **фиксированный** до выхода (не trailing в v1).

После входа цикл research ставит `i = entry_idx` (начинает управление со бара входа). В live: после fill на open нового часа сразу ставите protective stop-order на бирже.

---

## 4. Индикатор ATR (обязательно совпасть с research)

Research использует **простую скользящую среднюю True Range**, **не** классический Wilder RMA ATR TradingView.

### 4.1 True Range

```text
TR[0] = high[0] - low[0]
TR[i] = max(
  high[i] - low[i],
  abs(high[i] - close[i-1]),
  abs(low[i]  - close[i-1])
)   for i >= 1
```

### 4.2 ATR(period=20)

Для индекса `i`:

```text
start = max(1, i - period + 1)   # окно НИКОГДА не включает TR[0]
ATR[i] = mean(TR[start : i+1])  # среднее по [start .. i] включительно
ATR[0] = high[0] - low[0]
```

Референс-код research: `strategies/mss/atr.py`.

**Parity-тест:** на одном и том же 1h ряду ваш ATR[i] должен совпасть с research с точностью float.

---

## 5. Сайзинг позиции

На момент входа, до списания entry-fee:

```text
raw_risk_usd = equity * (risk_pct / 100.0)     # 1% → equity * 0.01
risk_usd     = min(raw_risk_usd, max_risk_usd) if max_risk_usd > 0 else raw_risk_usd
quantity     = risk_usd / risk_distance         # в монетах (BTC/ETH)
notional     = quantity * entry_price           # USDT
```

Пример: equity=$10_000, risk=1%, ATR-stop distance=$500 → risk_usd=$100 → qty = 100/500 = 0.2.

### 5.1 PnL / fees (research model)

Обе ноги — taker:

```text
cost_rate = fee_rate + slippage_rate   # default 0.0005 + 0
entry_fee = notional * cost_rate
exit_fee  = quantity * exit_price * cost_rate

if outcome == sl:
    # backtest упрощает gross = -risk_usd (ровно 1R), не qty*(exit-entry)
    gross = -risk_usd
else:
    sign = +1 if BUY else -1
    gross = quantity * (exit_price - entry_price) * sign

net = gross - entry_fee - exit_fee
equity := equity - entry_fee + gross - exit_fee
```

**Нюанс SL в backtest:** стоп моделируется как ровно −1R по `risk_usd`, даже если цена стопа чуть иначе из-за округлений. В live будет реальное fill стопа + fee.

### 5.2 Gross vs net для streak-счётчика

Для **`ls5_cond`** consecutive losses считаются по **gross price PnL** сделки:

```text
gross_move = (exit_price - entry_price) * sign
won = gross_move > 0
```

Не по net после комиссий. Это важно для parity с research.

---

## 6. Ордера на live (практика)

Research — идеализированные fills. Рекомендуемая проекция:

| Событие | Research | Live suggestion |
|---------|----------|-----------------|
| Entry | market at next open | на close сигнала: market/IOC на open следующего часа **или** wait bar close + market; зафиксируйте один режим |
| ATR stop | fill exactly at stop | **STOP_MARKET** / stop-limit на бирже сразу после входа |
| Channel exit | next bar open | на close, когда условие channel exit true → market на следующем open (или market на close — хуже parity) |
| Reduce-only | — | exits reduce-only |
| One-way mode | long или short | hedge mode не нужен |

Минимальный набор защиты:

1. Exchange stop сразу после entry.
2. Локальный watchdog: если стоп снят/не висит — аварийный flatten.
3. Не открывать второй вход при `position != flat`.
4. Идемпотентность: один signal_time → один entry attempt.

---

## 7. Псевдокод core (без ls5)

```python
# bars: list of {t, o, h, l, c}, sorted ascending, 1h
N, M, ATR_P, ATR_MULT = 30, 15, 20, 1.5
atr = compute_sma_atr(bars, ATR_P)  # §4

position = None
i = max(N, M, ATR_P)

while i < len(bars) - 1:
    if position is not None:
        d = position.direction
        stop = position.stop
        # 1) stop
        if (d == "BUY" and bars[i].l <= stop) or (d == "SELL" and bars[i].h >= stop):
            close_trade(exit_price=stop, outcome="sl", bar=i)
            position = None
            i += 1
            continue
        # 2) channel exit
        upper_m = max(b.h for b in bars[i-M:i])
        lower_m = min(b.l for b in bars[i-M:i])
        ch = (bars[i].c < lower_m) if d == "BUY" else (bars[i].c > upper_m)
        if ch:
            close_trade(exit_price=bars[i+1].o, outcome="time", bar=i+1)
            position = None
            i += 1
            continue

    if position is None:
        upper_n = max(b.h for b in bars[i-N:i])
        lower_n = min(b.l for b in bars[i-N:i])
        c = bars[i].c
        a = atr[i]
        want = None
        if a > 0 and c > upper_n:
            want = "BUY"
        elif a > 0 and c < lower_n:
            want = "SELL"
        if want:
            entry = bars[i+1].o
            stop = entry - ATR_MULT*a if want == "BUY" else entry + ATR_MULT*a
            risk = abs(entry - stop)
            open_trade(want, entry, stop, risk, signal_bar=i)
            i = i + 1  # = entry_idx; next loop manages from entry bar
            continue
    i += 1
```

---

## 8. Alternate B — `ls5_cond_brk2.0` (полный контракт)

Включается **поверх** core. Сигналы те же; меняется только **разрешение входа**.

### 8.1 При закрытии сделки

```text
sign = +1 if BUY else -1
gross = (exit_price - entry_price) * sign
if gross > 0:
    consec_gross_losses = 0
else:
    consec_gross_losses += 1
    if consec_gross_losses >= 5:
        pause_until_bar_index = max(pause_until_bar_index, exit_bar_index + 24)
        consec_gross_losses = 0   # счётчик сбрасывается после триггера паузы
```

`exit_bar_index`:

- для SL: бар `i`, на котором стоп поймали;
- для channel exit: бар fill `i+1` (как в research `_close(..., exit_bar_i=i+1)`).

### 8.2 Перед входом

Пауза активна, пока `i < pause_until_bar_index`, **если** не сработал early resume:

```text
def resume_ready(i, upper_n, lower_n, c, atr_i):
    if i >= pause_until_bar_index:
        return True
    if atr_i <= 0:
        return False
    # cooldown_resume_min_breakout_atr = 2.0
    if c > upper_n + 2.0 * atr_i:
        return True
    if c < lower_n - 2.0 * atr_i:
        return True
    return False
```

Если `not resume_ready(...):` — skip entry на этом баре.

### 8.3 Чего ls5 НЕ делает

- Не меняет размер позиции.
- Не инвертирует направление.
- Не смотрит equity drawdown.
- Не фильтрует «слабые» breakout вне паузы (`min_breakout_atr` остаётся 0).

---

## 9. Что сознательно НЕ брать в бот (отвергнуто research)

| Идея | Почему нет |
|------|------------|
| Equity DD circuit breaker | Режет runners, убивает IS return |
| ADX / min ATR% / min channel width | Плохой transfer на holdout |
| Streak-scaled risk (×0.5/×0.25 после 3/5 лоссов) | Съедает return, стрик по числу сделок не чинит |
| Same-side fatigue / flip-flop filters | Аутопсия: нет устойчивого режима; HO ломает IS |
| 15m / 30m core | Fee-bleed |
| Pyramiding / ATR-trail как default | Вне заморозки |
| Always-on signal invert | Отвергнуто |
| Подгонка N/M/ATR на 2024–2026 | Holdout только для валидации |

Опциональный smoother (не default): `min_breakout_atr=1.0` — ниже return, лучше DD. Если когда-нибудь включать — только как отдельный режим, не silent replace.

---

## 10. Ожидаемое поведение (чтобы не «сломать» бота паникой)

Типично на BTC 1h core:

| Метрика | Порядок величины |
|---------|------------------|
| Winrate | ~25–30% |
| Profit factor | ~1.2–1.5 (зависит от окна/fees) |
| Max DD | часто **−25%…−35%** на длинных окнах |
| Avg hold | часы–дни; runners существенно дольше |
| Max loss streak | десятки сделок возможны |

Если бот показывает WR 55% и крошечный DD при том же коде — скорее баг (lookahead / wrong ATR / пропуск шортов).

**Не судите go-live по месячному P&L.** День на EW-дашборде зелёный ~19% времени, неделя ~47% (монетка), месяц ~75%. Красный месяц при живом shadow — норма. Контракт «бот = backtest» и пороги — **§17**.

### 10.1 Проверка «это не подгонка под 2020+»

Pre-sample **2017\*–2019** (до design window 2020–2023), research fees 0.05%, те же параметры — baseline прибылен на BTC и ETH по годам (см. research `presample_2017_2019`). Holdout **2024–2026** тоже в плюсе. Это поддерживает гипотезу рабочего trend-follow edge, но **не гарантия** live (funding, slippage, outages).

---

## 11. Архитектура бота (чеклист с нуля)

1. **Market data**
   - Подписка на 1h kline close (websocket + REST backfill).
   - Persist OHLC; после рестарта догрузить ≥ max(warm, 120d).
2. **Clock**
   - Торговые решения **только на closed candle** (или эквивалент: убедились, что бар финален).
3. **Strategy engine**
   - Чистая функция/класс: bars → signals / desired position (детерминизм).
   - Отдельный модуль sizing.
   - Отдельный модуль ls5 state (если live-профиль B).
4. **Execution**
   - Place/cancel/replace stops.
   - Reconcile exchange position vs local state каждые N секунд.
5. **Risk guards (операционка, не alpha)**
   - Max notional, max leverage, kill-switch, daily loss limit (на ваш вкус).
   - Не путать с research DD-pause.
6. **Accounting / journal**
   - Полная схема полей — §17.5. Минимум: signal_time, entry, stop, exit, gross, fees, funding, consec_losses, pause_until, shadow-counterpart.
7. **Shadow mode (обязателен с дня 1)**
   - Три книги: Live, Shadow-ls5, Shadow-baseline. Theoretical fills на **тех же futures 1h klines**, что ест бот. Даже если на биржу идёт только ls5.
   - Метрики и пороги — §17.
8. **Parity harness**
   - До go-live: прогнать engine на сохранённых 1h JSON / CSV из research и сравнить список сделок (entry_time, direction, exit, outcome) с допуском по цене (≥99%, §15).
   - После go-live: каждую сделку сверять Live ↔ Shadow, не «live месяц vs средний год дашборда».

---

## 12. Минимальный набор unit-тестов для parity

1. **ATR:** фиксированный короткий OHLC → известный ATR[i].
2. **Channel causality:** бар `i` не входит в `max(high[i-N:i])`.
3. **Long entry:** искусственный пробой вверх → entry на следующем open, stop = entry − 1.5×ATR_signal.
4. **Stop priority:** на баре и SL, и channel → outcome `sl`, price=`stop`.
5. **No flat→reentry same bar** после выхода.
6. **ls5:** 5 gross losses подряд → следующие сигналы пропускаются; сильный breakout ≥2ATR во время паузы — вход разрешён.
7. **Sizing:** equity=10_000, risk=1%, cap=1000, distance=50 → risk_usd=100, qty=2.

---

## 13. Конфиг-файл (пример для нового репо)

```yaml
strategy:
  name: donchian_turtle
  tag: 1h_N30_M15_ATR1.5
  timeframe: 1h
  channel_n: 30
  exit_m: 15
  atr_period: 20
  atr_method: sma_tr          # NOT wilder
  atr_stop_mult: 1.5
  max_positions: 1
  min_breakout_atr: 0.0

symbols:
  - BTCUSDT
  - ETHUSDT

sizing:
  risk_pct: 1.0
  max_risk_usd: 1000.0

costs:
  fee_rate_per_side: 0.0005   # research default; подставьте свой VIP
  slippage_rate_per_side: 0.0
  include_funding: false      # включите в live PnL отдельно

live_profile: ls5_cond_brk2.0  # or baseline

# Три книги с дня 1. Знаменатель сверки — shadow на тех же барах, не yearly-дашборд.
telemetry:
  shadow_baseline: true
  shadow_ls5: true
  compare_to: same_window_shadow   # NEVER historical_dashboard_mean
  match_rate_min: 0.99
  slip_median_bps_ok: 2.0
  slip_median_bps_warn: 5.0
  slip_median_bps_kill: 10.0
  pnl_ratio_ok: 0.70          # live_net_xf / shadow_fee5_slip0, same window
  pnl_ratio_base: 0.60        # = fee5_slip2 на BTC holdout; ещё зелёный
  pnl_ratio_watch: 0.40
  pnl_ratio_min_trades: 50    # ниже — ratio не голосует
  funding_line_separate: true

ls5_cond_brk2.0:
  enabled: true
  loss_streak_n: 5
  loss_streak_pause_bars: 24
  cooldown_resume_min_breakout_atr: 2.0

disabled_by_research:
  pause_dd_pct: 0
  adaptive_risk: false
  invert_after_loss: false
  min_adx: 0
  min_atr_pct: 0
  min_channel_atr: 0
  risk_streak_scaling: false
```

---

## 14. Словарь терминов

| Термин | Значение |
|--------|----------|
| Signal bar `i` | Бар, на close которого возник сигнал |
| Entry bar `i+1` | Бар, на open которого вход |
| Runner | Крупный прибыльный тренд-hold (правый хвост R) |
| Gross loss | Убыток по цене без комиссий |
| IS / HO | In-sample 2020–2023 / Holdout 2024–2026 |
| Core | Замороженные N/M/ATR без паузы |
| Alternate B | `ls5_cond_brk2.0` |
| Shadow | Теоретические fills стратегии на тех же live-свечах, без биржевых ордеров |
| Match rate | Доля сделок Live ∩ Shadow по `(symbol, signal_time, direction)` |
| Slip bps | Проскальзывание live-цены vs shadow-цены, в базисных пунктах notional на сторону |
| PnL ratio | `live_net / shadow_net` **на том же окне**; не vs исторический средний месяц |
| Fee-0 dashboard | Yearly dashboard (risk 1.5%, fee 0%) — ожидание формы, **не** знаменатель live |

---

## 15. Definition of done для порта в новый репо

- [ ] ATR SMA-TR совпадает с research на фикстуре
- [ ] Список сделок baseline на BTC 2024 (или другом фиксированном окне) совпадает ≥99% по entry_time+direction (цены — с допуском тика)
- [ ] ls5 уменьшает число сделок vs baseline на том же окне и воспроизводит паузы
- [ ] На бирже: один вход → один stop reduce-only
- [ ] Рестарт бота восстанавливает position + consec_losses + pause_until из journal/exchange
- [ ] Shadow baseline пишет equity без реальных ордеров
- [ ] С дня 1 пишутся три книги (Live / Shadow-ls5 / Shadow-baseline), кривые двух балансов на одном экране (§17.4.1) и суточный отчёт §17.12
- [ ] Match rate, slip bps, funding и PnL ratio считаются по формулам §17.6–17.8
- [ ] Kill-switch не завязан на «месяц хуже +9% дашборда»

---

## 16. Контакты с research-артефактами (если есть доступ к старому репо)

| Что | Путь |
|-----|------|
| Checkpoint | `docs/DONCHIAN_CHECKPOINT_1H_N30.md` |
| Spec v1 | `docs/DONCHIAN_BREAKOUT_SPEC.md` |
| Trade engine | `strategies/donchian/trades.py` |
| ATR | `strategies/mss/atr.py` |
| Sizing/fees | `strategies/mss/accounting.py`, `strategies/mss/fees.py` |
| Loss-streak autopsy | `output/donchian_backtest/loss_streak_autopsy/VERDICT.md` |
| Pre-sample 2017–2019 | `output/donchian_backtest/presample_2017_2019/VERDICT.md` |
| Yearly dashboard (fee 0%) | `output/donchian_backtest/dashboard_yearly/dashboard_data.json` |
| Cost sensitivity (fee+slip) | `output/donchian_backtest/cost_sensitivity/VERDICT.md` |
| Live vs backtest telemetry | этот файл, **§17** |

---

## 17. Live vs backtest: прозрачная статистика с дня 1

**Задача этого раздела:** пока экономический результат ещё шум (месяцы, иногда год), вы каждый день можете ответить на один вопрос: *мы торгуем ту же стратегию, что в research, или уже другую?*

Если да — красный месяц значит «нужно время».  
Если нет — ждать бессмысленно: чинить бот / исполнение, а не «край».

### 17.1 Два вопроса, которые нельзя смешивать

| # | Вопрос | Горизонт ответа | Чем мерить |
|---|--------|-----------------|------------|
| **Q1** | Бот реализует замороженный Donchian 1:1? | **дни–недели** | match rate, ATR parity, ls5-state, slip bps |
| **Q2** | Край ещё жив в live (fees, funding, режим рынка)? | **12–24 месяца** | live и **корректный** shadow на длинном окне |

Смешивать их — главная ошибка. `live_месяц / +9%_дашборда` отвечает **ни на Q1, ни на Q2**.

Yearly-дашборд (fee 0%, fresh $1000, risk 1.5%) — это карта **формы** стратегии (низкий WR, редкие runners, зелёный год при красных неделях). Это не KPI, который live обязан повторить в первый квартал.

### 17.2 Три слоя сверки (считать все три)

Слой выше не готов → слой ниже **не интерпретировать**.

```text
A. Implementation  — те же сделки, тот же state machine
B. Execution       — те же цены ± несколько bps, комиссии/funding разложены
C. Economic        — live_net / shadow_net на том же окне
```

- **A красный** — баг порта. P&L врёт.
- **A зелёный, B красный** — стратегия та, исполнение нет. Край можно убить слипом, даже если сигналы идеальны.
- **A и B зелёные, C шумный/красный первые месяцы** — «всё ок, нужно время».
- **A и B зелёные, C устойчиво мёртвый на 12 месяцах** — уже Q2 (край/режим/издержки), не «бот сломан».

### 17.3 Что с чем сравнивать (и что запрещено)

**Единственный канонический знаменатель:** shadow на **тех же** closed 1h futures-свечах, том же `eval_from…eval_to`, том же профиле (`ls5_cond_brk2.0` если live = ls5).

| Сравнение | Можно? | Зачем |
|-----------|--------|-------|
| Live ls5 ↔ Shadow ls5, то же окно, те же futures klines | **Да, главный** | Q1 + C |
| Shadow ls5 ↔ Shadow baseline, то же окно | Да | ls5 реально пропускает паузы, а не «молчит» |
| Live fills ↔ Shadow theoretical prices по matched сделкам | **Да** | слой B |
| Engine на research JSON ↔ сохранённый список сделок research | Да, **до** go-live | порт 1:1 (§15) |
| Live ↔ yearly dashboard fee 0% / +9%/мес / +80…400%/год | **Нет** | другой risk, нет комиссий, другой путь, другой вопрос |
| Live месяц ↔ средний holdout 2024–2026 | **Нет** | другой рынок; знаменатель не тот |
| Live BTC ↔ historical ETH | Нет | |

Ожидаемый drift, который **не** баг:

- Research-сигналы считались по **spot** 1h Vision; live логично кормить **futures** 1h. Цены близки, не идентичны. Поэтому historical research report за 2024 **не** обязан совпасть тик-в-тик с live-shadow на futures. Совпадать обязаны Live и Shadow, питаемые **одним** рядом.
- Backtest SL = ровно −1R по `risk_usd`. Live SL = реальный stop fill + fee. Расхождение кладётся в слой B, не в «край умер».
- Funding в research = 0. В live — отдельная строка (§17.7.3). Не заворачивать funding внутрь slip.

### 17.4 Три параллельные книги (обязательны с часа 0)

Даже если на биржу уходит только ls5, движок считает все три:

| Книга | Ордера | Свечи | Роль |
|-------|--------|-------|------|
| **Live** | реальные | futures 1h | факт счёта |
| **Shadow-ls5** | нет, theoretical fills §3/§6 | те же | эталон live-профиля |
| **Shadow-baseline** | нет, core без паузы | те же | контроль, что ls5 не «сломан в ноль сделок» |

Правила книг:

1. Один и тот же bar-close clock. Решение только на **closed** candle.
2. Shadow не смотрит на live-позицию (нет «подстройки» под биржу). Если биржа рассинхронилась — это Live-инцидент, shadow идёт своим путём; в журнале флаг `live_desync=true`.
3. После рестарта обе shadow-книги переигрываются с persisted OHLC + journal state (`consec_gross_losses`, `pause_until_bar_index`, `equity` каждой книги). Live equity берётся с биржи, затем сверяется с journal.
4. Стартовый баланс shadow = баланс, с которого вы **решили** торговать live (не $1000 дашборда, если live стартовал с $10k). Risk% и cap — как в live-конфиге.
5. После **каждого** closed 1h писать снимок балансов (§17.4.1). Без этого кривые не построить.

### 17.4.1 Главный экран оператора: Live-баланс vs Shadow-баланс

Это **первый** вид, который вы открываете. Не таблица метрик, не чат с `verdict`, а **две (три) кривые эквити на одних осях с go-live**. Все пороги §17.6–17.13 — подписи к этому графику, не замена ему.

Пока этих кривых нет — live-телеметрия не запущена, даже если сделки пишутся.

#### Что должно быть на экране всегда (без скролла)

**Панель 1 — балансы (герой).**

| Серия | Откуда | Стиль |
|-------|--------|-------|
| **Live** | wallet + uPnL по правилам ниже | основная, толще |
| **Shadow-ls5** | theoretical equity того же профиля | основная |
| Shadow-baseline | core без паузы | тонкая / пунктир, можно тумблером |
| Live ex-funding | Live минус кумулятив funding | опционально, тумблер; для слоя C |

Ось X: время с `go_live_ts` (и пресеты 24h / 7d / 30d / 90d / LTD).  
Ось Y: USDT equity, **одна шкала**, старт обеих книг в одной точке.

Под графиком крупные цифры (не «найти в логе»):

```text
Live          $12,430   (−3.1% DD)
Shadow ls5    $12,610   (−2.8% DD)
Gap           −$180     (−1.4% от shadow)
pnl_ratio_xf  0.86      (или NA)
verdict       WAIT
status A/B/C  green / green / NA
```

`Gap = equity_live - equity_shadow_ls5` (для apples-to-apples лучше `equity_live_ex_funding - equity_shadow_ls5`).  
Растущий по модулю gap при **совпадающей форме** кривых = слой B (fills/funding).  
Разъехавшаяся **форма** (локальные пики в разных местах) = слой A (другие сделки).

**Панель 2 — gap во времени.**  
`gap_usd(t)` и `gap_pct(t) = gap / equity_shadow`. Нулевая линия. Если gap пилит вокруг нуля и обе кривые вниз — это WAIT. Если gap уходит вниз трендом — исполнение.

**Панель 3 — статусы и «всё остальное из §17»** (компактно, не вместо кривых):

- match_rate LTD / 7d, live_only, shadow_only
- side slip median / p90, отдельно SL
- funding LTD vs net
- WR / SL share / streak / pause_until (Live vs Shadow рядом)
- последние mismatch keys

**Панель 4 — сделки на той же оси, что equity.**  
Маркеры entry/exit Live и Shadow. Matched — один маркер (или два почти в одной точке). `live_only` / `shadow_only` — другим цветом, клик → строка журнала.

**Панель 5 — BTC и ETH раздельно**, плюс сумма (если торгуете оба). Суммарный Live vs суммарный Shadow — главный; рукав может врать месяц, как BTC 2021.

#### Как читать кривые за 5 секунд

| Что видно | Значит | Вердикт |
|-----------|--------|---------|
| Live и Shadow-ls5 почти совпадают, обе вниз | тот же бот, плохой отрезок рынка | **WAIT** |
| Live и Shadow почти совпадают, обе вверх | тот же бот, отрезок с runners | **WAIT** (не «я молодец, масштабируй») |
| Форма та же, Live стабильно ниже на N% | те же сделки, хуже fills/fees/funding | слой **B** |
| Форма разная, пики не там | разные сделки / ls5 state / пропуски | слой **A**, не смотреть P&L |
| Shadow-ls5 плоский, baseline растёт | ls5 в паузе (ок, если pause_parity зелёный) | не баг, пока streak сходится |
| Live скачет, Shadow нет | desync, ручной flatten, другой размер | **STOP** разбор |
| Только Live без Shadow на графике | телеметрия сломана | чинить экран, не стратегию |

Не выводить на этот экран линию «ожидаемые +9%/мес дашборда». Она провоцирует ложный STOP.

#### Снимок баланса на каждом баре

Файл `equity_snapshots.csv` (append-only), одна строка на closed 1h **и** на каждый fill (чтобы внутри часа было видно SL):

```text
ts_utc
equity_live                 # wallet equity; см. определение ниже
equity_live_ex_funding      # live минус кумулятив funding с go-live
upnl_live                   # нереализованный, если позиция открыта
wallet_cash                 # без uPnL, для сверки с биржей
equity_shadow_ls5
equity_shadow_baseline
gap_vs_ls5                  # live_ex_funding - shadow_ls5
cum_funding
cum_fees_live
cum_fees_shadow_ls5
match_rate_ltd
side_slip_median_ltd_bps
verdict
```

Без этого ряда график «с нуля после рестарта» не восстановить. Рестарт **дочитывает** файл, не начинает кривую заново.

#### Определение Live-баланса (чтобы не сравнивать тёплое с холодным)

Shadow считает paper-equity как в §5 (fees модели, funding = 0, SL в shadow = −1R).  
Live обязан показывать **сопоставимую** величину:

```text
equity_live = wallet_balance
            + unrealized_pnl   # mark цены биржи
            − (опционально) isolated margin extras, если не в wallet)

equity_live_ex_funding = equity_live - cum_funding_since_go_live
```

Стартовая точка: в момент go-live `equity_live == equity_shadow_ls5 == equity_shadow_baseline == start_equity`. Зафиксировать `start_equity` в конфиге и в первой строке snapshots. Если на бирже уже был мусор (пыль, чужая позиция) — сначала flatten до нуля, потом старт. Иначе gap с бара 0 бессмыслен.

Открытая позиция: на графике Live включает uPnL, Shadow — mark по `close` текущего **закрытого** бара (не тик за тиком, иначе shadow будет глаже). Внутри часа Live может дышать от mark-price; это не расхождение слоя A. Сравнивать книги **на closed candle**, как research.

Комиссии Live — фактические с биржи. Shadow — модель 0.05%/сторону. Разница VIP/BNB уйдёт в gap; это слой B, не A.

#### Обновление экрана

- Перерисовать на каждом closed 1h (обязательно).
- Дополнительно на каждом live fill (вход, стоп, channel-exit).
- Цифры Gap / ratio / verdict — те же, что в суточном отчёте §17.12; экран и файл не имеют права расходиться.

### 17.5 Журнал: обязательные поля

Писать **каждую** попытку входа/выхода, не только fills. Одна строка = одно событие. Сделка склеивается по `trade_id`.

#### 17.5.1 Сделка (закрытая)

| Поле | Кто заполняет | Зачем |
|------|----------------|-------|
| `trade_id` | engine | join Live↔Shadow |
| `book` | `live` / `shadow_ls5` / `shadow_baseline` | |
| `symbol` | `BTCUSDT` / `ETHUSDT` | |
| `direction` | `BUY` / `SELL` | match key |
| `signal_time` | open-time signal-бара `i` (unix sec) | **ключ сверки** |
| `entry_time` | open-time entry-бара `i+1` | |
| `exit_time` | unix sec фактического/теор. выхода | |
| `signal_bar_index`, `entry_bar_index`, `exit_bar_index` | | ls5 pause считается от exit bar |
| `entry_px_shadow` | `open[i+1]` | эталон |
| `entry_px_live` | avg fill (Live only) | slip |
| `stop_px` | `entry ± 1.5 * ATR[signal]` | |
| `exit_px_shadow` | stop или `open[i+1]` channel | |
| `exit_px_live` | avg fill / stop fill | |
| `outcome` | `sl` / `time` (channel) / `manual` / `desync_flatten` | |
| `atr_signal` | ATR[i] | parity индикатора |
| `upper_n`, `lower_n`, `close_signal` | | доказать, что пробой тот |
| `risk_usd`, `qty`, `notional` | §5 | |
| `gross_pnl` | price PnL без комиссий | ls5 streak = по gross |
| `fee_entry`, `fee_exit`, `fee_total` | биржа / модель | |
| `funding_pnl` | Live only; shadow = 0 | отдельная строка |
| `net_pnl` | gross − fees (+ funding в live, **отдельный столбец тоже**) | |
| `realized_r_gross` | `gross / risk_usd` (SL в live — фактический, не принудительно −1) | |
| `consec_gross_losses_after` | | ls5 parity |
| `pause_until_bar_after` | | ls5 parity |
| `skipped_reason` | `null` / `ls5_pause` / `no_bar` / `desync` / `kill_switch` | shadow-only и live-skip |
| `live_desync` | bool | позиция биржи ≠ desired |

#### 17.5.2 Бар (каждый closed 1h, оба символа)

Нужен, чтобы ловить «сигнал был, вход не взяли» без сделки:

| Поле | Зачем |
|------|-------|
| `bar_time`, `symbol`, `atr`, `upper_n`, `lower_n`, `close` | |
| `want_baseline` | `BUY`/`SELL`/`None` без ls5 |
| `want_ls5` | то же с паузой/resume |
| `ls5_pause_active`, `ls5_resume_ready` | |
| `live_desired`, `live_actual_position` | desync |
| `shadow_ls5_position`, `shadow_baseline_position` | |

Хранить бар-лог минимум **120 дней** (warmup + разбор). Сделки — бессрочно.

### 17.6 Слой A — implementation (те ли это сделки)

#### 17.6.1 Ключ матчинга

```text
key = (symbol, signal_time, direction)
```

`signal_time` — open time signal-бара, **не** время fill. Fill может сдвинуться на секунды; бар — нет.

Два множества закрытых+открытых попыток за окно:

```text
S = keys Shadow-ls5, у которых был вход (не skipped)
L = keys Live, у которых был вход
matched     = S ∩ L
live_only   = L \ S     # лишний вход
shadow_only = S \ L     # пропущенный вход
```

```text
match_rate = |matched| / |S ∪ L|
```

(Dice `2|matched|/(|S|+|L|)` почти совпадает при малом числе расхождений; в отчёте печатать **оба**, порог держать по `match_rate`.)

Отдельно:

```text
skip_parity = доля баров, где want_ls5 совпал у Live-движка и Shadow-ls5
pause_parity = доля баров, где (pause_active, resume_ready) совпали
atr_max_abs_rel_err = max |ATR_live[i] - ATR_shadow[i]| / ATR_shadow[i]
```

ATR на одном и том же ряде обязан совпасть почти в float. Если `atr_max_abs_rel_err > 1e-9` на overlapping bars — у вас другой ATR (часто Wilder). Это слой A, стоп-кран.

#### 17.6.2 Пороги слоя A

Считать **ежедневно**; порог применяется с **N ≥ 10** ключей в `|S ∪ L|` (до этого — разбор каждого расхождения вручную, без «зелёного света»).

| Метрика | Зелёный | Жёлтый | Красный |
|---------|---------|--------|---------|
| `match_rate` | **≥ 0.99** | 0.95–0.99 | **< 0.95** |
| `live_only` за календарную неделю | 0 | 1 (есть explain) | ≥ 2 без explain |
| `shadow_only` за неделю | 0 | 1 (рестарт/kill, задокументирован) | ≥ 2 или молчаливый пропуск runner |
| `skip_parity` | ≥ 0.995 | 0.98–0.995 | < 0.98 |
| `pause_parity` | ≥ 0.995 | иначе | любой устойчивый drift ls5 state |
| `atr_max_abs_rel_err` | ~0 (float) | — | любое систематическое расхождение |
| WR Live vs Shadow на **matched** | Δ ≤ 3 п.п. | 3–8 п.п. | > 8 п.п. (при n≥30) |
| Доля `outcome` sl / channel на matched | Δ ≤ 5 п.п. | 5–10 | > 10 |

Жёлтый: чинить в тот же день, торговлю можно не снимать, если explain есть (один reconnect).  
Красный: **не ждать край**. Снять live или оставить только shadow, пока match не вернётся в зелёный.

#### 17.6.3 Как разбирать расхождения

Каждый `live_only` / `shadow_only` — строка в `mismatches.csv`:

1. Бар-лог на `signal_time`: `want_ls5`, pause, ATR, канал, close.
2. Была ли позиция уже открыта (research не открывает вторую).
3. Рестарт / kill-switch / ручной flatten.
4. Опоздали к open следующего часа (вход «в теле» бара = другая цена и, часто, другой trade id — это **и** A, **и** B).
5. ls5: shadow в паузе, live вошёл (или наоборот) → сломан streak-счётчик (gross vs net!).

Типичные баги порта, которые слой A ловит за неделю, а P&L — за квартал:

- Wilder ATR вместо SMA-TR §4
- канал включает бар `i` (lookahead)
- вход по close сигнала, а не по open `i+1`
- шорты выключены / hedge mode
- ls5 считает net после комиссий
- re-entry на том же баре после SL
- другой `N/M` или `min_breakout_atr` «тихо»

### 17.7 Слой B — execution (те ли это цены и издержки)

Только по **matched** сделкам. Unmatched не усреднять в slip — они принадлежат слою A.

#### 17.7.1 Формулы проскальзывания

Знак: adverse > 0 (live хуже shadow).

```text
sign = +1 если BUY, −1 если SELL

entry_slip_bps = 10000 * sign * (entry_px_live - entry_px_shadow) / entry_px_shadow
exit_slip_bps  = 10000 * sign * (exit_px_shadow - exit_px_live)  / exit_px_shadow
rt_slip_bps    = entry_slip_bps + exit_slip_bps
side_slip_bps  = rt_slip_bps / 2
```

Для `outcome=sl`: `exit_px_shadow = stop_px`. Live stop-market часто хуже — это ожидаемо, но **медиана** всё равно лимитируется таблицей ниже.

Печатать: median / p90 / max по `entry_slip_bps`, `exit_slip_bps` (раздельно sl vs channel), `side_slip_bps`. Окно: rolling 20 сделок **и** с начала live.

Implied extra cost vs research-модели `fee=0.05%, slip=0`:

```text
# положительное = live дороже модели на сторону
extra_side_bps = side_slip_bps
  + 10000 * ((fee_live_paid / notional_round_trip) - 0.0005 * 2) / 2
```

Комиссии биржи почти всегда ~0.05%/сторону VIP0 taker; если меньше (maker/VIP) — extra отрицательный, это хорошо. Не прятать это в PnL ratio без расшифровки.

#### 17.7.2 Пороги слоя B (из `cost_sensitivity`)

Research stress, holdout 2024–2026, BTC+ETH, fee 0.05%/side, `$10k`, risk 1%:

| Сценарий | slip/side | BTC return | ETH return | Смысл для live |
|----------|-----------|------------|------------|----------------|
| `fee5_slip0` | 0 bps | +210.7% | +235.8% | эталон shadow «идеальные fills + комиссия» |
| `fee5_slip2` | 2 bps | +132.1% | +179.0% | **спокойная база** (RT ≈ 14 bps) |
| `fee5_slip5` | 5 bps | +49.8% | +111.3% | край тонкий, не масштабировать |
| `fee5_slip10` | 10 bps | **−27.9%** | +32.9% | на BTC holdout край **убит** |

Пороги по **медиане** `side_slip_bps` (matched, n≥20; до 20 сделок — смотреть каждую):

| Медиана side slip | p90 | Статус | Действие |
|-------------------|-----|--------|----------|
| **≤ 2 bps** | ≤ 8 | Зелёный | база research |
| 2–5 bps | ≤ 15 | Жёлтый | можно торговать, не поднимать риск, искать причину (особенно SL) |
| 5–10 bps | любое | Оранжевый | край уже как `fee5_slip5`; бумажный плюс может стать live-нулем |
| **≥ 10 bps** устойчиво | любое | Красный | на BTC-пути holdout это смерть края; чинить исполнение **или** остановить |

Раздельно контролировать **SL-ноги**. Channel-exit по next open должен быть близок к entry-slip. Если entry ~1 bp, а SL median 20 bps — стоп-маркет ест runners' противоположность (стопы). Имеет смысл stop-limit с аварийным watchdog, но **не** ценой пропущенного стопа.

#### 17.7.3 Funding — отдельная книга, не slip

Research funding **не моделировал**. Hold ~25h средний, runners — дни: funding не нуль.

```text
live_net_ex_funding = live_net - funding_pnl
funding_bps_per_day = 10000 * funding_pnl / notional / hold_days
```

В отчёте всегда три числа: `live_net`, `live_net_ex_funding`, `funding_pnl`.  
Слой C (PnL ratio) считать **дважды**: с funding и без. Решение «исполнение ок?» — по **ex-funding** vs shadow. Решение «край жив на перпетах?» — по live **с** funding на длинном окне.

Если funding стабильно съедает > половины shadow-expectancy на 6–12 месяцах при зелёных A/B — это Q2, не баг порта.

#### 17.7.4 Прочие издержки, которые shadow не видит

Логировать отдельно, не смешивать со slip:

- round-trip из-за **ошибочного** flatten + re-entry
- частичные fills / reduce-only reject
- пропущенный час (дыра в kline) → пропущенный channel-exit
- задержка: решение не на open `i+1`, а +N минут внутри бара

Каждый такой инцидент: `incident_id`, минуты задержки, extra bps, extra fee. Недельный `incident_bps_sum` — если он сам по себе > 5 bps/сторону equivalent, слой B жёлтый даже при «чистой» медиане matched.

### 17.8 Слой C — денежный коэффициент live / shadow

#### 17.8.1 Формула

На окне `[t0, t1]` (с момента go-live или rolling 90d — печатать **оба**):

```text
shadow_net = сумма net Shadow-ls5          # модель fee 0.05%, slip 0
live_net_xf = сумма (live net − funding)   # apples-to-apples с shadow
live_net_all = сумма live net включая funding

# защита от деления на шум:
eps = max(0.005 * start_equity, 1.0 * typical_risk_usd)

если |shadow_net| < eps:  ratio = NA (не голосует)
иначе:
  pnl_ratio_xf  = live_net_xf  / shadow_net
  pnl_ratio_all = live_net_all / shadow_net
```

Дополнительно на **matched** сделках (честнее на коротком окне):

```text
sum_R_live    = сумма realized_r_gross Live
sum_R_shadow  = сумма realized_r_gross Shadow по тем же keys
r_ratio       = sum_R_live / sum_R_shadow     # если |sum_R_shadow| ≥ 5
corr_R        = corr(R_live, R_shadow)        # n_matched ≥ 30
```

`corr_R` на matched при нормальном исполнении должен быть **≥ 0.95**. Если match_rate высокий, а `corr_R < 0.85` — цены/qty разъехались (слой B), не «рынок».

#### 17.8.2 Почему нельзя делить на fee-0 дашборд

Holdout 2024–2026, BTC, `$10k` (cost sensitivity):

```text
fee0_slip0   +543.6%
fee5_slip0   +210.7%     → 210.7/543.6 ≈ 0.39
fee5_slip2   +132.1%     → 132.1/543.6 ≈ 0.24
```

Идеальный live с VIP0 taker и **нулевым** слипом даст ~**40%** от fee-0 дашборда / fee-0 holdout. Это не провал. Знаменатель слоя C — **`fee5_slip0` shadow**, то есть ~1.0 при идеальных fills.

Отношение live / `fee5_slip0` shadow vs ladder. Пороги **не пересекаются**:

| `pnl_ratio_xf` | Соответствует ladder | Статус |
|----------------|----------------------|--------|
| **≥ 0.70** | лучше `slip2` (ETH slip2/slip0 ≈ 0.76) | Зелёный «хорошо» |
| **0.60 – < 0.70** | `fee5_slip2` (BTC 132/211 ≈ 0.63) | Зелёный «база research» |
| **0.40 – < 0.60** | между slip2 и slip5 | Жёлтый, не масштабировать |
| **0.20 – < 0.40** | ~`fee5_slip5` | Оранжевый: бумажный край почти съеден |
| **< 0.20 или минус при shadow_net > eps** | slip10 / баг / funding | Красный, **если A/B зелёные достаточно долго** |

Смешанный BTC+ETH: конфиг `pnl_ratio_ok: 0.70` (цель), база live = **≥ 0.60**. Не требовать 1.00: нулевой слип на стопах нереалистичен.

#### 17.8.3 Когда слой C имеет право голоса

| Выборка | C голосует? | Почему |
|---------|-------------|--------|
| < 20 matched | Нет | один стоп = весь ratio |
| 20–49 | Только жёлтый флаг, не kill | |
| **≥ 50 matched** (~2 мес BTC+ETH, ~25/символ/мес) | Мягкий фильтр | пороги §17.8.2 |
| ≥ 150 matched (~6 мес) | Рабочий фильтр | |
| 12 календарных месяцев | Первое сравнение с «годом дашборда» | всё ещё один режим рынка |
| 24 месяца | Q2 всерьёз | |

Пока C = `NA` или n < 50: решение только по A и B.

Никогда не включать C в kill-switch вида «месяц < 0 → стоп». Это как раз то, от чего §10 предостерегает.

### 17.9 Профиль, который обязан совпасть даже в минусе

Эти величины на Shadow-ls5 **и** на Live (matched / те же окна) должны быть «как research», иначе это не Donchian, а другой процесс. Сверять еженедельно с n≥20.

| Метрика | Ожидание research | Live/Shadow Δ, зелёный | Если сильно иначе |
|---------|-------------------|------------------------|-------------------|
| Winrate | **25–30%** (pooled ~28%) | ±5 п.п. | WR 45%+ → lookahead / нет шортов / не тот выход |
| SL share | ~60% выходов — ATR stop | ±10 п.п. | |
| Long share | ~50% (не вечный лонг) | 40–60% | |
| Avg hold | ~25h; медиана ~15–20h; p90 много выше (runners) | порядок тот | все холды 2h → другой выход |
| Max loss streak | 8–25 сделок **внутри прибыльного года** бывало | не kill | «чинить» стрик фильтрами запрещено §9 |
| Time in market | порядка 40–55% | | |
| Avg R (gross) | типично +0.2…+0.8 на годовом окне, не +0.05 при n=200 | смотреть vs shadow того же окна | |

Если Shadow сам показывает WR 28% и минус за месяц, а Live WR 28% и минус за месяц — **это соответствие**, не поломка.

Если Shadow +8% за месяц, Live −12% при match 99% — смотреть слой B (fills/funding), не «стратегия».

Если Shadow −5%, Live −5%, match 99%, slip 2 bps — **ждать**. Это Q2-шум.

### 17.10 Горизонты: что уже можно знать, а что ещё нельзя

Цифры ниже — **карта ожиданий**, не KPI. Источники: yearly dashboard 2018–2026 YTD (fee 0%, risk 1.5%, fresh $1000/год, EW BTC+ETH) и rolling EW equity 2020-01-01→2026-09-09 (fee 0%, risk 1%, непрерывное компаундинг). Live с 0.05% fee будет бледнее по деньгам; **форма** (неделя≈монетка, месяц чаще зелёный, год — единица дашборда) должна сохраниться.

#### 17.10.1 Yearly dashboard (форма)

| Горизонт | P(EW equity up) | Как жить с этим |
|----------|-----------------|-----------------|
| День | 19% up, 41% down, 40% flat | не смотреть |
| Неделя | **47%** | монетка; неделя в минусе ничего не значит |
| Месяц | **75%** up, 21% down | каждый 4-й месяц красный — in-sample норма |
| Год | **9/9** EW зелёные (2018–2026 YTD) | первая сопоставимая с дашбордом единица |

Средний месяц на том дашборде ~**+9%** арифметика, типичный (медиана внутри года) ближе к **+5…8%**. Это **fee 0%**. Live так не планировать.

Слабые, но всё ещё «рабочие» годы дашборда: BTC **2021 +6% при DD −33%**; EW **2023 DD −29%** при финише +142%; EW **2025 +80%** (худший полный год). Max loss streak 2023 BTC = **25**.

#### 17.10.2 Rolling 2020–2026 EW (fee 0%, непрерывный путь)

Overlapping окна — не независимые тесты; это форма пути.

| Окно | P(EW > 0) | Худшее окно | Комментарий |
|------|-----------|-------------|-------------|
| 7d | 49% | −6% | шум |
| 30d | 72% | −13% | |
| 91d (~3м) | 91% | −14% | красный квартал редок, но был |
| 182d (~6м) | 99.6% | −6% | почти всегда зелёный **на этом fee-0 пути** |
| 365d | 100% | **+31%** худший | ни одного красного года на этом отрезке |
| Max underwater | 140 дней (2025-05-14→09-30), DD −12% | красный квартал in-distribution |
| Подряд красных календарных месяцев | максимум **2** (n=80) | 3 красных подряд — уже необычно для EW fee-0 |

Live с комиссиями: 6–12 месяцев **не** обязаны быть зелёными как эта таблица. Таблица отвечает на «можно ли паниковать от одного красного месяца при зелёных A/B»: **нет**.

#### 17.10.3 Практический календарь после публикации

| Срок | Имеет смысл смотреть | Ещё нельзя заключать |
|------|----------------------|----------------------|
| День 1–7 | ATR parity, первые ключи match, опоздание к open, stop висит на бирже | P&L, WR |
| 2–4 недели | слой A (если ≥10 сделок), первые slip | «стратегия не работает» |
| 1–3 месяца | A обязан быть зелёный; B — медиана slip; 1 красный месяц ок | C почти всегда шум |
| 6–9 месяцев | C мягко (≥50–150 сделок): ratio vs shadow | «край мёртв навсегда» |
| **12 месяцев** | первый счёт, сопоставимый с годом дашборда; сравнивать Live **и** Shadow с fee-path, не с +80…400% | один режим рынка |
| **24 месяца** | Q2: жив ли край live | «гарантия следующего десятилетия» — никогда |

### 17.11 Исторические якоря (для глаз, не для деления)

Держать в шапке live-дашборда бота как reference, с подписью **«не KPI»**:

**Holdout 2024–2026, BTC, `$10k`, risk 1%, fee 0.05% (checkpoint):** +142% / DD −34% / n=516 / PF 1.22.  
**ls5_cond_brk2.0 тот же holdout:** +164% / DD −30% / n=456 / PF 1.25.

**Yearly dashboard EW fee 0% risk 1.5%:** 2018 +304%, 2019 +169%, 2020 +397%, 2021 +94%, 2022 +194%, 2023 +142%, 2024 +158%, 2025 +80%, 2026 YTD +166%. Все годы «+»; это не прогноз live.

**Pre-sample 2017\*–2019** с fee 0.05% тоже плюс (ETH 2019 всего +15% / DD −21%) — край не только «бычий 2020+», но и не «каждый год x2».

### 17.12 Что бот обязан показывать и печатать

Без **экрана §17.4.1** и без суточного файла live считать неготовым. Файл — архив; экран — то, на что вы смотрите каждый день.

#### Экран (обязательная вёрстка)

1. Кривые **Live** и **Shadow-ls5** (герой) + gap-панель под ними.
2. Крупно: два баланса, gap, `pnl_ratio_xf`, `verdict`, A/B/C.
3. Полоса метрик: match, slip, funding, WR, streak.
4. Маркеры сделок; mismatch — отдельно и кликабельно.
5. Переключатель BTC / ETH / combined.

Если оператор не видит оба баланса на одном графике — спецификация не выполнена. Таблица цифр без кривых **недостаточна**: форму расхождения (B vs A) глазами на числах не отличить.

#### Ежедневно (после последних 00:00–23:00 UTC closed hours)

#### Ежедневно (после последних 00:00–23:00 UTC closed hours)

```text
date_utc
equity_live, equity_shadow_ls5, equity_shadow_baseline
dd_live, dd_shadow_ls5
n_open_live, n_open_shadow
n_closed_today_live, n_closed_today_shadow
match_rate_ltd, match_rate_7d
live_only_7d, shadow_only_7d
side_slip_median_ltd_bps, side_slip_p90_ltd_bps, sl_exit_slip_median_bps
fee_paid_today, funding_today, live_net_today, live_net_xf_today, shadow_net_today
pnl_ratio_xf_ltd          # NA если |shadow| < eps
consec_gross_losses_live, pause_until_live, same for shadow_ls5
desync_seconds_max_24h
incidents_24h
status_A, status_B, status_C   # green/yellow/red/na
verdict                       # WAIT / INVESTIGATE / STOP
```

`verdict` считается **только** из матрицы §17.13, не из знака дневного PnL.

#### Еженедельно

Тот же набор + WR, SL share, long share, avg hold, max loss streak LTD, список всех mismatch keys за неделю (полный dump), топ-5 сделок по `|R|` (проверить, что runners совпали по ключу).

Раз в неделю: переиграть Shadow с нуля по сохранённому OHLC и сверить equity с инкрементальным shadow (регрессия state). Расхождение > $1 или > 0.1% — баг журнала.

### 17.13 Матрица решений: WAIT / INVESTIGATE / STOP

Считать каждый день. **STOP** = снять ордера, shadow оставить. **INVESTIGATE** = можно не торговать новые входы, открытые вести по стопу. **WAIT** = руки прочь от alpha.

| A | B | C | P&L live | Вердикт | Смысл |
|---|---|---|----------|---------|-------|
| зел. | зел. | NA / шум / красн. при n<50 | любой, включая −15% DD | **WAIT** | бот = backtest; нужно время |
| зел. | зел. | ≥0.60 при n≥50 | минус за месяц | **WAIT** | рынок, не порт |
| зел. | жёлт. | 0.40–0.70 | любой | **INVESTIGATE** | слип ест край; не масштабировать |
| зел. | красн. (≥10 bps med) | любой | любой | **STOP** исполнение | holdout BTC slip10 = смерть края |
| жёлт. | любой | любой | любой | **INVESTIGATE** | 1 mismatch/неделя ещё ок, если explain |
| красн. | любой | любой | даже плюс | **STOP** порт | зелёный P&L при чужих сделках — не наша стратегия |
| зел. | зел. | <0.20 или минус при shadow≫eps, n≥150, 6–9 мес | устойчивый минус | **INVESTIGATE** Q2 | сначала ещё раз проверить A/B; потом издержки/funding/режим |
| зел. | зел. | Live **и** Shadow красные 12 календарных месяцев | год в минусе | Q2: вне EW-истории 2018–2026 | не «докрутить ADX»; решать, жить ли с отсутствием края |
| любой | любой | — | DD хуже −35% **и** shadow DD при этом < −15% (расхождение) | **STOP** | Live рвётся, shadow нет → исполнение/сайзинг/плечо |
| любой | любой | — | DD −25…−35% **и** shadow рядом | **WAIT** | in-distribution (2021/2023/2025) |

Запрещённые триггеры kill-switch (если поставите — сломаете стратегию, §9):

- equity DD-pause как в research `--pause-dd-pct`
- «3 лосса подряд → стоп бота»
- «неделя < 0»
- «месяц < +9% дашборда»
- «WR < 40%»

Операционные kill-switch (можно): потеря связи с биржей, нет стопа на открытой позиции, desync > N секунд, notional/leverage выше лимита, ручной panic button. Это не alpha.

### 17.14 Примеры, чтобы не сойти с ума

**Пример 1 — «всё ок, нужно время».**  
30 дней, 28 matched, match_rate 1.00, side slip median 1.8 bps, WR 25%, 7 лоссов подряд, live −6%, shadow_ls5 −5%, funding −0.3%.  
`verdict = WAIT`. Это учебник turtle, не баг.

**Пример 2 — баг при зелёном счёте.**  
Live +12% за 3 недели, WR 52%, match_rate 0.71, live_only куча лонгов, shadow в плюсе слабее.  
`verdict = STOP` слой A. Вы торгуете не Donchian (часто: пропуск шортов или вход по close).

**Пример 3 — край съеден стопами.**  
match 0.99, corr_R 0.97, entry slip 1 bp, SL exit slip median 18 bps, `pnl_ratio_xf=0.25` при n=80, shadow +9%, live +2%.  
`verdict = STOP/INVESTIGATE` слой B. Сигналы наши, fills нет. Не добавлять ADX.

**Пример 4 — сравнивать с дашбордом нельзя.**  
Месяц live +3% после комиссий. Дашборд «средний месяц +9%». Оператор думает «коэффициент 0.33, стратегия сдохла». Shadow того же месяца +2.5%, match 1.00, slip 2 bps.  
Реальный `pnl_ratio_xf ≈ 1.2`. Вердикт WAIT. Дашборд fee-0 здесь ни при чём.

**Пример 5 — настоящий Q2.**  
12 месяцев, match 0.995, slip median 2.1 bps, funding отдельно −4% за год, shadow_ls5 −8%, live_xf −9%, live_all −13%. Оба пути красные полный год.  
Это уже не «подожди месяц». Это вне 9/9 fee-0 лет EW (на fee-path год может быть слабым, как ETH 2019 +15%). Решение: не чинить фильтрами из §9; либо терпеть второй год, либо снижать риск / останавливать. Слой A/B при этом так и остаются зелёными — бот честный.

### 17.15 Definition of done — live telemetry

Пока пункты не выполнены, автоторговля не считается запущенной «по research».

- [ ] Три книги пишут equity и сделки с часа 0
- [ ] Главный экран: Live-баланс и Shadow-ls5 на **одной** оси с go-live + gap; цифры двух балансов всегда видны (§17.4.1)
- [ ] `equity_snapshots.csv` пишется каждый closed бар и каждый fill; переживает рестарт
- [ ] Бар-лог want/pause на каждом closed 1h
- [ ] Суточный отчёт §17.12 уходит в файл/чат без ручной сборки; цифры = экран
- [ ] `verdict` считается матрицей §17.13, не по знаку PnL
- [ ] Mismatch dump за неделю пустой или с explain на каждую строку
- [ ] Shadow replay с нуля раз в неделю сходится с инкрементальным
- [ ] Funding не смешан со slip
- [ ] Нет алерта «ниже среднего месяца дашборда»

### 17.16 Чего сознательно не делать в статистике

- Один «коэффициент соответствия» = `live_pnl / backtest_2018_2026`.
- Оптимизировать бот под рост match за счёт подгонки fills к shadow (shadow должен быть слеп к бирже).
- Подкручивать N/M/ATR/ls5 по первым live-месяцам (§9: holdout не для подгонки).
- Выключать шорты, потому что «сейчас бычий рынок» — сломаете long share и слой A.
- Игнорировать shadow_baseline: если ls5 пропустил runner, который baseline взял, это должно быть видно (пауза сработала — ок; пауза зависла — баг).

---

*Этот файл — контракт реализации. Если live-поведение расходится с ним, это баг порта, пока сознательно не зафиксирован новый research-checkpoint.*
