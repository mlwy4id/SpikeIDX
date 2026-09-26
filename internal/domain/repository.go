package domain

import "context"

type UserID string

const DefaultUser UserID = "default"

type MarketDataProvider interface {
	Search(ctx context.Context, query string) ([]Stock, error)
	DailyOHLCV(ctx context.Context, code Code) ([]Candle, error)
}

type StockRepository interface {
	Ensure(ctx context.Context, s Stock) error
	Get(ctx context.Context, code Code) (Stock, error)
}

type OHLCVRepository interface {
	UpsertBatch(ctx context.Context, rows []OHLCV) error
	History(ctx context.Context, code Code, limit int) ([]OHLCV, error)
}

type SignalRepository interface {
	Upsert(ctx context.Context, s Signal) error
	ByDate(ctx context.Context, date TradingDate, shouldIncludeFiltered bool) ([]Signal, error)
}

type WatchlistRepository interface {
	List(ctx context.Context, user UserID) ([]Code, error)
	Add(ctx context.Context, user UserID, code Code) error
	Remove(ctx context.Context, user UserID, code Code) error
	Count(ctx context.Context, user UserID) (int, error)
}
