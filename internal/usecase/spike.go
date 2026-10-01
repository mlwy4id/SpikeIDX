package usecase

import (
	"math"
	"sort"

	"spikeidx/internal/domain"
)

func Stats(hist []domain.OHLCV, rule domain.SpikeRule) (avg, multiple, zScore, pctChange float64, ok bool) {
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
		zScore = (float64(last.Volume) - avg) / std
	}

	if prev.Close > 0 {
		pctChange = (last.Close - prev.Close) / prev.Close * 100
	}

	return avg, multiple, zScore, pctChange, true
}

func IsSpike(multiple, zScore, pctChange float64, rule domain.SpikeRule) (spike, isFiltered bool) {
	if multiple > rule.MultipleMin && zScore > rule.ZScoreMin {
		if rule.IsPriceFilterEnabled && math.Abs(pctChange) < rule.PctChangeMin {
			return true, true
		}

		return true, false
	}

	return false, false
}
