package domain

import (
	"context"
	"errors"
)

var ErrStockUnknown = errors.New("stock unknown: search first")

type MarketDataProvider interface {
	Search(ctx context.Context, query string) ([]Stock, error)
	DailyOHLCV(ctx context.Context, yahooSymbol string) ([]Candle, error)
}

type StockRepository interface {
	Ensure(ctx context.Context, s Stock) error
	Get(ctx context.Context, code string) (Stock, error)
}

type OHLCVRepository interface {
	UpsertBatch(ctx context.Context, rows []OHLCV) error
	History(ctx context.Context, code string, limit int) ([]OHLCV, error)
}

type SignalRepository interface {
	Upsert(ctx context.Context, s Signal) error
	ByDate(ctx context.Context, date string, includeFiltered bool) ([]Signal, error)
}

type WatchlistRepository interface {
	List(ctx context.Context, userID string) ([]string, error)
	Add(ctx context.Context, userID, code string) error
	Remove(ctx context.Context, userID, code string) error
	Count(ctx context.Context, userID string) (int, error)
}
