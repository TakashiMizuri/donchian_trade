# Donchian breakout — BTCUSDT

- Window: `2024-01-01 00:00` → `2027-01-01 00:00` UTC
- TF: **1h** | Donchian **N=30** entry / **M=15** exit
- Stop: **1.5 × ATR(20)** at signal bar
- Adaptive risk: **base 1%** → after win **2%**, after loss **0.5%**, cap **$1,000.00**
- Max positions: **1**
- Invert-after-loss: **ON**
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
| Final balance | $25,279.95 |
| Net PnL | $15,279.95 |
| Return | 152.80% |
| Max drawdown | -29.98% |
| Trades | 580 |
| Closed | 511 (SL 195 / channel 316) |
| Open | 0 |
| Net-positive after fees | 24.85% |
| Profit factor | 1.26 |
| Sum of realized R | 110.5 |
| Avg realized R | +0.216 |
| Max concurrent | 1 |
| Commission total | $11,950.71 |

## Monthly

| Month | n | net PnL | sum R |
|---|---:|---:|---:|
| 2024-01 | 15 | $1,652.39 | +22.3 |
| 2024-02 | 14 | $1,262.46 | +5.1 |
| 2024-03 | 17 | $-652.91 | -3.2 |
| 2024-04 | 14 | $-415.23 | -2.4 |
| 2024-05 | 14 | $158.64 | +9.6 |
| 2024-06 | 18 | $2,807.48 | +14.1 |
| 2024-07 | 16 | $1,813.55 | +11.9 |
| 2024-08 | 16 | $3,635.99 | +13.6 |
| 2024-09 | 17 | $3,046.35 | +9.9 |
| 2024-10 | 25 | $-2,518.09 | -16.1 |
| 2024-11 | 16 | $802.52 | +9.5 |
| 2024-12 | 23 | $-275.68 | -3.1 |
| 2025-01 | 21 | $1,659.87 | +1.9 |
| 2025-02 | 12 | $-1,662.13 | +1.6 |
| 2025-03 | 17 | $3,482.13 | +14.8 |
| 2025-04 | 17 | $-1,226.42 | -2.6 |
| 2025-05 | 15 | $2,905.35 | +5.6 |
| 2025-06 | 24 | $-2,717.11 | -10.2 |
| 2025-07 | 18 | $-1,557.84 | -5.5 |
| 2025-08 | 20 | $-439.16 | +1.5 |
| 2025-09 | 16 | $-972.78 | +8.4 |
| 2025-10 | 20 | $3,596.47 | +5.7 |
| 2025-11 | 21 | $1,936.52 | +1.7 |
| 2025-12 | 17 | $-1,217.61 | -3.3 |
| 2026-01 | 18 | $1,591.28 | +4.2 |
| 2026-02 | 18 | $-565.29 | +2.4 |
| 2026-03 | 16 | $2,120.57 | +7.2 |
| 2026-04 | 18 | $-833.74 | +6.1 |
| 2026-05 | 15 | $-1,383.24 | +0.9 |
| 2026-06 | 3 | $-180.50 | -1.0 |
