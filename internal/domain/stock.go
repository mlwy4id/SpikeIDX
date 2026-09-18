package domain

import "time"

type Stock struct {
	Code        string
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
	Code   string
	Date   time.Time
	Open   float64
	High   float64
	Low    float64
	Close  float64
	Volume int64
}
