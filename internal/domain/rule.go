package domain

type SpikeRule struct {
	MultipleMin          float64
	ZScoreMin            float64
	PctChangeMin         float64
	IsPriceFilterEnabled bool
	ADLSlopeWindow       int
	ADLSlopeMinRatio     float64
}

func DefaultSpikeRule() SpikeRule {
	return SpikeRule{
		MultipleMin: 2.0, ZScoreMin: 2.0,
		PctChangeMin: 2.0, IsPriceFilterEnabled: true,
		ADLSlopeWindow: 20, ADLSlopeMinRatio: 0.10,
	}
}
