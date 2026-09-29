package usecase

import (
	"sort"

	"spikeidx/internal/domain"
)

// CMF menghitung Chaikin Money Flow 20 bar terakhir:
// sum(MoneyFlowVolume,20) / sum(Volume,20), dengan
// MoneyFlowMultiplier = ((Close-Low)-(High-Close))/(High-Low).
// Guard: <20 bar -> 0, High==Low -> MFM 0, total volume 0 -> 0.
func CMF(hist []domain.OHLCV) float64 {
	if len(hist) < 20 {
		return 0
	}
	sorted := append([]domain.OHLCV(nil), hist...)
	sort.SliceStable(sorted, func(i, j int) bool { return sorted[i].Date.Before(sorted[j].Date) })
	window := sorted[len(sorted)-20:]

	var sumMFV, sumVol float64
	for _, h := range window {
		if h.High == h.Low {
			continue
		}
		mfm := ((h.Close - h.Low) - (h.High - h.Close)) / (h.High - h.Low)
		sumMFV += mfm * float64(h.Volume)
		sumVol += float64(h.Volume)
	}
	if sumVol == 0 {
		return 0
	}
	return sumMFV / sumVol
}
