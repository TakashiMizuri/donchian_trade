# Invert-after-loss

Base: **1h N=30 M=15 ATR×1.5**
After gross loss → flip next breakout; after win → normal signal.
IS: **2020–2023**. Holdout: **2024–2026**.

## In-sample

| variant | return | maxDD | n | WR | PF | Calmar |
|---------|-------:|------:|--:|---:|---:|-------:|
| baseline | 947.00% | -27.89% | 753 | 25.9% | 1.43 | 33.96 |
| ial+adaptive | 339.34% | -28.51% | 895 | 27.3% | 1.34 | 11.90 |
| invert_after_loss | 242.81% | -26.83% | 895 | 27.3% | 1.25 | 9.05 |

## Holdout 2024–2026

| variant | return | maxDD | n | WR | PF | Calmar |
|---------|-------:|------:|--:|---:|---:|-------:|
| ial+adaptive | 195.02% | -29.05% | 609 | 25.4% | 1.24 | 6.71 |
| baseline | 141.56% | -33.64% | 516 | 27.2% | 1.22 | 4.21 |
| invert_after_loss | 64.32% | -31.03% | 609 | 25.4% | 1.13 | 2.07 |
