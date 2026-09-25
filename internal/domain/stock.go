package domain

import "time"

type Stock struct {
	Code        Code
	YahooSymbol string
	Name        string
	Sector      string
}

type Candle struct {
	Date   time.Time
	Open   float64
	High   float64
	Low    float64
	Close  float64
	Volume int64
}

type OHLCV struct {
	Code   Code
	Date   time.Time
	Open   float64
	High   float64
	Low    float64
	Close  float64
	Volume int64
}
