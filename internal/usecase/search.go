package usecase

import (
	"context"

	"spikeidx/internal/domain"
)

func SearchAndCache(ctx context.Context, primary domain.MarketDataProvider, fallback domain.MarketDataProvider, stocks domain.StockRepository, query string) ([]domain.Stock, error) {
	res, err := primary.Search(ctx, query)
	if err != nil && fallback != nil {
		res, err = fallback.Search(ctx, query)
	}
	if err != nil {
		return nil, err
	}
	for _, s := range res {
		if err := stocks.Ensure(ctx, s); err != nil {
			return nil, err
		}
	}
	if res == nil {
		res = []domain.Stock{}
	}
	return res, nil
}
