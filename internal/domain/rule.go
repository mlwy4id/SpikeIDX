package domain

type SpikeRule struct {
	MultipleMin  float64
	ZScoreMin    float64
	CMFWeakMin   float64
	CMFStrongMin float64
	PctChangeMin float64
	IsPriceFilterEnabled bool
}

func DefaultSpikeRule() SpikeRule {
	return SpikeRule{
		MultipleMin: 1.5, ZScoreMin: 2.0,
		CMFWeakMin: 0.05, CMFStrongMin: 0.25,
		PctChangeMin: 2.0, IsPriceFilterEnabled: false,
	}
}
