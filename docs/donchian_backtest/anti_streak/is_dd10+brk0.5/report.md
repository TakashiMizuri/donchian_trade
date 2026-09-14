# Donchian breakout — BTCUSDT

- Window: `2020-01-01 00:00` → `2024-01-01 00:00` UTC
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
| Final balance | $15,794.34 |
| Net PnL | $5,794.34 |
| Return | 57.94% |
| Max drawdown | -10.01% |
| Trades | 526 |
| Closed | 61 (SL 33 / channel 28) |
| Open | 0 |
| Net-positive after fees | 34.43% |
| Profit factor | 2.04 |
| Sum of realized R | 55.8 |
| Avg realized R | +0.914 |
| Max concurrent | 1 |
| Commission total | $777.78 |

## Monthly

| Month | n | net PnL | sum R |
|---|---:|---:|---:|
| 2020-01 | 11 | $1,242.28 | +13.5 |
| 2020-02 | 8 | $668.62 | +7.0 |
| 2020-03 | 7 | $3,069.86 | +24.5 |
| 2020-04 | 10 | $2,370.62 | +17.2 |
| 2020-05 | 12 | $-417.51 | -1.5 |
| 2020-06 | 8 | $-397.06 | -1.5 |
| 2020-07 | 5 | $-686.22 | -3.5 |
