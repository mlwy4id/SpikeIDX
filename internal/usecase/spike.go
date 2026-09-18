package usecase

import (
	"math"

	"spikeidx/internal/domain"
)

// SpikeRule holds the v1 detection thresholds.
type SpikeRule struct {
	MultipleMin  float64 // e.g. 2.0
	ZScoreMin    float64 // e.g. 2.0
	PctChangeMin float64 // e.g. 2.0 (absolute)
}

// DefaultSpikeRule matches architecture.md.
func DefaultSpikeRule() SpikeRule {
	return SpikeRule{MultipleMin: 2.0, ZScoreMin: 2.0, PctChangeMin: 2.0}
}

// Stats computes avg20, multiple, z-score and pct-change for the last bar.
// hist must be oldest-first and contain >= 20 rows; the signal is for hist[len-1].
func Stats(hist []domain.OHLCV, rule SpikeRule) (avg, multiple, z, pct float64, ok bool) {
	if len(hist) < 20 {
		return 0, 0, 0, 0, false
	}
	window := hist[len(hist)-20:]
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

	last := hist[len(hist)-1]
	prev := hist[len(hist)-2]
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

// IsSpike applies the rule. Returns (spike, filteredByPrice).
func IsSpike(multiple, z, pct float64, rule SpikeRule) (spike, filtered bool) {
	if multiple > rule.MultipleMin && z > rule.ZScoreMin {
		if math.Abs(pct) < rule.PctChangeMin {
			return true, true // spike but noise-filtered
		}
		return true, false
	}
	return false, false
}
