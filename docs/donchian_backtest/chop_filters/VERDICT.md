# Chop / streak levers: regime, time-stop, loss-streak pause

Base: **1h N=30 M=15 ATR×1.5**  
IS: **2020–2023**. Holdout: **2024–2026**.  
Script: `scripts/run_donchian_chop_filters.py`

## Verdict

| Family | Holdout takeaway |
|--------|------------------|
| **ADX / ATR% / channel-width** | **Reject.** Cuts return hard; ADX25 ≈ flat; ATR% filters go near/below zero. |
| **Min inside bars (time-in-range)** | **No-op** on this core (HO identical to baseline). Breakouts already follow N-bar containment. |
| **Time-stop 48h** | **Best HO Calmar** (+176% / DD −30% / 5.80) but **IS much worse** (+499% vs +947%). Treat as fragile — may be period luck. |
| **Loss-streak pause ls5_24** | **Most credible.** HO +150% / DD −30% / Calmar 5.08; IS also better (+1110% / 42.8). Mild streak cooldown without equity-DD freeze. |

**Recommendation:** keep primary core unchanged. Candidate alternate: **`--loss-streak-n 5 --loss-streak-pause-bars 24`**. Do not promote ADX/ATR%/channel filters. Time-stop 48 interesting but IS/HO tension → not freeze yet.

## Holdout 2024–2026

| variant | return | maxDD | n | WR | PF | Calmar |
|---------|-------:|------:|--:|---:|---:|-------:|
| tstop48 | 176.09% | -30.37% | 510 | 28.2% | 1.23 | 5.80 |
| **ls5_24** | **149.93%** | **-29.54%** | 454 | 27.5% | 1.24 | **5.08** |
| baseline | 141.56% | -33.64% | 474 | 27.2% | 1.22 | 4.21 |
| inside12 / inside24 / tstop168 | 141.56% | -33.64% | 474 | 27.2% | 1.22 | 4.21 |
| tstop96 | 140.51% | -33.48% | 478 | 27.4% | 1.22 | 4.20 |
| ls3_24 | 136.45% | -33.03% | 436 | 28.4% | 1.26 | 4.13 |
| ls5_48 | 122.58% | -29.68% | 436 | 26.8% | 1.23 | 4.13 |
| chan3 | 125.01% | -32.92% | 469 | 27.3% | 1.20 | 3.80 |
| adx20 | 90.49% | -25.86% | 369 | 26.8% | 1.23 | 3.50 |
| ls3_48 | 64.50% | -43.01% | 394 | 26.1% | 1.16 | 1.50 |
| chan5 | 30.26% | -25.10% | 318 | 25.8% | 1.11 | 1.21 |
| atrpct0.4 | 23.86% | -33.74% | 429 | 26.8% | 1.06 | 0.71 |
| adx25 | 5.48% | -28.60% | 292 | 26.0% | 1.03 | 0.19 |
| atrpct0.6 | -8.40% | -33.35% | 302 | 26.5% | 0.97 | -0.25 |

## In-sample 2020–2023 (sanity)

| variant | return | maxDD | n | PF | Calmar |
|---------|-------:|------:|--:|---:|-------:|
| ls5_24 | 1109.56% | -25.94% | 659 | 1.53 | 42.77 |
| ls5_48 | 1026.47% | -25.78% | 636 | 1.54 | 39.81 |
| tstop168 | 1034.20% | -27.62% | 692 | 1.44 | 37.44 |
| inside24 | 961.64% | -27.89% | 685 | 1.44 | 34.48 |
| baseline | 947.00% | -27.89% | 686 | 1.43 | 33.96 |
| tstop48 | 498.92% | -30.91% | 755 | 1.30 | 16.14 |
| adx20 | 387.09% | -31.57% | 556 | 1.35 | 12.26 |
| atrpct0.6 | 174.11% | -26.46% | 545 | 1.22 | 6.58 |

## Why

- Regime filters remove trades in weak-trend / low-vol — but that is also where some runners start, and 2024–26 regime differs from 2020–23.
- Equity-DD pause was worse; **fixed bar cooldown after 5 losses** only skips a short chop cluster, then rejoins.
- Aggressive time-stops clip long runners on IS; a soft 48h stop helped this holdout but failed the IS check.
