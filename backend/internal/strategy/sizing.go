package strategy

func RiskUSD(equity, riskPct, maxRiskUSD float64) float64 {
	raw := equity * (riskPct / 100.0)
	if maxRiskUSD > 0 && raw > maxRiskUSD {
		return maxRiskUSD
	}
	return raw
}

func Quantity(equity, riskPct, maxRiskUSD, riskDistance float64) float64 {
	if riskDistance <= 0 {
		return 0
	}
	return RiskUSD(equity, riskPct, maxRiskUSD) / riskDistance
}

func ApplyFillCosts(equity, qty, entry, exit float64, dir Direction, outcome Outcome, riskUSD, feeRate float64) (gross, entryFee, exitFee, net, newEquity float64) {
	notional := qty * entry
	entryFee = notional * feeRate
	exitFee = qty * exit * feeRate
	sign := 1.0
	if dir == DirSell {
		sign = -1.0
	}
	if outcome == OutcomeSL {
		gross = -riskUSD
	} else {
		gross = qty * (exit - entry) * sign
	}
	net = gross - entryFee - exitFee
	newEquity = equity - entryFee + gross - exitFee
	return
}

func GrossMove(dir Direction, entry, exit float64) float64 {
	sign := 1.0
	if dir == DirSell {
		sign = -1.0
	}
	return (exit - entry) * sign
}
