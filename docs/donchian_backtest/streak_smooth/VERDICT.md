# Streak-smooth sizing (not DD-pause)

Base: **1h N=30 M=15 ATR×1.5** · `$10k` · risk `1%` · cap `$1000` · fee `0.05%`
Data: **native 1h** archives. IS pick: **BTC 2020–2023**. Validate: **BTC 2024–2026**, then **ETH blind**.
Script: `scripts/run_donchian_streak_smooth.py`

## Variants

| name | lever |
|------|-------|
| `baseline` | frozen core signals + flat 1% risk |
| `ls5_cond_brk2.0` | pause after 5 losses ≤24 bars; resume if breakout ≥2.0×ATR |
| `soft_adaptive` | adaptive risk: after win = base 1%; after loss = 0.5% (no size-up) |
| `streak_risk_3_5` | after 3 consec losses ×0.5 risk; after 5 ×0.25; reset on win |

## Verdict

IS prefers `ls5_cond_brk2.0` over streak sizing. Keep existing alternate B; **do not promote** streak_risk.

ETH blind HO context — baseline 235.8%/-15.3%; ls5_cond 231.7%/-18.1%; streak_risk 57.1%/-16.5%.

### IS pick notes

- `ls5_cond_brk2.0`: hits=3/3 (dd=True, dd≥20%=False, streak=True, calmar=True), ret_floor60%=True → PASS
- `soft_adaptive`: hits=1/3 (dd=True, dd≥20%=True, streak=False, calmar=False), ret_floor60%=False → fail
- `streak_risk_3_5`: hits=1/3 (dd=True, dd≥20%=False, streak=False, calmar=False), ret_floor60%=False → fail

**IS pick:** `ls5_cond_brk2.0`
**Promote alternate C:** `no`

## BTC in-sample 2020–2023

| variant | return | maxDD | n | maxLstreak | PF | Calmar |
|---------|-------:|------:|--:|-----------:|---:|-------:|
| baseline | 934.13% | -27.89% | 687 | 25 | 1.43 | 33.50 |
| ls5_cond_brk2.0 | 1127.97% | -25.94% | 661 | 22 | 1.53 | 43.48 |
| soft_adaptive | 547.36% | -19.39% | 687 | 25 | 1.57 | 28.22 |
| streak_risk_3_5 | 441.58% | -26.59% | 687 | 25 | 1.44 | 16.61 |

## BTC holdout 2024–2026

| variant | return | maxDD | n | maxLstreak | PF | Calmar |
|---------|-------:|------:|--:|-----------:|---:|-------:|
| baseline | 210.66% | -33.64% | 536 | 12 | 1.27 | 6.26 |
| ls5_cond_brk2.0 | 180.76% | -29.95% | 518 | 15 | 1.23 | 6.04 |
| soft_adaptive | 126.13% | -22.81% | 536 | 12 | 1.32 | 5.53 |
| streak_risk_3_5 | 27.30% | -34.19% | 536 | 12 | 1.08 | 0.80 |

## ETH blind in-sample 2020–2023

| variant | return | maxDD | n | maxLstreak | PF | Calmar |
|---------|-------:|------:|--:|-----------:|---:|-------:|
| baseline | 728.13% | -30.16% | 712 | 20 | 1.25 | 24.14 |
| ls5_cond_brk2.0 | 792.66% | -25.78% | 690 | 20 | 1.28 | 30.74 |
| soft_adaptive | 273.77% | -17.54% | 712 | 20 | 1.28 | 15.61 |
| streak_risk_3_5 | 242.30% | -24.67% | 712 | 20 | 1.20 | 9.82 |

## ETH blind holdout 2024–2026

| variant | return | maxDD | n | maxLstreak | PF | Calmar |
|---------|-------:|------:|--:|-----------:|---:|-------:|
| baseline | 235.82% | -15.29% | 514 | 14 | 1.38 | 15.42 |
| ls5_cond_brk2.0 | 231.75% | -18.14% | 500 | 13 | 1.38 | 12.78 |
| soft_adaptive | 127.28% | -10.99% | 514 | 14 | 1.35 | 11.58 |
| streak_risk_3_5 | 57.07% | -16.45% | 514 | 14 | 1.14 | 3.47 |

## Hypothesis check

Streak-scaled risk keeps the same entries; only `$ risk` shrinks during loss runs, then returns to full size after a win. Compare max DD, max loss streak, Calmar, and whether runners still carry return.

CLI:
```bash
py -3 scripts/run_donchian_backtest.py --from 2024-01-01 --to 2027-01-01 \
  --bar-seconds 3600 --donchian-n 30 --donchian-m 15 --atr-stop-mult 1.5 \
  --risk-streak-soft-n 3 --risk-streak-soft-mult 0.5 \
  --risk-streak-hard-n 5 --risk-streak-hard-mult 0.25 \
  --balance 10000 --risk-pct 1
```

