package domain

import "time"

type Signal struct {
	Code       string
	Date       time.Time
	Volume     int64
	Avg20      float64
	Multiple   float64 // Volume / Avg20
	ZScore     float64
	Close      float64
	PctChange  float64
	ADL        float64
	ADLSlope5  float64
	IsFiltered bool // true when filtered out by price-change rule
}
