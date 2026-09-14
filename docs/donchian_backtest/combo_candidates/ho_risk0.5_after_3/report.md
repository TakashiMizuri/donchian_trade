# Donchian breakout — BTCUSDT

- Window: `2024-01-01 00:00` → `2027-01-01 00:00` UTC
- TF: **1h** | Donchian **N=30** entry / **M=15** exit
- Stop: **1.5 × ATR(20)** at signal bar
- Adaptive risk: **base 1%** → after win **1%**, after loss **0.5%**, cap **$1,000.00**
- Max positions: **1**
- Invert-after-loss: **off**
- Min breakout: **off**
- Equity DD pause: **off**
- Min ADX: **off**
- Min ATR%: **off**
- Min channel width: **off**
- Min inside bars: **off**
- Max bars in trade: **off**
- Loss-streak pause: after **3** losses skip **1** bars
- Cooldown resume breakout: **off**
- Cooldown resume ADX: **off**
- Cooldown resume ATR%: **off**
- Fees: **Binance USD-M Futures BTCUSDT, VIP0 taker (no BNB discount)**, 0.0500% per side
- Entry: next open after N-channel break; exit: ATR stop or M-channel (next open)

## Results

| Metric | Value |
|---|---|
| Final balance | $19,098.11 |
| Net PnL | $9,098.11 |
| Return | 90.98% |
| Max drawdown | -22.81% |
| Trades | 516 |
| Closed | 474 (SL 288 / channel 186) |
| Open | 0 |
| Net-positive after fees | 27.22% |
| Profit factor | 1.28 |
| Sum of realized R | 169.4 |
| Avg realized R | +0.357 |
| Max concurrent | 1 |
| Commission total | $5,595.09 |

## Monthly

| Month | n | net PnL | sum R |
|---|---:|---:|---:|
| 2024-01 | 15 | $1,001.50 | +14.7 |
| 2024-02 | 13 | $1,022.97 | +23.3 |
| 2024-03 | 15 | $-510.32 | -5.0 |
| 2024-04 | 16 | $-12.72 | +4.0 |
| 2024-05 | 16 | $-301.43 | -2.6 |
| 2024-06 | 16 | $760.16 | +16.8 |
| 2024-07 | 14 | $1,973.30 | +27.0 |
| 2024-08 | 13 | $482.37 | +2.8 |
| 2024-09 | 18 | $415.71 | +1.3 |
| 2024-10 | 18 | $534.20 | +12.7 |
| 2024-11 | 16 | $-318.40 | +2.0 |
| 2024-12 | 22 | $-1,058.92 | -5.9 |
| 2025-01 | 15 | $828.50 | +8.2 |
| 2025-02 | 11 | $154.44 | +6.5 |
| 2025-03 | 16 | $276.67 | +9.8 |
| 2025-04 | 16 | $-544.41 | -3.5 |
| 2025-05 | 15 | $988.49 | +7.5 |
| 2025-06 | 19 | $-626.02 | -3.5 |
| 2025-07 | 20 | $-852.12 | -5.9 |
| 2025-08 | 22 | $-650.28 | -1.5 |
| 2025-09 | 19 | $718.82 | +7.5 |
| 2025-10 | 14 | $2,827.79 | +20.4 |
| 2025-11 | 17 | $149.52 | +8.1 |
| 2025-12 | 19 | $-1,397.72 | -11.0 |
| 2026-01 | 13 | $4,967.53 | +37.6 |
| 2026-02 | 15 | $-1,055.33 | -6.4 |
| 2026-03 | 16 | $732.68 | +6.6 |
| 2026-04 | 20 | $-1,682.94 | -10.0 |
| 2026-05 | 14 | $-87.42 | +4.5 |
| 2026-06 | 1 | $621.36 | +3.5 |
