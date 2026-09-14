# Donchian breakout — BTCUSDT

- Window: `2020-01-01 00:00` → `2024-01-01 00:00` UTC
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
| Final balance | $14,745.61 |
| Net PnL | $4,745.61 |
| Return | 47.46% |
| Max drawdown | -10.59% |
| Trades | 753 |
| Closed | 69 (SL 44 / channel 25) |
| Open | 0 |
| Net-positive after fees | 28.99% |
| Profit factor | 1.72 |
| Sum of realized R | 49.3 |
| Avg realized R | +0.714 |
| Max concurrent | 1 |
| Commission total | $709.07 |

## Monthly

| Month | n | net PnL | sum R |
|---|---:|---:|---:|
| 2020-01 | 15 | $1,223.05 | +14.0 |
| 2020-02 | 11 | $392.63 | +4.9 |
| 2020-03 | 10 | $2,725.72 | +22.8 |
| 2020-04 | 13 | $2,200.10 | +17.1 |
| 2020-05 | 16 | $-1,072.94 | -5.5 |
| 2020-06 | 4 | $-669.04 | -4.0 |
