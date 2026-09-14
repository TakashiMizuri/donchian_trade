# Donchian breakout — BTCUSDT

- Window: `2024-01-01 00:00` → `2027-01-01 00:00` UTC
- TF: **1h** | Donchian **N=30** entry / **M=15** exit
- Stop: **1.5 × ATR(20)** at signal bar
- Risk: **1%** equity, cap **$1,000.00**
- Max positions: **1**
- Invert-after-loss: **off**
- Min breakout: **0.5 × ATR** beyond channel
- Equity DD pause: **≥10%** → pause; resume ≤5%
- Fees: **Binance USD-M Futures BTCUSDT, VIP0 taker (no BNB discount)**, 0.0500% per side
- Entry: next open after N-channel break; exit: ATR stop or M-channel (next open)

## Results

| Metric | Value |
|---|---|
| Final balance | $15,586.71 |
| Net PnL | $5,586.71 |
| Return | 55.87% |
| Max drawdown | -10.09% |
| Trades | 384 |
| Closed | 109 (SL 64 / channel 45) |
| Open | 0 |
| Net-positive after fees | 33.94% |
| Profit factor | 1.55 |
| Sum of realized R | 62.4 |
| Avg realized R | +0.572 |
| Max concurrent | 1 |
| Commission total | $1,595.43 |

## Monthly

| Month | n | net PnL | sum R |
|---|---:|---:|---:|
| 2024-01 | 11 | $1,508.59 | +15.7 |
| 2024-02 | 11 | $2,225.62 | +21.5 |
| 2024-03 | 12 | $-789.95 | -4.9 |
| 2024-04 | 14 | $-15.69 | +1.5 |
| 2024-05 | 13 | $-42.17 | +1.2 |
| 2024-06 | 11 | $1,334.33 | +12.1 |
| 2024-07 | 9 | $2,176.30 | +16.2 |
| 2024-08 | 11 | $615.57 | +4.9 |
| 2024-09 | 14 | $-764.75 | -3.0 |
| 2024-10 | 3 | $-500.91 | -2.9 |
