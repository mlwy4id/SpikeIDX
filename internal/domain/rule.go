package domain

type SpikeRule struct {
	MultipleMin        float64
	ZScoreMin          float64
	PctChangeMin       float64
	PriceFilterEnabled bool
}

func DefaultSpikeRule() SpikeRule {
	return SpikeRule{MultipleMin: 2.0, ZScoreMin: 2.0, PctChangeMin: 2.0, PriceFilterEnabled: true}
}
