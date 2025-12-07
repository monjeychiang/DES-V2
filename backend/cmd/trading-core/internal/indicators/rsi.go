package indicators

// RSI computes a basic Relative Strength Index with smoothing disabled for simplicity.
func RSI(values []float64, period int) float64 {
	if period <= 0 || len(values) < period+1 {
		return 0
	}

	gain := 0.0
	loss := 0.0
	for i := len(values) - period; i < len(values); i++ {
		change := values[i] - values[i-1]
		if change > 0 {
			gain += change
		} else {
			loss -= change
		}
	}

	if loss == 0 {
		return 100
	}
	rs := gain / loss
	return 100 - (100 / (1 + rs))
}
