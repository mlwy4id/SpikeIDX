package infra

import (
	"context"

	"spikeidx/internal/domain"
	"spikeidx/internal/infra/postgres"
)

type Repos struct {
	Stocks    domain.StockRepository
	Watchlist domain.WatchlistRepository
	OHLCV     domain.OHLCVRepository
	Signals   domain.SignalRepository
	Close     func()
}

func WireStrict(ctx context.Context, dsn string) (*Repos, error) {
	db, err := postgres.Connect(ctx, dsn)
	if err != nil {
		return nil, err
	}
	if err := db.Migrate(ctx); err != nil {
		db.Close()
		return nil, err
	}
	return &Repos{
		Stocks:    postgres.NewStockRepo(db),
		Watchlist: postgres.NewWatchlistRepo(db),
		OHLCV:     postgres.NewOHLCVRepo(db),
		Signals:   postgres.NewSignalRepo(db),
		Close:     db.Close,
	}, nil
}
