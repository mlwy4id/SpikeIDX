package postgres

import (
	"context"
	"errors"
	"fmt"

	"spikeidx/internal/domain"
)

// SeedStocks ensures every stock in stocks exists in stocks_master.
//
// It is idempotent: each code is looked up with Get before Ensure, and only
// codes unknown to the repo count toward inserted. Seeding the same list
// twice yields inserted==0 on the second run. Re-seeding still calls Ensure
// for known codes so name/sector metadata is refreshed via upsert.
//
// All codes are validated with domain.ParseCode before the repo is touched:
// an invalid code aborts with domain.ErrInvalidCode and seeds nothing.
// Any other Get/Ensure failure aborts with a wrapped error.
func SeedStocks(ctx context.Context, repo domain.StockRepository, stocks []domain.Stock) (inserted int, err error) {
	normalized := make([]domain.Stock, 0, len(stocks))
	for _, s := range stocks {
		code, perr := domain.ParseCode(string(s.Code))
		if perr != nil {
			return 0, perr
		}
		s.Code = code
		normalized = append(normalized, s)
	}

	for _, s := range normalized {
		_, gerr := repo.Get(ctx, s.Code)
		if gerr != nil && !errors.Is(gerr, domain.ErrStockUnknown) {
			return inserted, fmt.Errorf("seed stocks: get %s: %w", s.Code, gerr)
		}
		isNew := errors.Is(gerr, domain.ErrStockUnknown)
		if eerr := repo.Ensure(ctx, s); eerr != nil {
			return inserted, fmt.Errorf("seed stocks: ensure %s: %w", s.Code, eerr)
		}
		if isNew {
			inserted++
		}
	}
	return inserted, nil
}
