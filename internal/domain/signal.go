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

func (s Signal) Interpretation() string { return s.InterpretationWithRule(DefaultSpikeRule()) }

func (s Signal) InterpretationWithRule(rule SpikeRule) string {
	if s.IsFiltered {
		return "terfilter (noise harga)"
	}

	window := rule.ADLSlopeWindow
	if window <= 0 {
		window = DefaultSpikeRule().ADLSlopeWindow
	}

	minRatio := rule.ADLSlopeMinRatio
	if minRatio < 0 {
		minRatio = 0
	}

	if avg := s.Avg20; avg <= 0 || window <= 0 {
		return "netral"
	}

	denom := s.Avg20 * float64(window)
	if denom == 0 {
		return "netral"
	}

	// Rasio CMF-style: slope/(avg*window) dalam [-1,1].
	switch ratio := s.ADLSlope5 / denom; {
	case ratio > minRatio:
		return "akumulasi"
	case ratio < -minRatio:
		return "distribusi"
	default:
		return "netral"
	}
}
