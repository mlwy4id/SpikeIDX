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
	IsFiltered bool
}

func (s Signal) IsActionable() bool { return !s.IsFiltered }

func (s Signal) Interpretation() string {
	if s.IsFiltered {
		return "terfilter (noise harga)"
	}

	switch {
	case s.ADLSlope5 > 0:
		return "akumulasi"
	case s.ADLSlope5 < 0:
		return "distribusi"
	default:
		return "netral"
	}
}
