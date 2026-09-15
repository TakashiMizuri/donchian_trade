package engine

import "math"

// ResidualCash is the unexplained wallet move between two account snapshots.
// wallet is settled value (equity − uPnL), not mark-to-market.
// Trading (closed nets + open-entry fees already in settled equity) is subtracted
// so only deposits / withdrawals remain.
func ResidualCash(prevWallet, wallet, prevRealized, realized, prevOpenFees, openFees float64) float64 {
	// +ΔopenFees: entry fee is reserved in the tracker at open and already inside realized at close.
	return (wallet - prevWallet) - (realized - prevRealized) + (openFees - prevOpenFees)
}

// SettledWallet is book value without mark: fees and realized hits show up here,
// uPnL does not.
func SettledWallet(equity, upnl float64) float64 {
	return equity - upnl
}

func cashThreshold(minUSD, equity float64) float64 {
	if minUSD <= 0 {
		minUSD = 5
	}
	floor := 0.0005 * math.Abs(equity) // 5 bps of equity — ignore mark dust
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
