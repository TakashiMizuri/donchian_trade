# Donchian live: конфиг через `.env` (продолжение start-pack)

**Это продолжение** [`DONCHIAN_LIVE_STARTPACK.md`](DONCHIAN_LIVE_STARTPACK.md), не замена.

Алгоритм бара, ATR SMA-TR, причинность канала, один слот, ls5, телеметрия §17 — **без изменений**.  
Меняется только то, что в start-pack было зашито как «всегда 1h / N=30 / M=15 / ATR(20)».

**Задача порта:** вынести TF, N, M, ATR, риск и cap в `.env`. После этого смена профиля = правка env + рестарт, без правки кода.

**Новый default live-профиль (2026-09, Lighter fee ≈ 0):** `30m` · `N=60` · `M=30` · `ATR period=40` · `ATR stop ×1.5` · `risk=1.0%`.

---

## 0. Зачем не «просто переключить на 30m»

Start-pack замораживал **1h N=30** потому что на taker 0.05% нижние TF умирали fee-bleed’ом. На Lighter комиссия ≈ 0, и это ограничение снято.

Но N/M/ATR period — это **число баров**, не часов. Скопировать `N=30` на 30m — другая стратегия (вдвое короче горизонт). Живой 30m должен быть **календарным эквивалентом** 1h N=30/M=15/ATR(20):

| TF | `bar_seconds` | N | M | ATR period | тот же календарь, что 1h N30 |
|----|---------------|---|---|------------|------------------------------|
| 1h | 3600 | 30 | 15 | 20 | эталон start-pack |
| **30m** | **1800** | **60** | **30** | **40** | **default сейчас** |
| 15m | 900 | 120 | 60 | 80 | исследование, не default |
| 5m | 300 | 360 | 180 | 240 | исследование, не default |

`ATR stop mult = 1.5` **не** масштабируется (безразмерный).  
ls5 `pause_bars=24` на 1h = **24 часа**. На другом TF пауза в барах = `24h / bar_duration`, иначе это уже не тот alternate.

Если захардкодить в боте `interval=30m` и `N=60`, через месяц нельзя будет откатиться на 1h или попробовать 15m без нового деплоя. Поэтому **единственный** правильный порт: все эти числа читаются из env.

---

## 1. Что решить в коде live-репо (один раз)

Движок стратегии уже должен принимать параметры (как research `run_strategy(..., channel_n, exit_m, atr_period, atr_stop_mult, bar_seconds, risk_pct, ...)`). Доделать обвязку:

1. **Никакого** `TIMEFRAME = "1h"` / `N = 30` в исходниках. Только `os.environ` / pydantic-settings.
2. Подписка на kline: `DONCHIAN_TIMEFRAME` (Binance-style: `1h`, `30m`, `15m`, `5m`).
3. Решения по-прежнему **только на closed candle** этого TF (start-pack §3, §11.2).
4. Warmup в барах: `max(N, M, ATR_PERIOD)`; буфер в днях лучше оставить ≥120, как в start-pack.
5. `bar_seconds` либо из env, либо таблица от TF — не вычислять «на глаз» в трёх местах по-разному.
6. Рестарт: journal + OHLC того TF, что в env. Смена TF без сброса state = другой канал на тех же `pause_until_bar_index`. При смене TF — новый run id / сброс ls5-state.
7. Shadow (§17) ест **тот же** TF, что live. Знаменатель «1h dashboard» по-прежнему запрещён.

После этого вы в том проекте только правите `.env`.

---

## 2. Default `.env` (скопировать в live-репо)

Имена можно префиксовать как удобно (`DONCHIAN_` ниже — чтобы не пересечься с биржевыми ключами). Типы: числа без единиц в имени, единицы в комментарии.

```dotenv
# --- профиль (человекочитаемый тег в журнале / UI) ---
DONCHIAN_TAG=30m_N60_M30_ATR40_R1.0

# --- рынок ---
DONCHIAN_SYMBOLS=BTCUSDT
# Позже: BTCUSDT,ETHUSDT  — но каждый рукав = полный risk% только если это РАЗНЫЕ счета.
# На одном счёте два символа = либо split equity, либо risk%/2. Research-рукава были full-capital каждый.

DONCHIAN_TIMEFRAME=30m
DONCHIAN_BAR_SECONDS=1800

# --- канал / стоп (календарный эквивалент 1h N=30/M=15/ATR(20)×1.5) ---
DONCHIAN_CHANNEL_N=60
DONCHIAN_EXIT_M=30
DONCHIAN_ATR_PERIOD=40
DONCHIAN_ATR_METHOD=sma_tr
DONCHIAN_ATR_STOP_MULT=1.5
DONCHIAN_MAX_POSITIONS=1
DONCHIAN_MIN_BREAKOUT_ATR=0

# --- сайзинг ---
DONCHIAN_RISK_PCT=1.0
# Cap одной сделки. 0 = выкл.
# 50_000 RUB / 80 = 625 USDT. На Lighter считайте в USDT (или в USD notional).
DONCHIAN_MAX_RISK_USD=625
DONCHIAN_MAX_RISK_ENABLED=true

# --- издержки модели shadow (live fees — с биржи) ---
# Research 1h замораживал 0.05% taker. На Lighter для shadow ставьте 0.
DONCHIAN_FEE_RATE_PER_SIDE=0
DONCHIAN_SLIPPAGE_RATE_PER_SIDE=0

# --- профиль входов ---
# baseline | ls5_cond_brk2.0
DONCHIAN_LIVE_PROFILE=baseline
DONCHIAN_SHADOW_BASELINE=true
DONCHIAN_SHADOW_LS5=true

# ls5: пауза в ЧАСАХ канонична; бары считаются от TF.
DONCHIAN_LS5_LOSS_STREAK_N=5
DONCHIAN_LS5_PAUSE_HOURS=24
# Если задан — побеждает hours (не оставляйте оба «на глаз»).
# 30m → 48; 15m → 96; 5m → 288; 1h → 24
DONCHIAN_LS5_PAUSE_BARS=48
DONCHIAN_LS5_RESUME_MIN_BREAKOUT_ATR=2.0

# --- запрещено research, пусть env это фиксирует явно ---
DONCHIAN_PAUSE_DD_PCT=0
DONCHIAN_ADAPTIVE_RISK=false
DONCHIAN_INVERT_AFTER_LOSS=false
DONCHIAN_MIN_ADX=0
DONCHIAN_MIN_ATR_PCT=0
DONCHIAN_MIN_CHANNEL_ATR=0
```

Валидация при старте (упасть, не торговать молча):

- `EXIT_M < CHANNEL_N`
- `TIMEFRAME` ∈ {`1h`,`30m`,`15m`,`5m`} (или ваш белый список)
- `BAR_SECONDS` совпадает с TF (1800 для 30m и т.д.)
- `LS5_PAUSE_BARS * BAR_SECONDS == LS5_PAUSE_HOURS * 3600` (если заданы оба)
- `ATR_METHOD == sma_tr`
- `MAX_POSITIONS == 1`

---

## 3. Готовые пресеты (вставить вместо блока канала/TF/риска)

Код не должен содержать пресеты как единственный путь — только env. Ниже шпаргалка, что вставить.

### 3.1 Default сейчас — 30m risk 1.0% (рекомендация)

```dotenv
DONCHIAN_TAG=30m_N60_M30_ATR40_R1.0
DONCHIAN_TIMEFRAME=30m
DONCHIAN_BAR_SECONDS=1800
DONCHIAN_CHANNEL_N=60
DONCHIAN_EXIT_M=30
DONCHIAN_ATR_PERIOD=40
DONCHIAN_ATR_STOP_MULT=1.5
DONCHIAN_RISK_PCT=1.0
DONCHIAN_MAX_RISK_USD=625
DONCHIAN_LS5_PAUSE_BARS=48
DONCHIAN_LIVE_PROFILE=baseline
```

Почему не 1.5%: на непрерывном RUB DCA 2023–2026 (cap 50k ₽, fee 0) BTC 30m **1.0%** ≈ 19,2 млн (+751%, DD **−14%**) против 1.5% ≈ 22,1 млн (+881%, DD **−27%**). Почти те же деньги, просадка ближе к 1h. Кап 50k ₽ при 1% включается после ~5 млн ₽ equity, не после 3,33 млн.

Live start-pack §1.1 и так ставил **risk 1.0%** (не 1.5% yearly-дашборда). 1.5% на дашборде — карта формы, не live KPI (§17.1).

### 3.2 Откат на эталон start-pack — 1h

```dotenv
DONCHIAN_TAG=1h_N30_M15_ATR1.5
DONCHIAN_TIMEFRAME=1h
DONCHIAN_BAR_SECONDS=3600
DONCHIAN_CHANNEL_N=30
DONCHIAN_EXIT_M=15
DONCHIAN_ATR_PERIOD=20
DONCHIAN_ATR_STOP_MULT=1.5
DONCHIAN_RISK_PCT=1.0
DONCHIAN_LS5_PAUSE_BARS=24
```

### 3.3 15m / 5m — только после того, как 30m live совпал с shadow

Не default. Спред не моделировался. Календарный N/M/ATR обязателен (таблица §0).  
`LS5_PAUSE_BARS`: 96 (15m), 288 (5m).

---

## 4. Что в start-pack больше не является догмой

Читать start-pack с поправками:

| Место | Было | Стало |
|-------|------|--------|
| §1.1 TF / N / M / ATR period | 1h / 30 / 15 / 20 | **из env**; default 30m / 60 / 30 / 40 |
| §1.1 risk % | 1.0 (live) | **из env**; default 1.0 |
| §1.1 max risk USD | 1000 | **из env**; default 625 USDT (= 50k ₽ @ 80) |
| §1.2 live default ls5 | включать ls5 | default **baseline**; ls5 по-прежнему в shadow. На 30m RUB DCA ls5 почти не помогал |
| §2.2 / §11 clock | «1h kline» | kline **`DONCHIAN_TIMEFRAME`** |
| §8 `pause_bars=24` | 24 бара 1h | **24 часа** → `PAUSE_BARS` от TF |
| §9 «15m/30m core» | отвергнуто из-за fee-bleed | **устарело для Lighter**, если N/M/ATR **календарно** смасштабированы. Сырой N=30 на 5m по-прежнему нельзя |
| §13 yaml | зашитый 1h | тот же смысл, значения из `.env` |
| §17 знаменатель | те же 1h свечи | те же свечи **выбранного TF** |

§4 ATR, §3 state machine, §5 sizing, §17 A/B/C — действуют как написано.

---

## 5. Сайзинг и cap (чтобы env не врал)

Формула start-pack §5 без изменений:

```text
raw = equity * (RISK_PCT / 100)
risk_usd = min(raw, MAX_RISK_USD)  если MAX_RISK_ENABLED и MAX_RISK_USD > 0
else risk_usd = raw
qty = risk_usd / risk_distance
```

При `RISK_PCT=1.0` и cap 625 USDT кап биндится, когда `equity > 62_500 USDT` (~5 млн ₽ @ 80).  
При 1.5% тот же cap биндился бы раньше (~41_667 USDT). Это и есть смысл 1% + cap: больше пути на полном проценте, меньше хвостовая просадка.

`MAX_RISK_ENABLED=false` или `MAX_RISK_USD=0` — чистые 1% от equity. В research без капа 30m взрывается компаундом; для live не включать, пока сознательно не решите.

Lighter: `FEE_RATE_PER_SIDE=0` в **shadow**. Live комиссии — факт биржи (если реально 0, слой B не должен видеть 5 bps «модели VIP0»).

---

## 6. Definition of done для «я просто поменяю `.env`»

- [ ] Смена `DONCHIAN_TIMEFRAME` меняет websocket/REST interval без правки Python/TS
- [ ] Смена `CHANNEL_N` / `EXIT_M` / `ATR_PERIOD` / `ATR_STOP_MULT` / `RISK_PCT` видна в первой же сделке журнала (`atr_signal`, `stop_px`, `risk_usd`)
- [ ] `DONCHIAN_TAG` пишется в каждую строку journal и в шапку экрана §17.4.1
- [ ] При старте лог: полный dump env-профиля (без секретов биржи)
- [ ] Тест: тот же engine + env 1h N30 даёт ≥99% match с research-фикстурой start-pack §15
- [ ] Тест: env 30m N60 на сохранённых 30m klines воспроизводит research-список (хотя бы 2024 BTC baseline) с тем же порогом
- [ ] Смена TF требует нового `run_id`; старый `pause_until_bar_index` не переносится молча
- [ ] Два символа на одном счёте не получают independently full `RISK_PCT`, пока это явно не задано (`DONCHIAN_SYMBOLS` из одного тикера — default)

---

## 7. Операционный порядок в том репо

1. Завезти чтение env + валидацию (§2). Убрать хардкод 1h/N=30.
2. Прогнать parity на **старом** пресете 1h — убедиться, что порт не сломан.
3. Поставить default `.env` из §3.1 (30m / 1.0% / N60).
4. Shadow-only на 30m, пока слой A не зелёный (start-pack §17.6).
5. Live BTC only, cap включён, baseline.
6. ETH / 15m / другой risk — снова только env, новый run id.

Не подкручивать N/M по первым live-неделям (start-pack §9, §17.16). Другой TF = другой checkpoint, не «дотюнить 30m до красивого месяца».

---

*Контракт значений по умолчанию: 30m N=60 M=30 ATR(40)×1.5 risk 1.0% cap 625 USDT fee-shadow 0. Контракт механизма: всё это — переменные окружения.*
