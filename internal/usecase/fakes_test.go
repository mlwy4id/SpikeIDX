package usecase

import (
	"context"
	"sort"

	"spikeidx/internal/domain"
)

type fakeStocks struct {
	data map[domain.Code]domain.Stock
}

func (f *fakeStocks) Ensure(_ context.Context, s domain.Stock) error {
	if f.data == nil {
		f.data = map[domain.Code]domain.Stock{}
	}
	
	f.data[s.Code] = s
	return nil
}

func (f *fakeStocks) Get(_ context.Context, code domain.Code) (domain.Stock, error) {
	if s, ok := f.data[code]; ok {
		return s, nil
	}
	
	return domain.Stock{}, domain.ErrStockUnknown
}

type fakeWatchlist struct {
	data map[domain.Code]bool
}

func (f *fakeWatchlist) List(_ context.Context, _ domain.UserID) ([]domain.Code, error) {
	out := []domain.Code{}

	for c := range f.data {
		out = append(out, c)
	}
	
	sort.Slice(out, func(i, j int) bool { return out[i] < out[j] })
	return out, nil
}

func (f *fakeWatchlist) Add(_ context.Context, _ domain.UserID, code domain.Code) error {
	if f.data == nil {
		f.data = map[domain.Code]bool{}
	}
	
	f.data[code] = true
	return nil
}

func (f *fakeWatchlist) Remove(_ context.Context, _ domain.UserID, code domain.Code) error {
	delete(f.data, code)
	return nil
}

func (f *fakeWatchlist) Count(_ context.Context, _ domain.UserID) (int, error) {
	return len(f.data), nil
}

type fakeOHLCV struct {
	data map[domain.Code][]domain.OHLCV
}

func (f *fakeOHLCV) UpsertBatch(_ context.Context, rows []domain.OHLCV) error {
	if f.data == nil {
		f.data = map[domain.Code][]domain.OHLCV{}
	}
	
	for _, r := range rows {
		hist := f.data[r.Code]
		replaced := false

		for i, h := range hist {
			if h.Date.Equal(r.Date) {
				hist[i] = r
				replaced = true
				break
			}
		}
		
		if !replaced {
			hist = append(hist, r)
		}
		
		sort.Slice(hist, func(i, j int) bool { return hist[i].Date.Before(hist[j].Date) })
		f.data[r.Code] = hist
	}
	
	return nil
}

func (f *fakeOHLCV) History(_ context.Context, code domain.Code, limit int) ([]domain.OHLCV, error) {
	hist := f.data[code]

	if limit > 0 && len(hist) > limit {
		hist = hist[len(hist)-limit:]
	}
	
	return append([]domain.OHLCV(nil), hist...), nil
}

type fakeSignals struct {
	data []domain.Signal
}

func (f *fakeSignals) Upsert(_ context.Context, s domain.Signal) error {
	for i, e := range f.data {
		if e.Code == s.Code && e.Date.Equal(s.Date) {
			f.data[i] = s
			return nil
		}
	}
	
	f.data = append(f.data, s)
	return nil
}

func (f *fakeSignals) ByDate(_ context.Context, date domain.TradingDate, includeFiltered bool) ([]domain.Signal, error) {
	var out []domain.Signal
	for _, e := range f.data {
		if !domain.NewTradingDate(e.Date).Equal(date) {
			continue
		}
		
		if e.IsFiltered && !includeFiltered {
			continue
		}
		
		out = append(out, e)
	}
	
	return out, nil
}

type fakeProvider struct {
	search    []domain.Stock
	searchErr error
	candles   map[domain.Code][]domain.Candle
	candleErr error
	codeErrs  map[domain.Code]error
}

func (f *fakeProvider) Search(_ context.Context, _ string) ([]domain.Stock, error) {
	return f.search, f.searchErr
}

func (f *fakeProvider) DailyOHLCV(_ context.Context, code domain.Code) ([]domain.Candle, error) {
	if err, ok := f.codeErrs[code]; ok {
		return nil, err
	}
	
	return f.candles[code], f.candleErr
}
