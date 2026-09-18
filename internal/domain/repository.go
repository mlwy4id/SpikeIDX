package domain

import "context"

// MarketDataProvider fetches EOD data. Yahoo is primary, IDX is fallback.
type MarketDataProvider interface {
	Search(ctx context.Context, query string) ([]Stock, error)
	DailyOHLCV(ctx context.Context, yahooSymbol string) ([]Candle, error)
}

// OHLCVRepository persists candles.
type OHLCVRepository interface {
	UpsertBatch(ctx context.Context, rows []OHLCV) error
	History(ctx context.Context, code string, limit int) ([]OHLCV, error)
}

// SignalRepository persists detection results.
type SignalRepository interface {
	Upsert(ctx context.Context, s Signal) error
	ByDate(ctx context.Context, date string, includeFiltered bool) ([]Signal, error)
}

// WatchlistRepository stores user watchlists.
type WatchlistRepository interface {
	List(ctx context.Context, userID string) ([]string, error)
	Add(ctx context.Context, userID, code string) error
	Remove(ctx context.Context, userID, code string) error
	Count(ctx context.Context, userID string) (int, error)
}
