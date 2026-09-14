# Donchian breakout — BTCUSDT

- Window: `2024-01-01 00:00` → `2027-01-01 00:00` UTC
- TF: **1h** | Donchian **N=30** entry / **M=15** exit
- Stop: **1.5 × ATR(20)** at signal bar
- Adaptive risk: **base 1%** → after win **2%**, after loss **0.5%**, cap **$1,000.00**
- Max positions: **1**
- Invert-after-loss: **off**
- Min breakout: **off**
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
| Final balance | $24,251.40 |
| Net PnL | $14,251.40 |
| Return | 142.51% |
| Max drawdown | -29.78% |
| Trades | 492 |
| Closed | 454 (SL 275 / channel 179) |
| Open | 0 |
| Net-positive after fees | 27.53% |
| Profit factor | 1.31 |
| Sum of realized R | 169.2 |
| Avg realized R | +0.373 |
| Max concurrent | 1 |
| Commission total | $8,000.27 |

## Monthly

| Month | n | net PnL | sum R |
|---|---:|---:|---:|
| 2024-01 | 15 | $1,094.36 | +14.7 |
| 2024-02 | 12 | $375.75 | +15.9 |
| 2024-03 | 15 | $-766.36 | -5.4 |
| 2024-04 | 16 | $-299.50 | +4.0 |
| 2024-05 | 16 | $-369.87 | -2.6 |
| 2024-06 | 15 | $696.82 | +17.8 |
| 2024-07 | 13 | $2,517.11 | +28.0 |
| 2024-08 | 12 | $1,218.22 | +3.8 |
| 2024-09 | 18 | $846.73 | -0.1 |
| 2024-10 | 16 | $353.09 | +14.7 |
| 2024-11 | 15 | $-925.41 | +3.0 |
| 2024-12 | 21 | $-1,708.67 | -6.2 |
| 2025-01 | 15 | $1,479.32 | +8.2 |
| 2025-02 | 10 | $-125.33 | +7.5 |
| 2025-03 | 16 | $-337.09 | +9.8 |
| 2025-04 | 16 | $-718.08 | -3.5 |
| 2025-05 | 14 | $2,032.27 | +8.5 |
| 2025-06 | 17 | $-651.74 | -1.5 |
| 2025-07 | 19 | $-1,119.77 | -4.9 |
| 2025-08 | 21 | $-1,094.92 | -1.0 |
| 2025-09 | 18 | $1,342.60 | +5.5 |
| 2025-10 | 14 | $5,839.72 | +20.4 |
| 2025-11 | 17 | $-670.35 | +8.1 |
| 2025-12 | 17 | $-1,762.67 | -10.7 |
| 2026-01 | 13 | $10,156.53 | +37.6 |
| 2026-02 | 15 | $-2,065.86 | -6.4 |
| 2026-03 | 16 | $1,434.21 | +5.8 |
| 2026-04 | 19 | $-3,148.93 | -11.8 |
| 2026-05 | 12 | $-558.96 | +6.5 |
| 2026-06 | 1 | $1,528.33 | +3.5 |
