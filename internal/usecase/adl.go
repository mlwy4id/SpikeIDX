package usecase

import "spikeidx/internal/domain"

func ADL(hist []domain.OHLCV) []float64 {
	out := make([]float64, len(hist))
	var cur float64

	for i, h := range hist {
		cur += moneyFlowMultiplier(h) * float64(h.Volume)
		out[i] = cur
	}

	return out
}

func ADLSlope(adl []float64, window int) float64 {
	if window <= 0 || len(adl) <= window {
		return 0
	}

	return adl[len(adl)-1] - adl[len(adl)-1-window]
}

func ADLSlope5(adl []float64) float64 {
	return ADLSlope(adl, 5)
}
