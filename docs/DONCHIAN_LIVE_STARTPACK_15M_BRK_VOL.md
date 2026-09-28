# Donchian live: переход на 15m + brk0.5+vol_rank

**Это продолжение** [`DONCHIAN_LIVE_STARTPACK.md`](DONCHIAN_LIVE_STARTPACK.md) и [`DONCHIAN_LIVE_STARTPACK_30M_ENV.md`](DONCHIAN_LIVE_STARTPACK_30M_ENV.md), не замена.

Алгоритм бара, ATR SMA-TR, причинность канала, один слот, сайзинг §5, телеметрия §17 — **без изменений**.  
Меняется **default live-профиль**: TF, календарный N/M/ATR, и два входных фильтра (сила пробоя + squeeze vol-rank).

**Новый default live-профиль (2026-09, Lighter fee ≈ 0):**

| | |
|--|--|
| TF | **15m** (`bar_seconds = 900`) |
| Канал | **N=120 · M=60 · ATR(80)×1.5** (календарный эквивалент 1h N30) |
| Risk | **1.0%** · cap как в 30m-env (625 USDT / 50k ₽ @ 80) |
| Live entry filter | **`brk0.5+vol_rank`** |
| Shadow | обязательный **baseline** (без фильтров) на том же 15m |
| ls5 | **OFF** в live; optional shadow only (на 30m уже почти бесполезен) |

Research-якоря: **R-002** (`min_breakout_atr=0.5`), **R-004/R-005** (vol_rank threshold **0.64**), **R-007** (transfer на 15m @ 1% — связка лучший smoother).

---

## 0. Зачем уходить с 30m baseline

30m @ 1% baseline был правильным шагом после 1h (fee≈0 на Lighter, календарный N). Дальше research показал:

1. **`min_breakout_atr=0.5`** (R-002) режет слабые пробои → лучше DD / Calmar без убийства return.
2. **`max_vol_rank=0.64`** (R-004 → R-005) пропускает входы только из «сжатой» волы → меньше шумных breakout’ов после уже раздутой волы.
3. На **15m @ 1%** (R-007) связка **`brk0.5+vol_rank`** подтвердилась как на 30m: выше return и ниже DD vs baseline на holdout и на всей выборке.

Holdout 2024–2026, EW BTC+ETH, fee 0, fresh $1000/year (из `dashboard_volrank_15m_risk1`):

| Вариант | Avg ret | Avg DD | Calmar |
|---------|--------:|-------:|-------:|
| baseline | +235% | −19.8% | 12.4 |
| brk0.5 | +270% | −16.7% | 16.3 |
| vol_rank0.64 | +289% | −16.6% | 17.3 |
| **brk0.5+vol_rank** | **+294%** | **−13.9%** | **21.2** |

All-years: связка **+359% / −14.1% / Calmar 30.3**, 4/7 лет лучше baseline.  
Плотность: ~**0.9 сделок/день** на портфель BTC+ETH (baseline ~1.5); P(month+) ≈ **80%**.

**Core-логика канала не меняется.** Фильтры — только gate на вход. Не тюнить порог 0.64 под 15m/HO (R-008: лестница на 5m оставила 0.64).

---

## 1. Что должно быть уже сделано (из 30m-env)

Перед этим переходом в live-репо **обязательно**:

- [ ] Все TF / N / M / ATR / risk / cap читаются из `.env` (30m-env §1, §6)
- [ ] Parity 1h N30 и 30m N60 зелёные
- [ ] Смена TF = новый `run_id`; старый `pause_until_bar_index` / journal не смешиваются

Если env-порта ещё нет — сначала [`DONCHIAN_LIVE_STARTPACK_30M_ENV.md`](DONCHIAN_LIVE_STARTPACK_30M_ENV.md), потом этот файл.

Дополнительно для этого профиля движок должен принимать (как research `run_strategy`):

| Параметр | Env | Default этого профиля |
|----------|-----|------------------------|
| `min_breakout_atr` | `DONCHIAN_MIN_BREAKOUT_ATR` | **0.5** |
| `max_vol_rank` | `DONCHIAN_MAX_VOL_RANK` | **0.64** (`0` = off) |
| `vol_rank_bars` | `DONCHIAN_VOL_RANK_BARS` | **80** на 15m |
| `vol_rank_lookback` | `DONCHIAN_VOL_RANK_LOOKBACK` | **4000** на 15m |

`0` для `MIN_BREAKOUT_ATR` / `MAX_VOL_RANK` = фильтр выключен (нужно для shadow baseline).

---

## 2. Спецификация фильтров (1:1 с research)

Оба фильтра применяются **на signal-баре `i`**, после того как close пробил N-канал, **до** постановки ордера на `open[i+1]`. Порядок в research: сначала vol_rank skip, затем проверка силы пробоя (оба должны пройти).

### 2.1 `min_breakout_atr = 0.5` (R-002)

Вместо «close за каналом» требуем запас в ATR:

```text
atr_i = ATR(i)          # SMA of TR, period ATR_PERIOD; бар i как в start-pack §4
upper_n / lower_n       # канал по [i-N, i) — бар i НЕ входит

if close[i] > upper_n + 0.5 * atr_i:  → BUY candidate
elif close[i] < lower_n - 0.5 * atr_i: → SELL candidate
else: skip
```

### 2.2 `max_vol_rank = 0.64` (R-004 / R-005)

**Squeeze / vol percentile** — каузально на баре `i`:

```text
vol_pct[j] = std(close[j-vol_bars+1 … j]) / close[j]     # rolling std / close
rank_input[j] = vol_pct[j-1]                             # shift(1) — без lookahead
vol_rank[i] = percentile_rank(rank_input[i] среди
                 rank_input[i-lookback+1 … i])           # pct rank в [0,1]

if vol_rank[i] is NaN (не прогрето): skip
if vol_rank[i] >= 0.64: skip   # «уже шумная» вола — не входим
# иначе вход разрешён (при выполнении остальных условий)
```

Календарные окна (заморожены, не ретюнить):

| TF | `vol_rank_bars` (~20h) | `vol_rank_lookback` (~42d) |
|----|------------------------|----------------------------|
| 30m | 40 | 2000 |
| **15m** | **80** | **4000** |
| 5m | 240 | 12000 |

Warmup для vol_rank: нужно ≥ `vol_rank_bars + vol_rank_lookback` баров **до** первого сигнала. На 15m это ~4080 баров ≈ **42.5 дня**; буфер ≥ **120 дней** из start-pack по-прежнему ок.

### 2.3 Профиль `brk0.5+vol_rank`

Вход только если:

1. `vol_rank < 0.64` (и конечен), **и**
2. close за каналом минимум на `0.5 × ATR`.

Выход, стоп, сайзинг, max_positions=1 — как core. Пирамида / ADX / ls5 / DD-pause — **OFF**.

---

## 3. Default `.env` (заменить 30m-блок)

```dotenv
# --- профиль ---
DONCHIAN_TAG=15m_N120_M60_ATR80_R1.0_brk0.5_vol0.64

# --- рынок ---
DONCHIAN_SYMBOLS=BTCUSDT
# ETH — отдельный рукав / отдельный счёт, либо явный split risk. Не два full 1% на одном equity.

DONCHIAN_TIMEFRAME=15m
DONCHIAN_BAR_SECONDS=900

# --- канал (календарный эквивалент 1h N30 / 30m N60) ---
DONCHIAN_CHANNEL_N=120
DONCHIAN_EXIT_M=60
DONCHIAN_ATR_PERIOD=80
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

# --- live entry profile ---
# baseline | brk0.5 | vol_rank | brk0.5+vol_rank
DONCHIAN_LIVE_PROFILE=brk0.5+vol_rank
DONCHIAN_MIN_BREAKOUT_ATR=0.5
DONCHIAN_MAX_VOL_RANK=0.64
DONCHIAN_VOL_RANK_BARS=80
DONCHIAN_VOL_RANK_LOOKBACK=4000

# Shadow: baseline без фильтров; twin (brk/vol) = DONCHIAN_SHADOW_BRK_VOL
SHADOW_BASELINE=true
DONCHIAN_SHADOW_BRK_VOL=true

# --- запрещено research ---
DONCHIAN_PAUSE_DD_PCT=0
DONCHIAN_ADAPTIVE_RISK=false
DONCHIAN_INVERT_AFTER_LOSS=false
DONCHIAN_MIN_ADX=0
DONCHIAN_MIN_ATR_PCT=0
DONCHIAN_MIN_CHANNEL_ATR=0
```

Валидация при старте (вдобавок к 30m-env §2):

- `EXIT_M < CHANNEL_N`
- `BAR_SECONDS == 900` при `TIMEFRAME=15m`
- `0 ≤ MIN_BREAKOUT_ATR` (0 = off)
- `0 ≤ MAX_VOL_RANK ≤ 1` (0 = off)
- если `MAX_VOL_RANK > 0`: `VOL_RANK_BARS ≥ 2`, `VOL_RANK_LOOKBACK ≥ 2`
- при `LIVE_PROFILE=brk0.5+vol_rank`: `MIN_BREAKOUT_ATR=0.5` и `MAX_VOL_RANK=0.64` (или явный override с WARNING в логе)
- `ATR_METHOD == sma_tr`, `MAX_POSITIONS == 1`

---

## 4. Готовые пресеты (шпаргалка)

### 4.1 Default сейчас — 15m risk 1.0% + brk+vol

```dotenv
DONCHIAN_TAG=15m_N120_M60_ATR80_R1.0_brk0.5_vol0.64
DONCHIAN_TIMEFRAME=15m
DONCHIAN_BAR_SECONDS=900
DONCHIAN_CHANNEL_N=120
DONCHIAN_EXIT_M=60
DONCHIAN_ATR_PERIOD=80
DONCHIAN_ATR_STOP_MULT=1.5
DONCHIAN_RISK_PCT=1.0
DONCHIAN_MAX_RISK_USD=625
DONCHIAN_MIN_BREAKOUT_ATR=0.5
DONCHIAN_MAX_VOL_RANK=0.64
DONCHIAN_VOL_RANK_BARS=80
DONCHIAN_VOL_RANK_LOOKBACK=4000
DONCHIAN_LIVE_PROFILE=brk0.5+vol_rank
DONCHIAN_SHADOW_BRK_VOL=true
```

### 4.2 Откат на предыдущий default — 30m baseline

```dotenv
DONCHIAN_TAG=30m_N60_M30_ATR40_R1.0
DONCHIAN_TIMEFRAME=30m
DONCHIAN_BAR_SECONDS=1800
DONCHIAN_CHANNEL_N=60
DONCHIAN_EXIT_M=30
DONCHIAN_ATR_PERIOD=40
DONCHIAN_ATR_STOP_MULT=1.5
DONCHIAN_RISK_PCT=1.0
DONCHIAN_MIN_BREAKOUT_ATR=0
DONCHIAN_MAX_VOL_RANK=0
DONCHIAN_LIVE_PROFILE=baseline
```

Новый `run_id`. Не тащить 15m pause/journal state.

### 4.3 Shadow-only сравнение на 15m (без live)

Тот же TF/N/M, но:

```dotenv
DONCHIAN_LIVE_PROFILE=baseline          # или paper-only
DONCHIAN_MIN_BREAKOUT_ATR=0
DONCHIAN_MAX_VOL_RANK=0
DONCHIAN_SHADOW_BASELINE=true
DONCHIAN_SHADOW_BRK_VOL=true            # shadow с 0.5 / 0.64
```

---

## 5. Что в предыдущих start-pack’ах больше не догма

| Место | Было | Стало |
|-------|------|--------|
| 30m-env default TF | 30m N60/M30/ATR40 | **15m N120/M60/ATR80** |
| 30m-env live profile | `baseline` | **`brk0.5+vol_rank`** |
| start-pack §1.1 `min_breakout_atr` | 0 (off) | **0.5** в live; 0 в shadow baseline |
| start-pack / 30m-env | vol_rank не было | **`max_vol_rank=0.64`**, окна §2.2 |
| 30m-env §3.3 «15m только после…» | исследование, не default | **устарело** — 15m+связка теперь default-кандидат |
| ls5 | shadow на 30m | по-прежнему **не** live default |

§3 state machine, §4 ATR, §5 sizing, §17 A/B/C — действуют. Знаменатель §17 = **15m** свечи выбранного профиля, не 1h и не 30m dashboard.

---

## 6. Операционный порядок в live-репо

1. Убедиться, что env-порт из 30m-env готов (§1).
2. Добавить в движок `min_breakout_atr` + `max_vol_rank` (+ bars/lookback), если ещё нет. Unit-тест: на фикстуре 15m 2024 BTC список входов `brk0.5+vol_rank` ≥99% match research.
3. Новый `run_id`. Поставить `.env` из §3 / §4.1.
4. **Shadow-only** 15m: baseline + brk_vol параллельно, пока слой A (§17.6) не зелёный на 15m klines.
5. Live BTC only, cap on, `LIVE_PROFILE=brk0.5+vol_rank`.
6. ETH — отдельным решением (не удваивать full 1% на одном счёте).
7. Не ретюнить `0.64` / `0.5` / N/M по первым живым неделям. Другой порог = новый research-checkpoint (как R-008), не «докрутить месяц».

Откат: §4.2 + новый run_id.

---

## 7. Definition of done

- [ ] `.env` из §3 поднимает бота на 15m без правки кода
- [ ] В journal на каждом сигнале: `breakout_atr`, `vol_rank`, `skipped_reason` (`brk` / `vol_rank` / `ok`)
- [ ] `DONCHIAN_TAG` в каждой строке journal и в шапке UI §17
- [ ] Shadow baseline на 15m совпадает с research baseline (хотя бы 2024 BTC) в пороге start-pack §15
- [ ] Shadow/live `brk0.5+vol_rank` совпадает с research-списком той же недели
- [ ] Слой A зелёный на **15m** знаменателе до увеличения size
- [ ] Warmup ≥ max(N, M, ATR, vol_bars+lookback); после рестарта нет сигналов «с полупустым» vol_rank
- [ ] Смена профиля 30m↔15m или baseline↔brk_vol = новый `run_id`

---

## 8. Чего не делать

- Не ставить на 15m сырой `N=30` (это другой горизонт, не календарный эквивалент).
- Не включать ls5 «на всякий случай» в live 15m.
- Не выключать shadow baseline — без него нельзя отличить баг фильтра от рынка.
- Не поднимать risk выше 1.0% «потому что сделок меньше» — плотность уже заложена в R-007; risk — отдельный рычаг.
- Не переносить 30m `pause_until_bar_index` / open position на 15m run.
- Не считать 5m default: на 5m (R-006) картина другая (чистый vol_rank лучше связки по return); это отдельный checkpoint.

---

## 9. Файлы research (якорь)

| Что | Где |
|-----|-----|
| Лог | `docs/RESEARCH_LOG.md` — R-002, R-004, R-005, R-007 |
| Дашборд 15m | `scripts/build_donchian_volrank_15m_dashboard.py` |
| Артефакты | `output/donchian_backtest/dashboard_volrank_15m_risk1/` |
| Canvas | `donchian-volrank-15m.canvas.tsx` |
| Движок | `strategies/donchian/trades.py` — `squeeze_vol_rank`, `min_breakout_atr`, `max_vol_rank` |

---

*Контракт значений: 15m N=120 M=60 ATR(80)×1.5 risk 1.0% · min_breakout_atr=0.5 · max_vol_rank=0.64 (bars=80, lookback=4000) · cap 625 USDT · fee-shadow 0 · ls5 off. Контракт механизма: всё из `.env`; смена профиля = новый run_id.*
