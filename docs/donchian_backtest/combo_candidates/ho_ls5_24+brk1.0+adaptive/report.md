# Donchian breakout — BTCUSDT

- Window: `2024-01-01 00:00` → `2027-01-01 00:00` UTC
- TF: **1h** | Donchian **N=30** entry / **M=15** exit
- Stop: **1.5 × ATR(20)** at signal bar
- Adaptive risk: **base 1%** → after win **2%**, after loss **0.5%**, cap **$1,000.00**
- Max positions: **1**
- Invert-after-loss: **off**
- Min breakout: **1 × ATR** beyond channel
- Equity DD pause: **off**
- Min ADX: **off**
- Min ATR%: **off**
- Min channel width: **off**
- Min inside bars: **off**
- Max bars in trade: **off**
- Loss-streak pause: after **5** losses skip **24** bars
- Cooldown resume breakout: **off**
- Cooldown resume ADX: **off**
- Cooldown resume ATR%: **off**
- Fees: **Binance USD-M Futures BTCUSDT, VIP0 taker (no BNB discount)**, 0.0500% per side
- Entry: next open after N-channel break; exit: ATR stop or M-channel (next open)

## Results

| Metric | Value |
|---|---|
| Final balance | $18,630.72 |
| Net PnL | $8,630.72 |
| Return | 86.31% |
| Max drawdown | -17.03% |
| Trades | 240 |
| Closed | 206 (SL 118 / channel 88) |
| Open | 0 |
| Net-positive after fees | 31.55% |
| Profit factor | 1.43 |
| Sum of realized R | 90.2 |
| Avg realized R | +0.438 |
| Max concurrent | 1 |
| Commission total | $3,498.45 |

## Monthly

| Month | n | net PnL | sum R |
|---|---:|---:|---:|
| 2024-01 | 7 | $1,848.35 | +12.1 |
| 2024-02 | 8 | $927.34 | +24.5 |
| 2024-03 | 5 | $-506.06 | -4.5 |
| 2024-04 | 7 | $734.21 | +6.0 |
| 2024-05 | 9 | $-339.04 | +4.0 |
| 2024-06 | 7 | $-205.96 | +1.1 |
| 2024-07 | 9 | $2,268.82 | +14.3 |
| 2024-08 | 8 | $-281.93 | -4.5 |
| 2024-09 | 6 | $-209.65 | +1.4 |
| 2024-10 | 9 | $-594.49 | +0.0 |
| 2024-11 | 5 | $189.26 | +5.7 |
| 2024-12 | 8 | $-909.49 | -6.3 |
| 2025-01 | 7 | $532.20 | +3.6 |
| 2025-02 | 7 | $-346.80 | -0.9 |
| 2025-03 | 8 | $186.89 | +4.5 |
| 2025-04 | 6 | $-427.33 | -2.5 |
| 2025-05 | 4 | $1,687.55 | +5.9 |
| 2025-06 | 6 | $-296.75 | -3.4 |
| 2025-07 | 6 | $-276.34 | +3.8 |
| 2025-08 | 11 | $-746.76 | -9.4 |
| 2025-09 | 4 | $140.52 | +6.4 |
| 2025-10 | 8 | $1,377.57 | +9.3 |
| 2025-11 | 5 | $1,337.44 | +4.3 |
| 2025-12 | 10 | $-1,058.59 | -5.8 |
| 2026-01 | 10 | $5,610.97 | +21.5 |
| 2026-02 | 2 | $328.93 | +1.1 |
| 2026-03 | 9 | $-730.57 | -1.3 |
| 2026-04 | 7 | $-756.71 | -0.3 |
| 2026-05 | 8 | $-510.33 | -0.1 |
