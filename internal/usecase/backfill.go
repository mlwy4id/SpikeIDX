package usecase

import (
	"context"

	"spikeidx/internal/domain"
)

func Backfill(ctx context.Context, provider domain.MarketDataProvider, store domain.OHLCVRepository, code domain.Code) (int, error) {
	candles, err := provider.DailyOHLCV(ctx, code)
	if err != nil {
		return 0, err
	}
	rows := make([]domain.OHLCV, 0, len(candles))
	for _, c := range candles {
		rows = append(rows, domain.OHLCV{
			Code: code, Date: c.Date,
			Open: c.Open, High: c.High, Low: c.Low, Close: c.Close, Volume: c.Volume,
		})
	}
	if err := store.UpsertBatch(ctx, rows); err != nil {
		return 0, err
	}
	return len(rows), nil
}
