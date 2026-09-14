# Donchian breakout — BTCUSDT

- Window: `2020-01-01 00:00` → `2024-01-01 00:00` UTC
- TF: **1h** | Donchian **N=30** entry / **M=15** exit
- Stop: **1.5 × ATR(20)** at signal bar
- Risk: **1%** equity, cap **$1,000.00**
- Max positions: **1**
- Invert-after-loss: **off**
- Min breakout: **off**
- Equity DD pause: **≥15%** → pause; resume ≤8%
- Fees: **Binance USD-M Futures BTCUSDT, VIP0 taker (no BNB discount)**, 0.0500% per side
- Entry: next open after N-channel break; exit: ATR stop or M-channel (next open)

## Results

| Metric | Value |
|---|---|
| Final balance | $13,863.22 |
| Net PnL | $3,863.22 |
| Return | 38.63% |
| Max drawdown | -15.94% |
| Trades | 753 |
| Closed | 83 (SL 52 / channel 31) |
| Open | 0 |
| Net-positive after fees | 27.71% |
| Profit factor | 1.48 |
| Sum of realized R | 45.3 |
| Avg realized R | +0.546 |
| Max concurrent | 1 |
| Commission total | $986.52 |

## Monthly

| Month | n | net PnL | sum R |
|---|---:|---:|---:|
| 2020-01 | 15 | $1,223.05 | +14.0 |
| 2020-02 | 11 | $392.63 | +4.9 |
| 2020-03 | 10 | $2,725.72 | +22.8 |
| 2020-04 | 13 | $2,200.10 | +17.1 |
| 2020-05 | 16 | $-1,072.94 | -5.5 |
| 2020-06 | 10 | $-393.82 | -1.4 |
| 2020-07 | 8 | $-1,122.97 | -6.5 |
