# Donchian breakout — BTCUSDT

- Window: `2024-01-01 00:00` → `2027-01-01 00:00` UTC
- TF: **1h** | Donchian **N=30** entry / **M=15** exit
- Stop: **1.5 × ATR(20)** at signal bar
- Risk: **1%** equity, cap **$1,000.00**
- Max positions: **1**
- Invert-after-loss: **off**
- Min breakout: **off**
- Equity DD pause: **≥10%** → pause; resume ≤5%
- Fees: **Binance USD-M Futures BTCUSDT, VIP0 taker (no BNB discount)**, 0.0500% per side
- Entry: next open after N-channel break; exit: ATR stop or M-channel (next open)

## Results

| Metric | Value |
|---|---|
| Final balance | $12,198.77 |
| Net PnL | $2,198.77 |
| Return | 21.99% |
| Max drawdown | -10.70% |
| Trades | 516 |
| Closed | 85 (SL 50 / channel 35) |
| Open | 0 |
| Net-positive after fees | 29.41% |
| Profit factor | 1.30 |
| Sum of realized R | 33.6 |
| Avg realized R | +0.396 |
| Max concurrent | 1 |
| Commission total | $1,147.12 |

## Monthly

| Month | n | net PnL | sum R |
|---|---:|---:|---:|
| 2024-01 | 15 | $1,337.72 | +14.7 |
| 2024-02 | 13 | $2,360.85 | +23.3 |
| 2024-03 | 15 | $-853.39 | -5.0 |
| 2024-04 | 16 | $302.20 | +4.0 |
| 2024-05 | 16 | $-575.08 | -2.6 |
| 2024-06 | 10 | $-292.70 | -0.8 |
