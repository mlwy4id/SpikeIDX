package usecase

import "spikeidx/internal/domain"

func ADL(hist []domain.OHLCV) []float64 {
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

func ADLSlope5(adl []float64) float64 {
	if len(adl) < 6 {
		return 0
	}
	
	return adl[len(adl)-1] - adl[len(adl)-6]
}
