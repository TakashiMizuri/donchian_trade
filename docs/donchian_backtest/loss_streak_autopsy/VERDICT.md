# Loss-streak autopsy (baseline core)

Core: **1h N=30 M=15 ATR×1.5** · `$10k` · risk `1%` · fee `0.05%`  
Streak definition: consecutive net losses of length **≥ 5**.  
Design: **BTC 2020–2023**. Sanity: BTC HO + ETH in CSVs.  
Script: `scripts/run_donchian_loss_streak_autopsy.py`

## Verdict

**Next hypothesis to test: `none` — do not invent a new entry filter from this autopsy.**

Loss streaks on this core **do not sit in a special market regime at entry**. ATR%, channel width/ATR, breakout strength, and ADX for streak trades look like winners (and like ordinary losses). Streaks are mostly **mixed long/short chop**, not one-sided fatigue.

Keep **`ls5_cond_brk2.0`** as the anti-streak alternate (pause after the streak happens). Do not promote same-side fatigue, flip-flop skip, or a new ADX/ATR% gate from these numbers.

### What we looked for

| Idea | Result |
|------|--------|
| Regime at entry (ATR%, ADX, channel/ATR, breakout×ATR) | **Weak** — streak ≈ winners ≈ all losses |
| Same-side fatigue (skip next same-side after k losses) | **Reject** — only **5/43** streak runs are ≥70% one direction; after 2–3 same-side losses next same-side R is not reliably worse |
| Quick opposite flip after a loss | **Reject** — IS suggests soft 24h flips; **HO flips the sign** (quick flips look fine/better) → classic overfit trap |
| Post-loss “probation” min breakout | Mild on IS, **weak transfer** on HO; always-on `min_breakout_atr` already known as smoother alternate |
| Recent trade density (exits in 24–168h) | **No separation** streak vs winners |
| Hold time | **CLEAR** but tautological (SL exits fast; runners hold) — not an entry feature |

### Why this is useful anyway

Confirms the mental model: low WR + fat right tail → long strings of small −R are **the cost of the edge**, not a detectable “bad regime” you can filter without also cutting runners. Pausing **after** N losses (`ls5_cond`) remains the cleaner lever than predicting the streak from the signal bar.

## BTC IS 2020–2023 — bucket medians

| bucket | n | %BUY | %SL | med atr% | med ch/ATR | med brk×ATR | med ADX | med hold h | med R |
|--------|--:|-----:|----:|---------:|-----------:|------------:|--------:|-----------:|------:|
| all | 687 | 52% | 64% | 0.81% | 4.33 | 0.51 | 21.7 | 14.0 | -1.00 |
| winners | 178 | 58% | 0% | 0.81% | 4.32 | 0.54 | 22.0 | 50.5 | 2.31 |
| runners_p75 | 45 | 62% | 0% | 0.67% | 4.13 | 0.65 | 20.0 | 87.0 | 9.49 |
| losers | 509 | 50% | 86% | 0.81% | 4.34 | 0.49 | 21.7 | 7.0 | -1.00 |
| streak≥5 | 311 | 51% | 88% | 0.78% | 4.41 | 0.52 | 22.1 | 8.0 | -1.00 |

## Feature separation (streak vs winners / vs all losses)

- atr_pct: weak (−4% vs winners)
- channel_atr: weak (+2%)
- breakout_atr: weak (−3% vs winners; actually *higher* than generic losses)
- adx: weak
- bars_held: CLEAR (−84% vs winners) — outcome, not filter

## Same-side fatigue (next same-side after k same-side losses)

### BTC IS

- after any loss → next: n=508, meanR=0.37, WR=28%
- k=2 same-side → next same-side: n=35, meanR=0.96, WR=31% (not worse)
- k=3: n=13, meanR=0.11 — small n
- k=4/5: tiny n / empty

Streak runs with ≥70% one direction: **5/43**

### BTC HO

- k=3 looks bad (meanR=−0.69, n=11) but k=2 is fine and HO opposite-flips disagree with IS — not freeze material.

## Files

- `output/donchian_backtest/loss_streak_autopsy/trades_*.csv`
- `output/donchian_backtest/loss_streak_autopsy/VERDICT.md`
