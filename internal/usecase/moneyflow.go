package usecase

import "spikeidx/internal/domain"

// moneyFlowMultiplier returns where the close sits within the bar's
// high-low range, in [-1, 1]. Positive means accumulation (close in the
// upper half), negative means distribution. A flat bar carries no
// location information, so it yields 0 instead of dividing by zero.
func moneyFlowMultiplier(h domain.OHLCV) float64 {
	if h.High == h.Low {
		return 0
	}

	return ((h.Close - h.Low) - (h.High - h.Close)) / (h.High - h.Low)
}
