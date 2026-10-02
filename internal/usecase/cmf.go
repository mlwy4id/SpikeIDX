package usecase

import (
	"sort"

	"spikeidx/internal/domain"
)

// CMF computes the 20-bar Chaikin Money Flow:
// sum(MoneyFlowVolume, 20) / sum(Volume, 20).
// Guards: fewer than 20 bars -> 0, flat bars contribute 0, zero total volume -> 0.
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
		sumMFV += moneyFlowMultiplier(h) * float64(h.Volume)
		sumVol += float64(h.Volume)
	}
	if sumVol == 0 {
		return 0
	}
	return sumMFV / sumVol
}
