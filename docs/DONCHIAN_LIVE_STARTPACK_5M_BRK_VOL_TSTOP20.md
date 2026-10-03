# Donchian live: 5m brk+vol + tstop 20h

**Это продолжение** цепочки start-pack (env → core → 15m brk+vol), **не замена** 15m default.

Алгоритм бара, ATR SMA-TR, причинность канала, фильтры `brk0.5+vol_rank`, сайзинг, телеметрия — **те же правила**, что на 15m.  
Меняются TF / календарный N/M/ATR / окна vol-rank / **time-stop 20h**.

**Кандидатный 5m-профиль (2026-10, R-015 5m — user pick):**

| | |
|--|--|
| TF | **5m** (`bar_seconds = 300`) |
| Канал | **N=360 · M=180 · ATR(240)×1.5** (календарный эквивалент 1h N30 / 15m N120) |
| Risk | **1.0%** · cap как в 15m-профиле (625 USDT / 50k ₽ @ 80) |
| Live entry | **`brk0.5+vol_rank`** (`min_breakout_atr=0.5`, `max_vol_rank=0.64`) |
| Vol windows | **`vol_rank_bars=240`**, **`vol_rank_lookback=12000`** (~20h / ~42d) |
| Live exit add-on | **`tstop 20h`** → `max_bars_in_trade=240` |
| Shadow | обязательный **brk+vol без tstop** на том же 5m + optional baseline |
| ls5 | **OFF** |
| Не фризить | tstop 16h (bar-parity с 15m/48h) как «баланс»; 4h как smoother; wall-clock 48h (=576 bars) — отброшен |

**Отношение к 15m default:** live-default остаётся [`DONCHIAN_LIVE_STARTPACK_15M_BRK_VOL_TSTOP48.md`](DONCHIAN_LIVE_STARTPACK_15M_BRK_VOL_TSTOP48.md). Этот файл — отдельный 5m-кандидат (user pick по return/Calmar).

Research-якоря: **R-006/R-008** (5m brk+vol / порог 0.64), **R-015 (5m)** (лесенка tstop + user pick 20h), slip ladder `dashboard_5m_tstop20h_slip_ladder`.

---

## 0. Зачем 5m + tstop 20h

На 5m тот же gate `brk0.5+vol0.64`, что на 15m/30m. Лесенка tstop (R-015 5m) при risk 1%:

- Pass-зона: **2h–20h** (bars = hours × 12).
- **User pick: 20h (240 bars)** — макс return/Calmar среди pass.
- 16h (=192 bars, bar-count parity с 15m/48h) — баланс; 4h — smoother.
- Wall-clock **48h (576 bars)** на 5m — **не pass** (DD хуже): не копировать 15m-часы 1:1.

Сравнение vs 5m live без tstop (EW BTC+ETH, fee 0, fresh $1000/year):

| Окно | live brk+vol | **+ tstop 20h** |
|------|-------------:|----------------:|
| IS 2020–23 | +796% / −15.4% / 61.2 | **+1427% / −15.2% / 137** |
| HO 2024–26 | +877% / −21.7% / 44.7 | **+974% / −20.7% / 51.4** |
| 2017–26 avg | +899% / −15.2% / 72.8 | **+1324% / −15.3% / 108** |

**Slip (важно):** на 5m профиль чувствителен к издержкам. Slip ladder (fee 0, 0..20 bps/side): half-EW ≈ **4 bps**, kill ≈ **20 bps**; @5 bps avg EW ещё +, HO уже заметно слабее; @10 bps HO ≈ 0. Не поднимать size, пока shadow с реалистичным slip не зелёный.

---

## 1. Что должно быть уже сделано

Перед этим профилем в live-репо:

- [ ] Env-порт из [`DONCHIAN_LIVE_STARTPACK_30M_ENV.md`](DONCHIAN_LIVE_STARTPACK_30M_ENV.md)
- [ ] Entry-фильтры из [`DONCHIAN_LIVE_STARTPACK_15M_BRK_VOL.md`](DONCHIAN_LIVE_STARTPACK_15M_BRK_VOL.md) (логика brk/vol та же; на 5m другие окна)
- [ ] `max_bars_in_trade` в движке (как в 15m tstop48 start-pack)
- [ ] Отдельный `run_id` — не смешивать 15m journal / pause state с 5m

| Параметр | Env | Default этого профиля |
|----------|-----|------------------------|
| `timeframe` | `DONCHIAN_TIMEFRAME` | **5m** |
| `bar_seconds` | `DONCHIAN_BAR_SECONDS` | **300** |
| `channel_n` | `DONCHIAN_CHANNEL_N` | **360** |
| `exit_m` | `DONCHIAN_EXIT_M` | **180** |
| `atr_period` | `DONCHIAN_ATR_PERIOD` | **240** |
| `min_breakout_atr` | `DONCHIAN_MIN_BREAKOUT_ATR` | **0.5** |
| `max_vol_rank` | `DONCHIAN_MAX_VOL_RANK` | **0.64** |
| `vol_rank_bars` | `DONCHIAN_VOL_RANK_BARS` | **240** |
| `vol_rank_lookback` | `DONCHIAN_VOL_RANK_LOOKBACK` | **12000** |
| `max_bars_in_trade` | `DONCHIAN_MAX_BARS_IN_TRADE` | **240** (= 20h) |
| `risk_pct` | `DONCHIAN_RISK_PCT` | **1.0** |

`MAX_BARS_IN_TRADE=0` = tstop off (shadow без tstop).

---

## 2. Спецификация (1:1 с research)

### 2.1 Entry — как 15m brk+vol, другие окна

Порядок на signal-баре `i`: vol_rank skip, затем сила пробоя (оба должны пройти). Формулы — [`DONCHIAN_LIVE_STARTPACK_15M_BRK_VOL.md`](DONCHIAN_LIVE_STARTPACK_15M_BRK_VOL.md) §2.

| TF | `vol_rank_bars` (~20h) | `vol_rank_lookback` (~42d) |
|----|------------------------|----------------------------|
| 30m | 40 | 2000 |
| 15m | 80 | 4000 |
| **5m** | **240** | **12000** |

Warmup: ≥ `vol_rank_bars + vol_rank_lookback` баров до первого сигнала (~12240×5m ≈ **42.5 дня**); буфер ≥ **120 дней** ок.

### 2.2 Time-stop 20h

Как в `strategies/donchian/trades.py`:

```text
bars_held = i - entry_idx
time_stop = max_bars_in_trade > 0 and bars_held >= max_bars_in_trade
# SL first; else channel exit OR time_stop → exit at open[i+1], outcome="time"
```

| TF | 20h в барах | `max_bars_in_trade` |
|----|------------:|--------------------:|
| 1h | 20 | 20 |
| 30m | 40 | 40 |
| 15m | 80 | 80 |
| **5m** | **240** | **240** |

На **этом** профиле только **5m → 240**. Не ставить 240 на 15m (это другие часы). Не ставить 576 «как 48h с 15m».

Не ретюнить 20h по живым неделям. Другой горизонт (16h/4h) = отдельный checkpoint / shadow.

---

## 3. Default `.env`

```dotenv
# --- профиль ---
DONCHIAN_TAG=5m_N360_M180_ATR240_R1.0_brk0.5_vol0.64_tstop20h

# --- рынок ---
DONCHIAN_SYMBOLS=BTCUSDT
# ETH — отдельный рукав / отдельный счёт.

DONCHIAN_TIMEFRAME=5m
DONCHIAN_BAR_SECONDS=300

# --- канал (календарный эквивалент 1h N30) ---
DONCHIAN_CHANNEL_N=360
DONCHIAN_EXIT_M=180
DONCHIAN_ATR_PERIOD=240
DONCHIAN_ATR_METHOD=sma_tr
DONCHIAN_ATR_STOP_MULT=1.5
DONCHIAN_MAX_POSITIONS=1

# --- сайзинг ---
DONCHIAN_RISK_PCT=1.0
DONCHIAN_MAX_RISK_USD=625
DONCHIAN_MAX_RISK_ENABLED=true

# --- издержки shadow ---
DONCHIAN_FEE_RATE_PER_SIDE=0
DONCHIAN_SLIPPAGE_RATE_PER_SIDE=0
# Stress: 4–5 bps/side уже режет half-EW на research ladder — смотреть shadow.

# --- live entry ---
DONCHIAN_LIVE_PROFILE=brk0.5+vol_rank
DONCHIAN_MIN_BREAKOUT_ATR=0.5
DONCHIAN_MAX_VOL_RANK=0.64
DONCHIAN_VOL_RANK_BARS=240
DONCHIAN_VOL_RANK_LOOKBACK=12000

# --- time-stop 20h ---
DONCHIAN_MAX_BARS_IN_TRADE=240

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

- `BAR_SECONDS == 300` при `TIMEFRAME=5m`
- `EXIT_M < CHANNEL_N`
- `CHANNEL_N == 360`, `EXIT_M == 180`, `ATR_PERIOD == 240` (или WARNING override)
- `VOL_RANK_BARS == 240`, `VOL_RANK_LOOKBACK == 12000` при live 5m brk+vol
- `MAX_BARS_IN_TRADE == 240` при этом профиле (или WARNING)
- entry-фильтры как в 15m brk+vol start-pack

---

## 4. Готовые пресеты

### 4.1 Этот профиль — 5m brk+vol + tstop 20h

```dotenv
DONCHIAN_TAG=5m_N360_M180_ATR240_R1.0_brk0.5_vol0.64_tstop20h
DONCHIAN_TIMEFRAME=5m
DONCHIAN_BAR_SECONDS=300
DONCHIAN_CHANNEL_N=360
DONCHIAN_EXIT_M=180
DONCHIAN_ATR_PERIOD=240
DONCHIAN_ATR_STOP_MULT=1.5
DONCHIAN_RISK_PCT=1.0
DONCHIAN_MAX_RISK_USD=625
DONCHIAN_MIN_BREAKOUT_ATR=0.5
DONCHIAN_MAX_VOL_RANK=0.64
DONCHIAN_VOL_RANK_BARS=240
DONCHIAN_VOL_RANK_LOOKBACK=12000
DONCHIAN_LIVE_PROFILE=brk0.5+vol_rank
DONCHIAN_MAX_BARS_IN_TRADE=240
DONCHIAN_LS5_ENABLED=false
```

### 4.2 Shadow alternate — tstop 16h (баланс / bar-parity с 15m)

```dotenv
DONCHIAN_TAG=5m_N360_M180_ATR240_R1.0_brk0.5_vol0.64_tstop16h
DONCHIAN_MAX_BARS_IN_TRADE=192
# остальное как §4.1
```

### 4.3 Shadow alternate — tstop 4h (smoother)

```dotenv
DONCHIAN_TAG=5m_N360_M180_ATR240_R1.0_brk0.5_vol0.64_tstop4h
DONCHIAN_MAX_BARS_IN_TRADE=48
```

### 4.4 Откат на 15m live default

См. [`DONCHIAN_LIVE_STARTPACK_15M_BRK_VOL_TSTOP48.md`](DONCHIAN_LIVE_STARTPACK_15M_BRK_VOL_TSTOP48.md) §4.1. Новый `run_id`.

---

## 5. Что не догма с других pack’ов

| Место | Было | Стало (этот профиль) |
|-------|------|----------------------|
| Default TF live | 15m + tstop 48h | **5m + tstop 20h** только как **кандидатный** run; 15m остаётся primary default |
| Vol windows | 15m 80/4000 | **5m 240/12000** |
| Tstop | 15m 192 (=48h) | **5m 240 (=20h)** — user pick; не 576 |
| Slip budget | 15m терпимее | **half ~4 bps** — жёстче смотреть fills/slip |

---

## 6. Операционный порядок

1. Env + brk/vol + `max_bars_in_trade` готовы (§1).
2. Новый `run_id`. `.env` из §3 / §4.1.
3. **Shadow-only** 5m: baseline + brk+vol tstop=0 + paper tstop=240. Слой A зелёный на **5m** знаменателе.
4. Прогнать shadow с slip ≥ 4–5 bps/side (или live taker estimate) — сравнить с `dashboard_5m_tstop20h_slip_ladder`.
5. Live BTC only, cap on, `MAX_BARS_IN_TRADE=240`.
6. ETH — отдельным решением.
7. Не ретюнить 240 / 0.64 / 0.5 / N/M по первым неделям. Не «дотянуть» до 48h wall-clock.

Откат: §4.4 (15m default) или tstop=0 на 5m + новый run_id.

---

## 7. Definition of done

- [ ] `.env` из §3 поднимает бота на 5m + tstop20h без правки кода
- [ ] Journal: `breakout_atr`, `vol_rank`, `skipped_reason`; на выходе `outcome` + `bars_held` / `max_bars_in_trade`
- [ ] `DONCHIAN_TAG=…_tstop20h` в journal и UI
- [ ] Shadow brk+vol **без** tstop совпадает с research 5m brk+vol
- [ ] Paper/live tstop=240 ≥99% match research fills той же недели
- [ ] Слой A зелёный на **5m**; slip-stress не хуже research ladder @4–5 bps без сюрпризов
- [ ] Смена 15m↔5m или смена tstop hours = новый `run_id`

---

## 8. Чего не делать

- Не копировать `MAX_BARS_IN_TRADE=192` или `48` с 15m «как часы» без пересчёта баров.
- Не ставить 576 (48h wall-clock) — на 5m это failed шкалы.
- Не считать этот профиль заменой 15m default без отдельного ops-решения.
- Не поднимать risk выше 1% «потому что сделок больше на 5m».
- Не выключать shadow без tstop и без slip-stress.
- Не включать ls5 / N140 / vol0.55 в live «заодно».

---

## 9. Файлы research (якорь)

| Что | Где |
|-----|-----|
| Лог | `docs/RESEARCH_LOG.md` — **R-015 (5m)** (лесенка + user pick 20h) |
| Slip ladder | `scripts/build_donchian_5m_tstop20h_slip_ladder_dashboard.py` |
| Артефакты | `output/donchian_backtest/dashboard_5m_tstop20h_slip_ladder/` |
| Canvas | `donchian-5m-tstop20h-slip-ladder.canvas.tsx` |
| Tstop ladder script | `scripts/build_donchian_tstop_ladder_5m.py` |
| Движок | `strategies/donchian/trades.py` — filters + `max_bars_in_trade` |
| Primary live (15m) | [`DONCHIAN_LIVE_STARTPACK_15M_BRK_VOL_TSTOP48.md`](DONCHIAN_LIVE_STARTPACK_15M_BRK_VOL_TSTOP48.md) |

---

*Контракт значений: 5m N=360 M=180 ATR(240)×1.5 risk 1.0% · min_breakout_atr=0.5 · max_vol_rank=0.64 (bars=240, lookback=12000) · max_bars_in_trade=240 (20h) · cap 625 USDT · fee-shadow 0 · ls5 off. Кандидатный профиль (user pick); primary live default = 15m tstop48h. Контракт механизма: всё из `.env`; смена профиля = новый run_id.*
