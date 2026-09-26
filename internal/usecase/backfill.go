package usecase

import (
	"context"

	"spikeidx/internal/domain"
)

func Backfill(ctx context.Context, provider domain.MarketDataProvider, repos domain.OHLCVRepository, code domain.Code) (int, error) {
	candles, err := provider.DailyOHLCV(ctx, code)

	if err != nil {
		return 0, err
	}

	rows := make([]domain.OHLCV, 0, len(candles))

	for _, c := range candles {
		rows = append(rows, domain.OHLCV{
			Code: code, Date: domain.NewTradingDate(c.Date).Time(),
			Open: c.Open, High: c.High, Low: c.Low, Close: c.Close, Volume: c.Volume,
		})
	}

	if err := repos.UpsertBatch(ctx, rows); err != nil {
		return 0, err
	}

	return len(rows), nil
}
