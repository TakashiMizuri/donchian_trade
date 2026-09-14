package engine

import "math"

// ResidualCash is the unexplained wallet move between two account snapshots.
// Trading (closed nets + estimated open-entry fees) is subtracted so only
// deposits / withdrawals remain. Adverse mark moves live in uPnL, not wallet.
func ResidualCash(prevWallet, wallet, prevRealized, realized, prevOpenFees, openFees float64) float64 {
	// +ΔopenFees: entry fee is reserved in the tracker at open and already inside realized at close.
	return (wallet - prevWallet) - (realized - prevRealized) + (openFees - prevOpenFees)
}

func cashThreshold(minUSD, equity float64) float64 {
	if minUSD <= 0 {
		minUSD = 5
	}
	floor := 0.0005 * math.Abs(equity) // 5 bps — swallow mark/fee dust
	if floor > minUSD {
		return floor
	}
	return minUSD
}

func classifyCash(amount float64) string {
	if amount < 0 {
		return "withdraw"
	}
	return "deposit"
}
