# ETHUSDT transfer check (native 1h)

Symbol: **ETHUSDT** (Binance spot; no native ETHUSD — USDT quote).  
Data: native **1h** archives under `data/binance/ethusdt_1h/` (2020–2026, through ~2026-09-08). **No 1m download.**  
Rules: **1h N=30 M=15 ATR×1.5**, risk 1% / $1000 cap, taker 0.05%/side.  
Same params as BTC checkpoint / frozen alternate B — **no re-optimization on ETH**.  
IS: **2020–2023**. Holdout: **2024–2026**.

## Verdict

Core Donchian **transfers to ETH** on this window — holdout is strong.  
Frozen `ls5_cond_brk2.0` **helps IS**, but on ETH holdout it is **slightly worse** than plain baseline (more DD, similar return). Do **not** force the BTC alternate onto ETH without its own check.

| | BTC holdout (ref) | ETH holdout |
|--|------------------:|------------:|
| baseline | +142% / DD −34% | **+236% / DD −15%** |
| ls5_cond_brk2.0 | +164% / DD −30% | +232% / DD −18% |

## Holdout 2024–2026

| variant | return | maxDD | n | WR | PF | Calmar |
|---------|-------:|------:|--:|---:|---:|-------:|
| baseline | 235.82% | -15.29% | 514 | 28.8% | 1.38 | 15.42 |
| ls5_cond_brk2.0 | 231.75% | -18.14% | 500 | 28.6% | 1.38 | 12.78 |

## In-sample 2020–2023

| variant | return | maxDD | n | WR | PF | Calmar |
|---------|-------:|------:|--:|---:|---:|-------:|
| ls5_cond_brk2.0 | 792.66% | -25.78% | 690 | 28.8% | 1.28 | 30.74 |
| baseline | 728.13% | -30.16% | 712 | 28.7% | 1.25 | 24.14 |

Charts: `output/donchian_backtest/ethusdt_transfer/ho_baseline/chart.png`, `ho_ls5_cond_brk2.0/chart.png`
