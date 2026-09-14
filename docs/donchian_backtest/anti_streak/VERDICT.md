# Anti-loss-streak: equity DD pause + breakout buffer

Base: **1h N=30 M=15 ATR×1.5**  
IS: **2020–2023**. Holdout: **2024–2026**.  
Script: `scripts/run_donchian_anti_streak.py`

## Verdict

| Idea | Result |
|------|--------|
| **Equity DD circuit breaker** | **Fails as edge.** Caps DD but skips most trades (incl. runners). IS collapses (+47% / +143% vs +947%). Not a path to beat streaks without killing the model. |
| **Min breakout ATR (esp. 1.0)** | **Useful risk filter, not a free lunch.** Fewer weak breakouts → higher WR/PF, lower holdout DD (−16% vs −34%), better Calmar. Absolute return lower (+102% vs +142%). |

**Recommendation:** keep primary core unchanged. Treat **`--min-breakout-atr 1.0`** as a candidate alternate if the goal is smoother equity / lower DD. Do **not** promote DD-pause.

## Holdout 2024–2026

| variant | return | maxDD | n | skipDD | WR | PF | Calmar |
|---------|-------:|------:|--:|-------:|---:|---:|-------:|
| baseline | 141.56% | -33.64% | 474 | 0 | 27.2% | 1.22 | 4.21 |
| **brk1.0** | **101.57%** | **-16.45%** | 208 | 0 | **32.2%** | **1.43** | **6.17** |
| brk0.5 | 121.84% | -34.84% | 342 | 0 | 27.8% | 1.31 | 3.50 |
| dd15_8 | 92.40% | -15.21% | 289 | 194 | 27.3% | 1.26 | 6.07 |
| dd20_10 | 80.66% | -20.39% | 304 | 178 | 26.6% | 1.22 | 3.96 |
| dd10+brk0.5 | 55.87% | -10.09% | 109 | 261 | 33.9% | 1.55 | 5.54 |
| dd10_5 | 21.99% | -10.70% | 85 | 422 | 29.4% | 1.30 | 2.05 |

## In-sample 2020–2023 (sanity)

| variant | return | maxDD | n | skipDD | PF | Calmar |
|---------|-------:|------:|--:|-------:|---:|-------:|
| baseline | 947.00% | -27.89% | 686 | 0 | 1.43 | 33.96 |
| brk1.0 | 679.93% | -21.08% | 293 | 0 | 1.88 | 32.26 |
| brk0.5 | 916.52% | -27.69% | 478 | 0 | 1.55 | 33.09 |
| dd20_10 | 142.02% | -20.24% | 227 | 504 | 1.46 | 7.02 |
| dd10_5 | 47.46% | -10.59% | 69 | 673 | 1.72 | 4.48 |

## Why the chart looks like that

Classic trend-follow: **many small −R in chop**, **few large +R in trends**. Equity-DD pause freezes entries right when recovery / next trend often starts. Breakout buffer only takes “decisive” channel breaks — fewer chop losses, still misses some runners.
