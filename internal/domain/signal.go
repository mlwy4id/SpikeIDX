package domain

import "time"

type Signal struct {
	Code       Code
	Date       time.Time
	Volume     int64
	Avg20      float64
	Multiple   float64
	ZScore     float64
	Close      float64
	PctChange  float64
	ADL        float64
	ADLSlope5  float64
	CMF        float64
	IsFiltered bool
}

func (s Signal) IsActionable() bool { return !s.IsFiltered }

func (s Signal) Interpretation() string {
	if s.IsFiltered {
		return "terfilter (noise harga)"
	}

	rule := DefaultSpikeRule()
	regime := "markup"
	if abs(s.PctChange) < rule.PctChangeMin {
		regime = "sideways"
	}

	switch {
	case s.CMF >= rule.CMFStrongMin:
		return "akumulasi kuat " + regime
	case s.CMF >= rule.CMFWeakMin:
		return "akumulasi lemah " + regime
	case s.CMF != 0:
		// CMF terisi tapi di bawah ambang akumulasi: fallback ke ADL.
		if s.ADLSlope5 < 0 {
			return "distribusi"
		}
		return "netral"
	default:
		// Baris lama tanpa CMF (nol): pertahankan semantik ADL lama.
		switch {
		case s.ADLSlope5 > 0:
			return "akumulasi"
		case s.ADLSlope5 < 0:
			return "distribusi"
		default:
			return "netral"
		}
	}
}

func abs(v float64) float64 {
	if v < 0 {
		return -v
	}
	return v
}
