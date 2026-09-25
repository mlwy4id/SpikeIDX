package domain

import "errors"

var (
	ErrStockUnknown     = errors.New("stock unknown: search first")
	ErrInvalidCode      = errors.New("invalid stock code")
	ErrWatchlistFull    = errors.New("watchlist penuh (100/100)")
	ErrInsufficientData = errors.New("insufficient data: need >= 20 rows")
)
