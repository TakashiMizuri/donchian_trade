# Conditional cooldown — resume breakout sweep

Base: **1h N=30 M=15 ATR×1.5** + `ls5_24`  
Sweep: `cooldown_resume_min_breakout_atr ∈ {1.00, 1.25, 1.50, 1.75, 2.00}`  
IS: **2020–2023**. Holdout: **2024–2026**.  
Script: `scripts/run_donchian_cond_resume_sweep.py`

## Verdict

**Not a lonely spike at 1.5.** The whole band **1.25–2.0** beats fixed `ls5_24` on holdout Calmar. Effect is a **plateau**, not a knife-edge.

| Candidate | Holdout | Notes |
|-----------|---------|-------|
| **cond_brk2.00** | +164% / DD −30.0% / Calmar **5.47** | Best Calmar IS+HO; slightly fewer early resumes |
| cond_brk1.50 | +164% / DD −30.8% / Calmar 5.34 | Prior pick; nearly identical return |
| cond_brk1.25 | +161% / −30.8% / 5.24 | Softer resume, still useful |
| ls5_24 fixed | +150% / −29.5% / 5.08 | No early resume |
| baseline | +142% / −33.6% / 4.21 | — |

**Freeze recommendation:** prefer **`2.0`** over `1.5` as the conditional-resume default (slightly better risk-adjusted, IS also best). Difference vs 1.5 is small — either is fine as alternate; don’t overfit within the band.

## Holdout 2024–2026

| variant | return | maxDD | n | WR | PF | Calmar |
|---------|-------:|------:|--:|---:|---:|-------:|
| cond_brk2.00 | 163.92% | -29.95% | 456 | 27.4% | 1.25 | 5.47 |
| cond_brk1.50 | 164.41% | -30.77% | 458 | 27.5% | 1.25 | 5.34 |
| cond_brk1.25 | 161.39% | -30.77% | 459 | 27.5% | 1.25 | 5.24 |
| cond_brk1.75 | 160.82% | -30.77% | 457 | 27.4% | 1.24 | 5.23 |
| cond_brk1.00 | 157.88% | -30.77% | 462 | 27.5% | 1.24 | 5.13 |
| ls5_24 | 149.93% | -29.54% | 454 | 27.5% | 1.24 | 5.08 |
| baseline | 141.56% | -33.64% | 474 | 27.2% | 1.22 | 4.21 |

## In-sample 2020–2023

| variant | return | maxDD | n | WR | PF | Calmar |
|---------|-------:|------:|--:|---:|---:|-------:|
| cond_brk2.00 | 1141.18% | -25.94% | 660 | 26.4% | 1.53 | 43.99 |
| cond_brk1.50 / 1.75 | 1128.06% | -25.94% | 661 | 26.3% | 1.53 | 43.48 |
| ls5_24 | 1109.56% | -25.94% | 659 | 26.4% | 1.53 | 42.77 |
| cond_brk1.00 | 1079.30% | -26.97% | 667 | 26.1% | 1.50 | 40.03 |
| cond_brk1.25 | 1059.19% | -27.00% | 666 | 26.1% | 1.50 | 39.22 |
| baseline | 947.00% | -27.89% | 686 | 25.9% | 1.43 | 33.96 |

Charts: `output/donchian_backtest/cond_resume_sweep/ho_cond_brk2.00/chart.png`
