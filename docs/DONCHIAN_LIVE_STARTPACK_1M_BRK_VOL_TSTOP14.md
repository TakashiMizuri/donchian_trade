# Donchian live: 1m brk+vol + tstop 14h (satellite)

**Это параллельный experimental / satellite-бот**, **не замена** primary live.

| Роль | Профиль | Документ |
|------|---------|----------|
| Primary default | 15m brk+vol + tstop 48h | [`DONCHIAN_LIVE_STARTPACK_15M_BRK_VOL_TSTOP48.md`](DONCHIAN_LIVE_STARTPACK_15M_BRK_VOL_TSTOP48.md) |
| Candidate live | 5m brk+vol + tstop 20h | [`DONCHIAN_LIVE_STARTPACK_5M_BRK_VOL_TSTOP20.md`](DONCHIAN_LIVE_STARTPACK_5M_BRK_VOL_TSTOP20.md) |
| **Satellite (этот файл)** | **1m brk+vol + tstop 14h @ risk 1%** | здесь |

Алгоритм бара, ATR SMA-TR, причинность канала, фильтры `brk0.5+vol_rank`, сайзинг, телеметрия — **те же правила**, что на 15m/5m.  
Меняются TF / календарный N/M/ATR / окна vol-rank / **time-stop 14h** / **операционная рамка** (отдельный счёт, write-off budget).

**Satellite-профиль (2026-10, R-018 + fair@1% + user ops-pick):**

| | |
|--|--|
| TF | **1m** (`bar_seconds = 60`) |
| Канал | **N=1800 · M=900 · ATR(1200)×1.5** (календарный эквивалент 1h N30 × 60) |
| Risk | **1.0%** · cap **1000 USDT** (research) |
| Live entry | **`brk0.5+vol_rank`** (`min_breakout_atr=0.5`, `max_vol_rank=0.64`) |
| Vol windows | **`vol_rank_bars=1200`**, **`vol_rank_lookback=60000`** (~20h / ~42d) |
| Live exit add-on | **`tstop 14h`** → `max_bars_in_trade=840` |
| Роль | **отдельный бот + отдельный баланс**, который **не жалко потерять** |
| Shadow | обязательный paper на том же 1m + slip telemetry |
| ls5 | **OFF** |
| Не фризить | tstop 1h/4h (sniff peaks, multi-year fail); risk 0.5% как softer satellite-alt |

Research-якоря: **R-018** (1m sniff → multi-year PASS tstop14h), slip `dashboard_1m_tstop14h_slip_ladder` (@0.5%) и `dashboard_1m_tstop14h_slip_ladder_risk1` (@1%), TF compare `dashboard_tf_slip_compare_1m_3m_5m`, Calmar-grid `dashboard_tf_risk_calmar_slip2`.

---

## 0. Зачем satellite 1m @1% (и зачем отдельно)

### 0.1 Что показал research

На честном сравнении @ risk 1% (fee 0, EW BTC+ETH, fresh $1000/year):

| Профиль | Avg EW @0 bps | HO @0 | half-EW | HO kill | Worst DD @2 bps |
|---------|--------------:|------:|--------:|--------:|----------------:|
| **1m tstop14h @1%** | **+17824%** | **+23126%** | ~2 bps | ~5 bps | **−83%** |
| 3m tstop20h @1% | +2968% | +3768% | ~2 | ~9 | −72% |
| 5m tstop20h @1% | +1451% | +974% | **~4** | ~10 | **−39%** |

Multi-year PASS на 1m — только **tstop 14h** (840 bars).  
По метрике **avg EW / |worst DD| @ slip 2 bps** лидирует **1m @1%** (score ≈57 vs ~21 у 5m@1%) — но DD под издержками уходит в зону почти wipeout в плохие годы.

### 0.2 Почему не менять primary на 1m

- Cost buffer **тоньше**, чем у 5m: half ≈ 2 bps @1% (у 5m ≈ 4).
- При slip 2–5 bps **worst year DD** −83%…−98%; HO @5 bps уже отрицательный.
- Avg EW до ~7 bps ещё «ебёт» остальные TF — это **маскирует** path-dependent ruin в отдельных годах.
- 1m = больше сделок / больше чувствительность к microstructure, latency, partial fills.

**Вывод research:** 1m @1% — **кандидат на edge**, не на единственный live-якорь.

### 0.3 Зачем тогда запускать

Имеет смысл как **платный live-эксперимент**:

1. Primary (15m и/или 5m) остаётся якорем капитала.
2. Satellite 1m отвечает на вопрос, который бэктест не закрыл: **какой realized slip / capacity на минутках**.
3. Баланс satellite = **write-off budget** (отдельный кошелёк/субаккаунт). Не «потом долью с основной».
4. Масштабировать 1m **запрещено**, пока нет своей статистики fills (bps mark→fill) за достаточный sample.

Это **не** «тихий перевод всего размера на 1m».

---

## 1. Операционная рамка (обязательна)

Перед любым live на 1m:

| Правило | Деталь |
|---------|--------|
| Отдельный бот | Свой процесс / свой `run_id` / свой journal. Не смешивать с 5m/15m pause state |
| Отдельный баланс | Отдельный futures-счёт или жёсткий изолированный budget |
| Write-off | Сумма, потеря которой **не** ломает ops и психологию primary |
| Size vs primary | Satellite **≪** primary (ориентир: единицы–десятки % от основного risk budget, не 50/50) |
| Один символ сначала | **BTCUSDT** only; ETH — отдельным решением после стабилизации |
| Cap on | `MAX_RISK_USD=1000` (research). На tiny-счёте 1% equity обычно < cap |
| Не докладывать | Пополнение satellite после просадки = смена эксперимента; новый `run_id` + явное решение |
| Не усреднять PnL | Primary и satellite в отчётах **раздельно** |

Рекомендуемый стартовый баланс satellite (пример, не догма): сумма, на которой комфортен полный −80…−100% как research-сценарий @2–5 bps в плохой год.

---

## 2. Что должно быть уже сделано

В live-репо:

- [x] Env-порт из [`DONCHIAN_LIVE_STARTPACK_30M_ENV.md`](DONCHIAN_LIVE_STARTPACK_30M_ENV.md)
- [x] Entry-фильтры из [`DONCHIAN_LIVE_STARTPACK_15M_BRK_VOL.md`](DONCHIAN_LIVE_STARTPACK_15M_BRK_VOL.md) (логика brk/vol та же; на 1m другие окна)
- [x] `max_bars_in_trade` в движке (как в 15m/5m tstop packs)
- [x] Поддержка `timeframe=1m` / `bar_seconds=60` (стрим klines, warmup, journal)
- [x] Отдельный `run_id` + отдельный `.env` / compose-сервис для satellite (`.env.b.example` → `bot_b`)
- [ ] Primary 5m или 15m **уже** крутится отдельно (satellite не вместо)

| Параметр | Env | Default этого профиля |
|----------|-----|------------------------|
| `timeframe` | `DONCHIAN_TIMEFRAME` | **1m** |
| `bar_seconds` | `DONCHIAN_BAR_SECONDS` | **60** |
| `channel_n` | `DONCHIAN_CHANNEL_N` | **1800** |
| `exit_m` | `DONCHIAN_EXIT_M` | **900** |
| `atr_period` | `DONCHIAN_ATR_PERIOD` | **1200** |
| `min_breakout_atr` | `DONCHIAN_MIN_BREAKOUT_ATR` | **0.5** |
| `max_vol_rank` | `DONCHIAN_MAX_VOL_RANK` | **0.64** |
| `vol_rank_bars` | `DONCHIAN_VOL_RANK_BARS` | **1200** |
| `vol_rank_lookback` | `DONCHIAN_VOL_RANK_LOOKBACK` | **60000** |
| `max_bars_in_trade` | `DONCHIAN_MAX_BARS_IN_TRADE` | **840** (= 14h) |
| `risk_pct` | `DONCHIAN_RISK_PCT` | **1.0** |
| `max_risk_usd` | `DONCHIAN_MAX_RISK_USD` | **1000** |

`MAX_BARS_IN_TRADE=0` = tstop off (только shadow без tstop).

---

## 3. Спецификация (1:1 с research)

### 3.1 Entry — как 15m/5m brk+vol, другие окна

Порядок на signal-баре `i`: vol_rank skip, затем сила пробоя (оба должны пройти). Формулы — [`DONCHIAN_LIVE_STARTPACK_15M_BRK_VOL.md`](DONCHIAN_LIVE_STARTPACK_15M_BRK_VOL.md) §2.

| TF | `vol_rank_bars` (~20h) | `vol_rank_lookback` (~42d) |
|----|------------------------|----------------------------|
| 30m | 40 | 2000 |
| 15m | 80 | 4000 |
| 5m | 240 | 12000 |
| **1m** | **1200** | **60000** |

Warmup: ≥ `vol_rank_bars + vol_rank_lookback` баров до первого сигнала  
(`1200 + 60000 = 61200` × 1m ≈ **42.5 дня**). Буфер загрузки ≥ **60–120 дней** ок.

### 3.2 Time-stop 14h

Как в `strategies/donchian/trades.py`:

```text
bars_held = i - entry_idx
time_stop = max_bars_in_trade > 0 and bars_held >= max_bars_in_trade
# SL first; else channel exit OR time_stop → exit at open[i+1], outcome="time"
```

| TF | 14h в барах | `max_bars_in_trade` |
|----|------------:|--------------------:|
| 1h | 14 | 14 |
| 15m | 56 | 56 |
| 5m | 168 | 168 |
| **1m** | **840** | **840** |

На **этом** профиле только **1m → 840**. Не ставить 840 на 5m/15m.  
Не «выровнять» до 20h (=1200 bars на 1m) без нового research pass — multi-year pass был на **14h**.

Не ретюнить 14h / 0.64 / 0.5 / N/M по первым живым неделям.

### 3.3 Risk 1.0% — осознанный satellite-выбор

Research sniff/multi-year для R-018 шёл в основном @ **0.5%** (DD ближе к 5m@1%).  
Fair-compare и Calmar-grid @ slip2 показали максимум score на **1m @1%**.

Этот start-pack фиксирует **1.0%** как user pick для satellite (max edge / max DD).  
Soft alternate (меньше pain при том же TF): §5.2 risk **0.5%** — отдельный `run_id`, не «подорожнику» на том же счёте.

---

## 4. Default `.env` (satellite)

```dotenv
# --- профиль ---
DONCHIAN_TAG=1m_N1800_M900_ATR1200_R1.0_brk0.5_vol0.64_tstop14h_satellite

# --- рынок ---
DONCHIAN_SYMBOLS=BTCUSDT
# ETH — отдельный рукав / отдельный счёт / отдельное решение.

DONCHIAN_TIMEFRAME=1m
DONCHIAN_BAR_SECONDS=60

# --- канал (календарный эквивалент 1h N30 × 60) ---
DONCHIAN_CHANNEL_N=1800
DONCHIAN_EXIT_M=900
DONCHIAN_ATR_PERIOD=1200
DONCHIAN_ATR_METHOD=sma_tr
DONCHIAN_ATR_STOP_MULT=1.5
DONCHIAN_MAX_POSITIONS=1

# --- сайзинг ---
DONCHIAN_RISK_PCT=1.0
DONCHIAN_MAX_RISK_USD=1000
DONCHIAN_MAX_RISK_ENABLED=true

# --- издержки: в research fee/slip=0; в live — измерять ---
DONCHIAN_FEE_RATE_PER_SIDE=0
DONCHIAN_SLIPPAGE_RATE_PER_SIDE=0
# Stress paper: 2 bps/side = рабочий сценарий; 5 bps = зона HO-kill на research.

# --- live entry ---
DONCHIAN_LIVE_PROFILE=brk0.5+vol_rank
DONCHIAN_MIN_BREAKOUT_ATR=0.5
DONCHIAN_MAX_VOL_RANK=0.64
DONCHIAN_VOL_RANK_BARS=1200
DONCHIAN_VOL_RANK_LOOKBACK=60000

# --- time-stop 14h ---
DONCHIAN_MAX_BARS_IN_TRADE=840

# Shadow: brk+vol без tstop + optional baseline
DONCHIAN_SHADOW_BASELINE=true
DONCHIAN_SHADOW_BRK_VOL=true
DONCHIAN_SHADOW_MAX_BARS_IN_TRADE=0

# --- ls5 off ---
DONCHIAN_LS5_ENABLED=false
DONCHIAN_SHADOW_LS5=false

# --- запрещено research ---
DONCHIAN_PAUSE_DD_PCT=0
DONCHIAN_ADAPTIVE_RISK=false
DONCHIAN_INVERT_AFTER_LOSS=false
DONCHIAN_MIN_ADX=0
DONCHIAN_MIN_ATR_PCT=0
DONCHIAN_MIN_CHANNEL_ATR=0
```

Валидация при старте:

- `BAR_SECONDS == 60` при `TIMEFRAME=1m`
- `EXIT_M < CHANNEL_N`
- `CHANNEL_N == 1800`, `EXIT_M == 900`, `ATR_PERIOD == 1200` (или WARNING override)
- `VOL_RANK_BARS == 1200`, `VOL_RANK_LOOKBACK == 60000` при live 1m brk+vol
- `MAX_BARS_IN_TRADE == 840` при этом профиле (или WARNING)
- entry-фильтры как в 15m brk+vol start-pack
- WARNING если `TAG` не содержит `satellite` / отдельный account id не задан (ops-дисциплина)

---

## 5. Готовые пресеты

### 5.1 Этот профиль — 1m brk+vol + tstop 14h @1% (satellite live)

```dotenv
DONCHIAN_TAG=1m_N1800_M900_ATR1200_R1.0_brk0.5_vol0.64_tstop14h_satellite
DONCHIAN_TIMEFRAME=1m
DONCHIAN_BAR_SECONDS=60
DONCHIAN_CHANNEL_N=1800
DONCHIAN_EXIT_M=900
DONCHIAN_ATR_PERIOD=1200
DONCHIAN_ATR_STOP_MULT=1.5
DONCHIAN_RISK_PCT=1.0
DONCHIAN_MAX_RISK_USD=1000
DONCHIAN_MIN_BREAKOUT_ATR=0.5
DONCHIAN_MAX_VOL_RANK=0.64
DONCHIAN_VOL_RANK_BARS=1200
DONCHIAN_VOL_RANK_LOOKBACK=60000
DONCHIAN_LIVE_PROFILE=brk0.5+vol_rank
DONCHIAN_MAX_BARS_IN_TRADE=840
DONCHIAN_LS5_ENABLED=false
```

### 5.2 Soft satellite — тот же 1m, risk 0.5% (меньше DD, всё ещё edge)

```dotenv
DONCHIAN_TAG=1m_N1800_M900_ATR1200_R0.5_brk0.5_vol0.64_tstop14h_satellite
DONCHIAN_RISK_PCT=0.5
# остальное как §5.1
```

Research @0.5%: half≈1 bps, HO @2 bps ещё +, HO @5 bps уже слабый/красный. Мягче по DD fee=0, но cost buffer всё равно тонкий.

### 5.3 Shadow-only paper (без live orders)

Тот же §5.1, но execution = paper + обязательный slip stress 2 и 5 bps/side в учёте shadow.

### 5.4 Откат / стоп satellite

- Остановить 1m-бот; primary **не трогать**.
- Новый эксперимент = новый `run_id` (не «продолжить с теми же журналами после смены risk/tstop»).

---

## 6. Slip, fills и kill-switch

### 6.1 Research ladder @ risk 1% (якорь ожиданий)

| Slip (bps/side) | Avg EW | HO EW | Worst DD | Комментарий |
|----------------:|-------:|------:|---------:|-------------|
| 0 | +17824% | +23126% | −40% | fee=0 потолок |
| 1 | +10219% | +11978% | −68% | |
| **2** | **+4721%** | **+2626%** | **−83%** | рабочий stress |
| 3 | +2893% | +378% | −91% | |
| 4 | +2099% | +9% | −96% | HO почти ноль |
| **5** | +1540% | **−69%** | **−98%** | HO kill зона |
| 7 | +495% | −96% | −99% | avg ещё +, path мёртв |
| 10 | +26% | −100% | −100% | |

Round-trip ≈ **2×** slip/side.

### 6.2 Что логировать на каждой сделке

- signal mid / mark at decision
- fill price entry/exit
- **realized slip bps** = (fill − mark) / mark × 10000 (знак по стороне)
- latency signal→ack / ack→fill
- `bars_held`, `outcome` (`sl` / `channel` / `time`)
- equity, risk_usd, capped yes/no

Без этого satellite бесполезен: не отличить «edge жив» от «бэктест врал».

### 6.3 Kill-switch / review triggers (не авто-тюнинг)

Остановить или перевести в paper-only и разобрать, если:

| Триггер | Порог (ориентир) |
|---------|------------------|
| Median realized slip/side | устойчиво **> 2–3 bps** на достаточном N |
| Rolling realized slip | устойчиво **≥ 5 bps** → research HO-kill зона |
| Equity DD от пика satellite | **≲ −60…−70%** — допустимый research-хвост; решить заранее: переживать как write-off или стоп |
| Equity → ~0 / margin wipe | эксперимент завершён; **не** доливать молча |
| Primary здоров, satellite нет | **не** переносить size с primary на 1m «чтобы отбить» |

Пороги — ops-дисциплина, не новые research-gates в коде (не фризить `pause_dd` без отдельного R-).

---

## 7. Операционный порядок

1. Primary (15m и/или 5m) жив отдельно.
2. Env + brk/vol + `max_bars_in_trade` + 1m klines готовы (§2).
3. Выделить write-off баланс. Зафиксировать сумму письменно в run notes.
4. Новый `run_id`. `.env` из §4 / §5.1.
5. **Shadow-only** на 1m: baseline + brk+vol tstop=0 + paper tstop=840.  
   Сверить fills с research week (match ≥99% на paper без slip).
6. Shadow accounting stress **2 bps** и **5 bps**/side — сравнить с `dashboard_1m_tstop14h_slip_ladder_risk1`.
7. Live BTC only, cap on, `MAX_BARS_IN_TRADE=840`, tiny size.
8. Еженедельно: median slip bps, DD, N сделок, vs research ladder.  
   Не ретюнить N/M/ATR/tstop/risk по первым неделям.
9. ETH / size-up — только после стабильного slip-профиля и явного решения.

Откат: §5.4. Primary не откатывать «из-за» satellite.

---

## 8. Definition of done

- [x] `.env` из §4 поднимает **отдельный** бот на 1m + tstop14h без правки кода (`.env.b.example` → `bot_b`)
- [ ] Отдельный счёт / budget; PnL не смешан с 5m/15m
- [ ] Journal: `breakout_atr`, `vol_rank`, `skipped_reason`; на выходе `outcome` + `bars_held` / `max_bars_in_trade`
- [ ] Journal: mark vs fill → **realized slip bps** entry/exit
- [ ] `DONCHIAN_TAG=…_tstop14h_satellite` в journal и UI
- [ ] Shadow brk+vol **без** tstop совпадает с research 1m brk+vol
- [ ] Paper tstop=840 ≥99% match research fills той же недели (без slip)
- [ ] Kill-switch правила из §6.3 записаны в run notes до первого live-ордера
- [ ] Смена risk 1.0↔0.5 или tstop hours = новый `run_id`

---

## 9. Чего не делать

- Не считать этот профиль заменой 5m/15m default.
- Не запускать 1m на том же счёте/журнале, что primary.
- Не поднимать risk выше 1% «потому что Calmar-grid так сказал».
- Не копировать `MAX_BARS_IN_TRADE=240` с 5m (это 20h на 5m, на 1m = 4h).
- Не ставить 1200 bars «как 20h fair» без нового multi-year pass.
- Не игнорировать DD: линейный equity-график 2025 **прячет** −35% early-year DD у нуля оси.
- Не масштабировать после зелёной недели без slip-статистики.
- Не доливать после −70% «чтобы стратегия восстановилась» — это уже другой эксперимент.
- Не включать ls5 / DD-pause / adaptive risk / invert «заодно».
- Не выключать shadow и slip telemetry.

---

## 10. Ожидания (чтобы не удивиться)

| Сценарий | Что нормально |
|----------|----------------|
| Winrate | Низкий (~25–35% зона Donchian); прибыль от runners |
| Луз-стрики | Длинные — цена edge, не баг |
| DD @ fee≈0 | десятки %; на 2025 EW cap$1k ~−35% (часто рано в году) |
| DD @ slip≈2 | research worst year ~−80%+ |
| PnL vs 5m | Может сильно обогнать **или** почти обнулиться в плохом cost-режиме |
| Успех эксперимента | Не «+X% за месяц», а **измеренный slip + поведение vs ladder** |

---

## 11. Файлы research (якорь)

| Что | Где |
|-----|-----|
| Лог | `docs/RESEARCH_LOG.md` — **R-018** (1m кандидат tstop14h) |
| Slip @0.5% | `scripts/build_donchian_1m_tstop14h_slip_ladder.py` → `dashboard_1m_tstop14h_slip_ladder/` |
| Slip @1% | `scripts/build_donchian_1m_tstop14h_slip_ladder_risk1.py` → `dashboard_1m_tstop14h_slip_ladder_risk1/` |
| TF slip compare | `output/donchian_backtest/dashboard_tf_slip_compare_1m_3m_5m/` |
| TF×risk Calmar @slip2 | `scripts/build_donchian_tf_risk_calmar_slip2.py` → `dashboard_tf_risk_calmar_slip2/` |
| Equity 2025 overlay | `scripts/plot_donchian_tf_equity_2025_cap1k_ew.py` |
| Canvas | `donchian-1m-tstop14h-slip-ladder*.canvas.tsx`, `donchian-tf-risk-calmar-slip2.canvas.tsx` |
| Движок | `strategies/donchian/trades.py` — filters + `max_bars_in_trade` |
| Primary 5m candidate | [`DONCHIAN_LIVE_STARTPACK_5M_BRK_VOL_TSTOP20.md`](DONCHIAN_LIVE_STARTPACK_5M_BRK_VOL_TSTOP20.md) |
| Primary 15m default | [`DONCHIAN_LIVE_STARTPACK_15M_BRK_VOL_TSTOP48.md`](DONCHIAN_LIVE_STARTPACK_15M_BRK_VOL_TSTOP48.md) |

---

*Контракт значений: 1m N=1800 M=900 ATR(1200)×1.5 risk 1.0% · min_breakout_atr=0.5 · max_vol_rank=0.64 (bars=1200, lookback=60000) · max_bars_in_trade=840 (14h) · cap 1000 USDT · fee-shadow 0 · ls5 off. Роль: **satellite / write-off balance**, parallel to 5m/15m — не primary default. Контракт механизма: всё из `.env`; отдельный run_id; смена risk/tstop/TF = новый run_id; scale-up только после measured slip.*
