package engine

import "math"

// ResidualCash is the unexplained wallet move between two account snapshots.
// wallet is settled value (equity − uPnL), not mark-to-market.
//
// Trading subtracted from Δwallet:
//   - realized excluding funding (price PnL and fees land in settled equity at close)
//   - all journaled funding, including open trades (Lighter posts funding into
//     collateral as it accrues; we also add it to net at close — without this
//     term the close looks like a withdrawal of the same dollars)
//   - open-entry fees already reserved in the tracker
//
// Leftover is deposits / withdrawals.
func ResidualCash(prevWallet, wallet, prevRealizedXF, realizedXF, prevOpenFees, openFees, prevFunding, funding float64) float64 {
	return (wallet - prevWallet) - (realizedXF - prevRealizedXF) - (funding - prevFunding) + (openFees - prevOpenFees)
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

// closeDustCash is a leftover at trade close (funding already in the wallet, or
// mark vs fill), not a user transfer. explained is the trade PnL in the same tick.
func closeDustCash(amount, explained, maxAbs float64) bool {
	a := math.Abs(amount)
	if a < 1e-9 || a >= maxAbs {
		return false
	}
	// Tiny residual next to a much larger trade PnL.
	if math.Abs(explained) >= 5*a {
		return true
	}
	tol := math.Max(0.05, 0.02*a)
	// Double-count / out-of-phase: residual ≈ ±trade PnL, or residual is a fraction of
	// the same-tick trade move with opposite sign (false deposit of a loss size).
	if explained != 0 && amount*explained < 0 && a <= math.Abs(explained)+tol {
		return true
	}
	return math.Abs(math.Abs(explained)-a) <= tol
}
