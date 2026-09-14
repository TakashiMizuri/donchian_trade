# Donchian breakout — BTCUSDT

- Window: `2024-01-01 00:00` → `2027-01-01 00:00` UTC
- TF: **1h** | Donchian **N=30** entry / **M=15** exit
- Stop: **1.5 × ATR(20)** at signal bar
- Adaptive risk: **base 1%** → after win **2%**, after loss **0.5%**, cap **$1,000.00**
- Max positions: **1**
- Invert-after-loss: **ON**
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
| Final balance | $19,822.95 |
| Net PnL | $9,822.95 |
| Return | 98.23% |
| Max drawdown | -13.29% |
| Trades | 260 |
| Closed | 203 (SL 79 / channel 124) |
| Open | 0 |
| Net-positive after fees | 28.57% |
| Profit factor | 1.53 |
| Sum of realized R | 112.7 |
| Avg realized R | +0.555 |
| Max concurrent | 1 |
| Commission total | $3,930.19 |

## Monthly

| Month | n | net PnL | sum R |
|---|---:|---:|---:|
| 2024-01 | 8 | $496.30 | +5.3 |
| 2024-02 | 10 | $1,381.15 | +32.4 |
| 2024-03 | 5 | $-265.40 | +0.9 |
| 2024-04 | 7 | $-129.83 | -0.8 |
| 2024-05 | 7 | $258.63 | +13.7 |
| 2024-06 | 10 | $612.95 | +2.7 |
| 2024-07 | 8 | $4,479.77 | +20.4 |
| 2024-08 | 7 | $-33.59 | +3.2 |
| 2024-09 | 5 | $782.69 | +7.9 |
| 2024-10 | 7 | $476.57 | +12.3 |
| 2024-11 | 4 | $-353.51 | -3.6 |
| 2024-12 | 8 | $-8.20 | +3.0 |
| 2025-01 | 6 | $751.62 | +1.9 |
| 2025-02 | 8 | $490.50 | -0.2 |
| 2025-03 | 9 | $-635.89 | +0.9 |
| 2025-04 | 7 | $3,313.66 | +24.8 |
| 2025-05 | 4 | $-733.41 | -3.8 |
| 2025-06 | 6 | $-195.08 | +2.5 |
| 2025-07 | 5 | $-706.65 | -4.3 |
| 2025-08 | 8 | $-430.55 | -3.2 |
| 2025-09 | 3 | $251.41 | +3.2 |
| 2025-10 | 10 | $2,349.36 | +5.3 |
| 2025-11 | 7 | $-532.36 | -4.4 |
| 2025-12 | 8 | $-715.41 | +1.2 |
| 2026-01 | 9 | $1,832.65 | +4.0 |
| 2026-02 | 5 | $-135.66 | -0.9 |
| 2026-03 | 9 | $-1,142.20 | -6.7 |
| 2026-04 | 6 | $-556.48 | -4.8 |
| 2026-05 | 7 | $-384.60 | +0.0 |
