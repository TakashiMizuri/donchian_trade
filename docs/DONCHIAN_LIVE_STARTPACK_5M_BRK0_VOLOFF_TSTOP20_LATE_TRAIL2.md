# Donchian live: 5m brk off + vol off + tstop 20h + late trail 18h/2R (кандидат)

**Это продолжение** [`DONCHIAN_LIVE_STARTPACK_5M_BRK_VOL_TSTOP20_LATE_TRAIL.md`](DONCHIAN_LIVE_STARTPACK_5M_BRK_VOL_TSTOP20_LATE_TRAIL.md) (R-024), **не замена** 15m default.

Алгоритм бара, ATR SMA-TR, канал, tstop 20h, сайзинг — **без изменений**.  
Меняются входы и тугость trail: **brk off, vol-rank off, trail 2R** (было brk0.5 + vol0.64 + trail 3R).

**Актуальный 5m-кандидат (2026-11-04, R-026, user freeze):**

| | |
|--|--|
| База | 5m N=360 M=180 ATR(240)×1.5 · **brk off** · **vol-rank off** · tstop 20h (`max_bars_in_trade=240`) · risk **1.0%** · cap 625 USDT |
| Live exit add-on | **late trail** `on=216` bars (18h) · **`trail=2R`** · `min_mfe=6R` |
| Роль | **кандидатный 5m-run**, parallel to 15m primary |
| Shadow | обязательный paper **R-024** (brk0.5+vol0.64 + trail 3R) на том же 5m |
| ls5 / BE / scale-out | **OFF** |
| Не фризить | возврат brk/vol «на глаз»; trail 3R на этом book; naive trail с бара 0; копия 1m 6R/15 |

Research-якоря: **R-015 (5m)** (база tstop 20h), **R-024** (late trail 18h/3R на brk+vol), **R-026** (joint: фильтры off + 2R; 4/4 vs R-024; пресэмпл 2017–20; BNB+SOL @2bps).

**Отношение к 15m default:** live-default остаётся [`DONCHIAN_LIVE_STARTPACK_15M_BRK_VOL_TSTOP48.md`](DONCHIAN_LIVE_STARTPACK_15M_BRK_VOL_TSTOP48.md).  
1m satellite — свой pack (`720 / 6 / 15`).

---

## 0. Зачем снимать фильтры и зажимать trail

Модули на 5m укладывали по одному. С trail в движке vol 0.64 **не** IS-argmax (0.80 и без трейла, и с ним) — среди vol-ручек frozen 0.64 остаётся 4-cell по умолчанию. Joint как на 1m: снять **оба** фильтра и **ужесточить** trail. Vol-off + **3R** = 3/4 (HO ETH −21R). **2R / MFE6** закрывает 4 клетки.

vs frozen R-024 (path-correct, fee 0, BTC+ETH):

| | n | ΔIS / ΔHO | 4-cell | yearly EW | worst DD |
|--|--:|----------:|:------:|----------:|---------:|
| R-024 brk0.5 vol0.64 trail 3R | 4369 | — | — | +2501% | −22.4% |
| **R-026 brk0 vol-off trail 2R** | **7405** | **+890 / +310** | **4/4** | **+8896%** | **−24.6%** |

Валидация (те же knobs, без ретюна):

| Окно | slip | Avg EW prod → trail 3R → **joint 2R** | Joint worst DD |
|------|------|----------------------------------------|----------------|
| BTC+ETH 2020–26 | 0 | +1233% → +2485% → **+8896%** | −24.6% |
| BTC+ETH 2020–26 | 2 bps | +804% → +1612% → **+4818%** | −35.7% |
| BTC+ETH 2017–20 | 2 bps | +1544% → +2277% → **+7371%** | −22.5% |
| **BNB+SOL 2020–26** | 2 bps | +195% → +607% → **+1726%** | −57.5% |

Порядок среднего года держится. Цена: больше сделок (~1.7×), DD чуть глубже frozen trail (на 2017 listing и BNB+SOL 2020 listing — заметно). 2024 BNB+SOL joint всё ещё выше trail по return (+204% vs +153%), DD хуже (−43.5% vs −21.1%).

Yearly % — компаунд $1000/год @1% research cap $1000, не live-ожидание. Live cap этого профиля = 625.

---

## 1. Что должно быть уже сделано

- [x] 5m-база из [`DONCHIAN_LIVE_STARTPACK_5M_BRK_VOL_TSTOP20.md`](DONCHIAN_LIVE_STARTPACK_5M_BRK_VOL_TSTOP20.md)
- [x] R-024 pack как shadow (brk+vol + 3R)
- [x] Движок: `late_trail_*`; `min_breakout_atr=0` и `max_vol_rank=0` = off

| Параметр | Env | Default этого профиля |
|----------|-----|------------------------|
| `min_breakout_atr` | `DONCHIAN_MIN_BREAKOUT_ATR` | **0** (off) |
| `max_vol_rank` | `DONCHIAN_MAX_VOL_RANK` | **0** (off) |
| `late_trail_on_bars` | `DONCHIAN_LATE_TRAIL_ON_BARS` | **216** |
| `late_trail_r` | `DONCHIAN_LATE_TRAIL_R` | **2** |
| `late_trail_min_mfe` | `DONCHIAN_LATE_TRAIL_MIN_MFE` | **6** |
| tstop / канал / risk | как 5m tstop20h | N360/M180/A240×1.5, tstop 240, risk 1.0% |

`MAX_VOL_RANK=0` = vol-фильтр выключен (не «пропускать rank 0»).  
`LATE_TRAIL_ON_BARS=0` = trail выключен.

---

## 2. Спецификация late trail (как R-024, другой R)

Peak для стопа — **прошлый бар**. Orig ATR-стоп первый на том же баре.

```text
bars_held = i - entry_idx
prev_peak_r = (peak - entry) / risk     # mirrored for SELL

if bars_held >= 216 and prev_peak_r >= 6 and not orig_SL_hit:
    trail_stop = peak - 2 * risk        # + for SELL
    if wick through trail_stop:
        exit NOW at trail_stop, outcome="time"

# иначе: orig SL → channel OR tstop 240 → open[i+1]
```

На **этом** профиле только **5m → 216 / 2R / MFE 6**. Не копировать 2R на 1m/15m. Не ретюнить 216/2/6 по живым неделям.

Fill trail = wick через lagged peak±2R.

---

## 3. Default `.env`

```dotenv
DONCHIAN_TAG=5m_N360_M180_ATR240_R1.0_brk0_voloff_tstop20h_late18h_2R

DONCHIAN_MAX_BARS_IN_TRADE=240

DONCHIAN_MIN_BREAKOUT_ATR=0
DONCHIAN_MAX_VOL_RANK=0
DONCHIAN_LIVE_PROFILE=core

DONCHIAN_LATE_TRAIL_ON_BARS=216
DONCHIAN_LATE_TRAIL_R=2
DONCHIAN_LATE_TRAIL_MIN_MFE=6
```

Валидация при старте:

- `MIN_BREAKOUT_ATR == 0`, `MAX_VOL_RANK == 0`
- `LATE_TRAIL_ON_BARS == 216`, `LATE_TRAIL_R == 2`, `LATE_TRAIL_MIN_MFE == 6` (или WARNING override)
- `LATE_TRAIL_ON_BARS < MAX_BARS_IN_TRADE`
- WARNING если `LATE_TRAIL_R == 3` на этом профиле (это R-024)
- WARNING если `LATE_TRAIL_ON_BARS == 720` на 5m (это 1m-часы)

---

## 4. Готовые пресеты

### 4.1 Этот профиль (R-026)

```dotenv
DONCHIAN_TAG=5m_N360_M180_ATR240_R1.0_brk0_voloff_tstop20h_late18h_2R
DONCHIAN_TIMEFRAME=5m
DONCHIAN_BAR_SECONDS=300
DONCHIAN_CHANNEL_N=360
DONCHIAN_EXIT_M=180
DONCHIAN_ATR_PERIOD=240
DONCHIAN_ATR_STOP_MULT=1.5
DONCHIAN_RISK_PCT=1.0
DONCHIAN_MAX_RISK_USD=625
DONCHIAN_MIN_BREAKOUT_ATR=0
DONCHIAN_MAX_VOL_RANK=0
DONCHIAN_LIVE_PROFILE=core
DONCHIAN_MAX_BARS_IN_TRADE=240
DONCHIAN_LATE_TRAIL_ON_BARS=216
DONCHIAN_LATE_TRAIL_R=2
DONCHIAN_LATE_TRAIL_MIN_MFE=6
DONCHIAN_LS5_ENABLED=false
```

Vol-окна можно оставить в env (240 / 12000) — при `MAX_VOL_RANK=0` движок их не применяет.

### 4.2 Shadow / откат — R-024

См. [`DONCHIAN_LIVE_STARTPACK_5M_BRK_VOL_TSTOP20_LATE_TRAIL.md`](DONCHIAN_LIVE_STARTPACK_5M_BRK_VOL_TSTOP20_LATE_TRAIL.md) §4.1.  
`MIN_BREAKOUT_ATR=0.5`, `MAX_VOL_RANK=0.64`, `LATE_TRAIL_R=3`, `LATE_TRAIL_MIN_MFE=8`. Новый `run_id`.

### 4.3 Откат на 15m live default

См. [`DONCHIAN_LIVE_STARTPACK_15M_BRK_VOL_TSTOP48.md`](DONCHIAN_LIVE_STARTPACK_15M_BRK_VOL_TSTOP48.md) §4.1. Новый `run_id`.

---

## 5. Операционный порядок

1. Primary 15m жив отдельно. 5m — кандидатный run, свой `run_id`.
2. Новый `run_id`. `.env` из §3 / §4.1.
3. Shadow-only: paper **R-024** vs paper **R-026** на одной неделе. Match ≥99% с research fills без slip.
4. Slip-stress как в 5m tstop20h pack (≥ 4–5 bps/side) — больше имён, тот же тонкий cost-buffer, не шире.
5. Live BTC only сначала, cap on. Не смешивать journal с R-024.
6. Не возвращать brk/vol и не ставить 3R «потому что DD». Не ретюнить 216/2/6 / tstop 240 по первым неделям.

Откат: §4.2 или §4.3.

---

## 6. Definition of done

- [x] `.env` из §3 поднимает бота на 5m: brk0, vol off, tstop240, late trail 2R
- [x] Journal: `outcome` + `bars_held`; trail-выход = `time` (fill = trail_stop, не next open)
- [x] `DONCHIAN_TAG=…_brk0_voloff_…_late18h_2R`
- [x] Shadow R-024 совпадает с предыдущим 5m-кандидатом
- [ ] Paper ≥99% match research той же недели (без slip)
- [ ] Смена brk/vol/trail R = новый `run_id`

---

## 7. Чего не делать

- Не включать brk/vol обратно без нового 4-cell **и** слип + чужой символ.
- Не копировать 2R на 1m/15m (у 1m свой 6R; у 15m нет этого фриза).
- Не поднимать size из-за +8896% fee-0 — под 2 bps год уже +4818%, DD глубже.
- Не считать BNB+SOL 2024 (DD −43.5%) поводом крутить knobs на живой неделе.
- Не считать этот профиль заменой 15m default без ops-решения.

---

## 8. Файлы research

| Что | Где |
|-----|-----|
| Лог | `docs/RESEARCH_LOG.md` — **R-026** |
| Joint сетка | `scripts/run_donchian_5m_joint_retune.py` → `joint_retune_5m_trail/` |
| Stack check | `scripts/run_donchian_5m_joint_stack_check.py` |
| Yearly BTC/ETH | `scripts/build_donchian_5m_late_trail_yearly_dashboard.py` |
| Yearly BNB/SOL | тот же скрипт `--symbols BNBUSDT SOLUSDT --tag bnb_sol --slip-bps 2` |
| Canvas | `donchian-5m-tstop20h-vs-late-trail-yearly.canvas.tsx` · `…-slip2` · `…-2017-2020` · `donchian-5m-bnb-sol-vs-late-trail-yearly-slip2.canvas.tsx` |
| Shadow | [`DONCHIAN_LIVE_STARTPACK_5M_BRK_VOL_TSTOP20_LATE_TRAIL.md`](DONCHIAN_LIVE_STARTPACK_5M_BRK_VOL_TSTOP20_LATE_TRAIL.md) |
| База | [`DONCHIAN_LIVE_STARTPACK_5M_BRK_VOL_TSTOP20.md`](DONCHIAN_LIVE_STARTPACK_5M_BRK_VOL_TSTOP20.md) |
| 1m analogue | [`DONCHIAN_LIVE_STARTPACK_1M_BRK0_VOLOFF_TSTOP14_LATE_TRAIL6.md`](DONCHIAN_LIVE_STARTPACK_1M_BRK0_VOLOFF_TSTOP14_LATE_TRAIL6.md) |

---

*Контракт значений: 5m N=360 M=180 ATR(240)×1.5 risk 1.0% · min_breakout_atr=0 · max_vol_rank=0 · max_bars_in_trade=240 (20h) · late_trail_on_bars=216 · late_trail_r=2 · late_trail_min_mfe=6 · cap 625 USDT · fee-shadow 0 · ls5 off. Кандидатный профиль (user freeze R-026); primary live default = 15m tstop48h. Контракт механизма: всё из `.env`; смена brk/vol/trail = новый run_id.*
