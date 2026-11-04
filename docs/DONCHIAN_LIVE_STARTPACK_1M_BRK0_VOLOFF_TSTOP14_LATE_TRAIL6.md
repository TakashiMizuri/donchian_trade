# Donchian live: 1m brk off + vol off + tstop 14h + late trail 12h/6R (satellite)

**Это продолжение** [`DONCHIAN_LIVE_STARTPACK_1M_BRK_VOL_TSTOP14_LATE_TRAIL.md`](DONCHIAN_LIVE_STARTPACK_1M_BRK_VOL_TSTOP14_LATE_TRAIL.md) (R-023), **не замена** primary 15m/5m.

Алгоритм бара, ATR SMA-TR, канал, tstop 14h, сайзинг, satellite-рамка — **без изменений**.  
Меняются входы и тугость trail: **brk off, vol-rank off, trail 6R** (было brk0.5 + vol0.64 + trail 8R).

**Актуальный satellite-профиль (2026-11-04, R-025, user freeze):**

| | |
|--|--|
| База | 1m N=1800 M=900 ATR(1200)×1.5 · **brk off** · **vol-rank off** · tstop 14h (`max_bars_in_trade=840`) · risk **1.0%** · cap 1000 USDT |
| Live exit add-on | **late trail** `on=720` bars (12h) · **`trail=6R`** · `min_mfe=15R` |
| Роль | **отдельный бот + write-off баланс**, parallel to 5m/15m |
| Shadow | обязательный paper **R-023** (brk0.5+vol0.64 + trail 8R) на том же 1m |
| ls5 / BE / scale-out | **OFF** |
| Не фризить | возврат brk/vol «на глаз»; trail 8R на этом book; naive trail с бара 0; 5m/15m копия 6R |

Research-якоря: **R-018** (база tstop14h), **R-023** (late trail 12h/8R на brk+vol), **R-025** (joint: фильтры off + 6R; 4/4 vs R-023; пресэмпл 2017–20; BNB+SOL).

---

## 0. Зачем снимать фильтры и зажимать trail

Модули на 1m укладывали по одному. С trail в движке ранжирование меняется: vol-off выигрывает IS sum R (больше раннеров на harvest), но vol-off + **8R** — 3/4 (IS ETH −18R). **6R** переворачивает ETH. brk 0.5 резал чоп, пока выходы были −1R; с late trail слабый брейк, который доезжает до 12h, ценен.

vs frozen R-023 (path-correct, fee 0, BTC+ETH):

| | n | ΔIS / ΔHO | 4-cell | yearly EW | worst DD |
|--|--:|----------:|:------:|----------:|---------:|
| R-023 brk0.5 vol0.64 trail 8R | 8273 | — | — | +29795% | −35.7% |
| **R-025 brk0 vol-off trail 6R** | **14280** | **+1658 / +949** | **4/4** | **+60500%** | **−39.4%** |

Валидация (те же knobs, без ретюна):

| Окно | slip | Avg EW prod → trail 8R → **joint 6R** | Joint worst DD |
|------|------|----------------------------------------|----------------|
| BTC+ETH 2020–26 | 0 | +17824% → +29809% → **+60500%** | −39.4% |
| BTC+ETH 2020–26 | 2 bps | +4721% → +11209% → **+22011%** | −74.6% |
| BTC+ETH 2017–20 | 2 bps | +17754% → +26980% → **+48495%** | −39.4% |
| **BNB+SOL 2020–26** | 2 bps | +469% → +1637% → **+6383%** | −79.9% |

Порядок среднего года держится. Цена: больше сделок (~1.7×), глубже DD. 2024 на BNB+SOL trail 8R чуть выше joint. SOL часто не лучше R-023 — EW тянет BNB.

Yearly % — компаунд $1000/год @1%, не live-ожидание. 1m satellite = write-off.

---

## 1. Что должно быть уже сделано

- [x] Satellite-база из [`DONCHIAN_LIVE_STARTPACK_1M_BRK_VOL_TSTOP14.md`](DONCHIAN_LIVE_STARTPACK_1M_BRK_VOL_TSTOP14.md)
- [x] R-023 pack как shadow (brk+vol + 8R)
- [x] Движок: `late_trail_*`; `min_breakout_atr=0` и `max_vol_rank=0` = off

| Параметр | Env | Default этого профиля |
|----------|-----|------------------------|
| `min_breakout_atr` | `DONCHIAN_MIN_BREAKOUT_ATR` | **0** (off) |
| `max_vol_rank` | `DONCHIAN_MAX_VOL_RANK` | **0** (off) |
| `late_trail_on_bars` | `DONCHIAN_LATE_TRAIL_ON_BARS` | **720** |
| `late_trail_r` | `DONCHIAN_LATE_TRAIL_R` | **6** |
| `late_trail_min_mfe` | `DONCHIAN_LATE_TRAIL_MIN_MFE` | **15** |
| tstop / канал / risk | как 1m tstop14h | N1800/M900/A1200×1.5, tstop 840, risk 1.0% |

`MAX_VOL_RANK=0` = vol-фильтр выключен (не «пропускать rank 0»).  
`LATE_TRAIL_ON_BARS=0` = trail выключен.

---

## 2. Спецификация late trail (как R-023, другой R)

Peak для стопа — **прошлый бар**. Orig ATR-стоп первый на том же баре.

```text
bars_held = i - entry_idx
prev_peak_r = (peak - entry) / risk     # mirrored for SELL

if bars_held >= 720 and prev_peak_r >= 15 and not orig_SL_hit:
    trail_stop = peak - 6 * risk        # + for SELL
    if wick through trail_stop:
        exit NOW at trail_stop, outcome="time"

# иначе: orig SL → channel OR tstop 840 → open[i+1]
```

На **этом** профиле только **1m → 720 / 6R / MFE 15**. Не копировать 6R на 5m (там свой 2R pack, R-026). Не ретюнить 720/6/15 по живым неделям.

---

## 3. Default `.env` (satellite)

```dotenv
DONCHIAN_TAG=1m_N1800_M900_ATR1200_R1.0_brk0_voloff_tstop14h_late12h_6R_satellite

DONCHIAN_MAX_BARS_IN_TRADE=840

DONCHIAN_MIN_BREAKOUT_ATR=0
DONCHIAN_MAX_VOL_RANK=0
DONCHIAN_LIVE_PROFILE=core

DONCHIAN_LATE_TRAIL_ON_BARS=720
DONCHIAN_LATE_TRAIL_R=6
DONCHIAN_LATE_TRAIL_MIN_MFE=15
```

Валидация при старте:

- `MIN_BREAKOUT_ATR == 0`, `MAX_VOL_RANK == 0`
- `LATE_TRAIL_ON_BARS == 720`, `LATE_TRAIL_R == 6`, `LATE_TRAIL_MIN_MFE == 15` (или WARNING override)
- `LATE_TRAIL_ON_BARS < MAX_BARS_IN_TRADE`
- WARNING если `TAG` не содержит `satellite`
- WARNING если `LATE_TRAIL_R == 8` на этом профиле (это R-023)

---

## 4. Готовые пресеты

### 4.1 Этот профиль (R-025)

```dotenv
DONCHIAN_TAG=1m_N1800_M900_ATR1200_R1.0_brk0_voloff_tstop14h_late12h_6R_satellite
DONCHIAN_TIMEFRAME=1m
DONCHIAN_BAR_SECONDS=60
DONCHIAN_CHANNEL_N=1800
DONCHIAN_EXIT_M=900
DONCHIAN_ATR_PERIOD=1200
DONCHIAN_ATR_STOP_MULT=1.5
DONCHIAN_RISK_PCT=1.0
DONCHIAN_MAX_RISK_USD=1000
DONCHIAN_MIN_BREAKOUT_ATR=0
DONCHIAN_MAX_VOL_RANK=0
DONCHIAN_LIVE_PROFILE=core
DONCHIAN_MAX_BARS_IN_TRADE=840
DONCHIAN_LATE_TRAIL_ON_BARS=720
DONCHIAN_LATE_TRAIL_R=6
DONCHIAN_LATE_TRAIL_MIN_MFE=15
DONCHIAN_LS5_ENABLED=false
```

Vol-окна можно оставить в env (1200 / 60000) — при `MAX_VOL_RANK=0` движок их не применяет.

### 4.2 Shadow / откат — R-023

См. [`DONCHIAN_LIVE_STARTPACK_1M_BRK_VOL_TSTOP14_LATE_TRAIL.md`](DONCHIAN_LIVE_STARTPACK_1M_BRK_VOL_TSTOP14_LATE_TRAIL.md) §4.1.  
`MIN_BREAKOUT_ATR=0.5`, `MAX_VOL_RANK=0.64`, `LATE_TRAIL_R=8`. Новый `run_id`.

---

## 5. Операционный порядок

1. Primary 15m/5m жив отдельно. Satellite-счёт и write-off budget как в 1m-pack §1.
2. Новый `run_id`. `.env` из §3 / §4.1.
3. Shadow-only: paper **R-023** vs paper **R-025** на одной неделе. Match ≥99% с research fills без slip.
4. Live BTC only сначала, cap on. Не смешивать journal с R-023.
5. Не возвращать brk/vol и не ставить 8R «потому что DD». Не ретюнить 720/6/15 / tstop 840 по первым неделям.
6. Kill-switch / slip — те же пороги, что 1m-pack §6. Больше сделок = тот же тонкий cost-buffer, не шире.

---

## 6. Definition of done

- [x] `.env` из §3 поднимает **отдельный** 1m-бот: brk0, vol off, tstop840, late trail 6R
- [x] Journal: `outcome` + `bars_held`; trail-выход = `time` (fill = trail_stop)
- [x] `DONCHIAN_TAG=…_brk0_voloff_…_late12h_6R_satellite`
- [x] Shadow R-023 совпадает с предыдущим satellite
- [ ] Paper ≥99% match research той же недели (без slip)
- [ ] Смена brk/vol/trail R = новый `run_id`

---

## 7. Чего не делать

- Не включать brk/vol обратно без нового 4-cell **и** слип + чужой символ.
- Не копировать 6R на 5m/15m (5m = R-026 `216/2/6`).
- Не поднимать size из-за +60500% fee-0 — под 2 bps год уже другой, DD −75% класс.
- Не считать BNB+SOL 2024 (trail > joint) поводом крутить knobs на живой неделе.
- Не делать этот профиль primary. Роль — satellite / write-off.

---

## 8. Файлы research

| Что | Где |
|-----|-----|
| Лог | `docs/RESEARCH_LOG.md` — **R-025** |
| Joint сетка | `scripts/run_donchian_1m_joint_retune.py` → `joint_retune_1m_trail/` |
| Stack check | `scripts/run_donchian_1m_joint_stack_check.py` |
| Yearly BTC/ETH | `scripts/build_donchian_1m_late_trail_yearly_dashboard.py` |
| Yearly BNB/SOL | тот же скрипт `--symbols BNBUSDT SOLUSDT --tag bnb_sol --slip-bps 2` |
| Canvas | `donchian-1m-tstop14h-vs-late-trail-yearly.canvas.tsx` · `…-slip2` · `…-2017-2020` · `donchian-1m-bnb-sol-vs-late-trail-yearly-slip2.canvas.tsx` |
| Shadow | [`DONCHIAN_LIVE_STARTPACK_1M_BRK_VOL_TSTOP14_LATE_TRAIL.md`](DONCHIAN_LIVE_STARTPACK_1M_BRK_VOL_TSTOP14_LATE_TRAIL.md) |
| База | [`DONCHIAN_LIVE_STARTPACK_1M_BRK_VOL_TSTOP14.md`](DONCHIAN_LIVE_STARTPACK_1M_BRK_VOL_TSTOP14.md) |

---

*Контракт значений: 1m N=1800 M=900 ATR(1200)×1.5 risk 1.0% · min_breakout_atr=0 · max_vol_rank=0 · max_bars_in_trade=840 (14h) · late_trail_on_bars=720 · late_trail_r=6 · late_trail_min_mfe=15 · cap 1000 USDT · fee-shadow 0 · ls5 off. Роль: **satellite / write-off**. Контракт механизма: всё из `.env`; отдельный run_id; смена brk/vol/trail = новый run_id.*
