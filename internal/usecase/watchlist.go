package usecase

import (
	"context"

	"spikeidx/internal/domain"
)

func AddToWatchlist(ctx context.Context, stocks domain.StockRepository, wl domain.WatchlistRepository, input string) (domain.Code, error) {
	code, err := domain.ParseCode(input)

	if err != nil {
		return "", err
	}

	if _, err := stocks.Get(ctx, code); err != nil {
		return "", err
	}

	n, err := wl.Count(ctx, domain.DefaultUser)

	if err != nil {
		return "", err
	}

	if n >= domain.MaxWatchlist {
		return "", domain.ErrWatchlistFull
	}

	if err := wl.Add(ctx, domain.DefaultUser, code); err != nil {
		return "", err
	}

	return code, nil
}

func RemoveFromWatchlist(ctx context.Context, wl domain.WatchlistRepository, input string) error {
	code, err := domain.ParseCode(input)

	if err != nil {
		return err
	}

	return wl.Remove(ctx, domain.DefaultUser, code)
}
