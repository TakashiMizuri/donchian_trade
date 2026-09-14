# Pre-sample 2017–2019 (before design window)

## Data note

**2016 is not available** on Binance Vision for `BTCUSDT` / `ETHUSDT` (spot listing ~**2017-08-17**). Fetch of 2016 monthly zips returns 404.
Earliest blind pre-sample: **2017-08-17 → 2019-12-31**.
Design window was **2020–2023**; holdout **2024–2026**. So 2017–2019 is out-of-design.

Core: **1h N=30 M=15 ATR×1.5** · `$10k` · risk `1%` · cap `$1000` · fee `0.05%`
Script: `scripts/run_donchian_presample_2017_2019.py`

## Verdict

Baseline: **6/6** year×symbol cells with return>0 and PF≥1.0.
Full years only (2018–2019): **4/4** profitable PF≥1 cells.

**Yes — looks like a real trend-follow edge**, not only 2020+ luck: pre-design years are mostly profitable under the same frozen params. Caveat: 2017 is a short listing stub; crypto regimes change; live costs/funding still apply.

## Results

| year | symbol | variant | return | maxDD | n | WR | PF | Calmar |
|------|--------|---------|-------:|------:|--:|---:|---:|-------:|
| 2017* | BTCUSDT | baseline | 40.8% | -10.6% | 60 | 31.7% | 1.91 | 3.85 |
| 2017* | BTCUSDT | ls5_cond_brk2.0 | 41.1% | -10.6% | 59 | 30.5% | 1.92 | 3.88 |
| 2017* | ETHUSDT | baseline | 17.6% | -10.9% | 64 | 29.7% | 1.39 | 1.61 |
| 2017* | ETHUSDT | ls5_cond_brk2.0 | 16.1% | -10.0% | 63 | 30.2% | 1.37 | 1.61 |
| 2018 | BTCUSDT | baseline | 49.0% | -13.0% | 179 | 26.8% | 1.33 | 3.76 |
| 2018 | BTCUSDT | ls5_cond_brk2.0 | 53.6% | -12.8% | 176 | 26.7% | 1.35 | 4.18 |
| 2018 | ETHUSDT | baseline | 206.7% | -10.5% | 178 | 29.8% | 2.05 | 19.75 |
| 2018 | ETHUSDT | ls5_cond_brk2.0 | 188.3% | -10.5% | 174 | 29.9% | 2.04 | 17.99 |
| 2019 | BTCUSDT | baseline | 133.6% | -10.8% | 146 | 28.8% | 1.87 | 12.34 |
| 2019 | BTCUSDT | ls5_cond_brk2.0 | 127.3% | -13.2% | 144 | 27.8% | 1.85 | 9.68 |
| 2019 | ETHUSDT | baseline | 15.4% | -20.6% | 163 | 25.8% | 1.12 | 0.75 |
| 2019 | ETHUSDT | ls5_cond_brk2.0 | 10.2% | -20.5% | 161 | 25.5% | 1.08 | 0.50 |

\* 2017 = partial (listing → year-end).

## How to read this

If a strategy only worked in the window you tuned on, pre-sample collapses. Here params were frozen on **2020–2023**; **2017–2019 were not used to pick N/M/ATR**.
Yearly dashboard (fee 0%, fresh $1k) already showed strong 2018–2019 — this run uses **research fees** for a stricter check.

