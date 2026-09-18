package usecase

import "spikeidx/internal/domain"

// ADL computes the Chaikin Accumulation/Distribution Line over hist (oldest-first).
// Returns the full ADL series aligned with hist.
func AccumulationDistributionLine(hist []domain.OHLCV) []float64 {
	out := make([]float64, len(hist))
	var cur float64
	for i, h := range hist {
		var mfm float64
		if h.High != h.Low {
			mfm = ((h.Close - h.Low) - (h.High - h.Close)) / (h.High - h.Low)
		}
		cur += mfm * float64(h.Volume)
		out[i] = cur
	}
	return out
}

// ADLSlope5 returns ADL[last] - ADL[last-5]. Positive means accumulation.
func AccumulationDistributionLineSlope5(adl []float64) float64 {
	if len(adl) < 6 {
		return 0
	}
	return adl[len(adl)-1] - adl[len(adl)-6]
}
