package usecase

import (
	"math"
	"sort"

	"spikeidx/internal/domain"
)

func Stats(hist []domain.OHLCV, rule domain.SpikeRule) (avg, multiple, z, pct float64, ok bool) {
	if len(hist) < 20 {
		return 0, 0, 0, 0, false
	}
	sorted := append([]domain.OHLCV(nil), hist...)
	sort.SliceStable(sorted, func(i, j int) bool { return sorted[i].Date.Before(sorted[j].Date) })
	window := sorted[len(sorted)-20:]
	var sum float64
	for _, h := range window {
		sum += float64(h.Volume)
	}
	avg = sum / 20

	var variance float64
	for _, h := range window {
		d := float64(h.Volume) - avg
		variance += d * d
	}
	std := math.Sqrt(variance / 20)

	last := sorted[len(sorted)-1]
	prev := sorted[len(sorted)-2]
	if avg > 0 {
		multiple = float64(last.Volume) / avg
	}
	if std > 0 {
		z = (float64(last.Volume) - avg) / std
	}
	if prev.Close > 0 {
		pct = (last.Close - prev.Close) / prev.Close * 100
	}
	return avg, multiple, z, pct, true
}

func IsSpike(multiple, z, pct float64, rule domain.SpikeRule) (spike, filtered bool) {
	if multiple > rule.MultipleMin && z > rule.ZScoreMin {
		if rule.PriceFilterEnabled && math.Abs(pct) < rule.PctChangeMin {
			return true, true
		}
		return true, false
	}
	return false, false
}
