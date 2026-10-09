package main

import (
	"context"
	"errors"
	"testing"
	"time"

	"spikeidx/internal/domain"
	"spikeidx/internal/usecase"
)

type workerWatchlist struct {
	codes []domain.Code
	err   error
}

func (f *workerWatchlist) List(_ context.Context, _ domain.UserID) ([]domain.Code, error) {
	return f.codes, f.err
}

func (f *workerWatchlist) Add(_ context.Context, _ domain.UserID, _ domain.Code) error {
	return nil
}

func (f *workerWatchlist) Remove(_ context.Context, _ domain.UserID, _ domain.Code) error {
	return nil
}

func (f *workerWatchlist) Count(_ context.Context, _ domain.UserID) (int, error) {
	return len(f.codes), nil
}

type workerStocks struct {
	data map[domain.Code]domain.Stock
}

func (f *workerStocks) Ensure(_ context.Context, s domain.Stock) error {
	if f.data == nil {
		f.data = map[domain.Code]domain.Stock{}
	}
	f.data[s.Code] = s
	return nil
}

func (f *workerStocks) Get(_ context.Context, code domain.Code) (domain.Stock, error) {
	if s, ok := f.data[code]; ok {
		return s, nil
	}
	return domain.Stock{}, domain.ErrStockUnknown
}

type workerOHLCV struct {
	data map[domain.Code][]domain.OHLCV
}

func (f *workerOHLCV) UpsertBatch(_ context.Context, rows []domain.OHLCV) error {
	if f.data == nil {
		f.data = map[domain.Code][]domain.OHLCV{}
	}
	for _, r := range rows {
		f.data[r.Code] = append(f.data[r.Code], r)
	}
	return nil
}

func (f *workerOHLCV) History(_ context.Context, code domain.Code, limit int) ([]domain.OHLCV, error) {
	hist := f.data[code]
	if limit > 0 && len(hist) > limit {
		hist = hist[len(hist)-limit:]
	}
	return append([]domain.OHLCV(nil), hist...), nil
}

type workerSignals struct {
	data []domain.Signal
}

func (f *workerSignals) Upsert(_ context.Context, s domain.Signal) error {
	f.data = append(f.data, s)
	return nil
}

func (f *workerSignals) ByDate(_ context.Context, _ domain.TradingDate, _ bool) ([]domain.Signal, error) {
	return f.data, nil
}

type workerProvider struct {
	candles map[domain.Code][]domain.Candle
	calls   int
}

func (f *workerProvider) Search(_ context.Context, _ string) ([]domain.Stock, error) {
	return nil, nil
}

func (f *workerProvider) DailyOHLCV(_ context.Context, code domain.Code) ([]domain.Candle, error) {
	f.calls++
	return f.candles[code], nil
}

func workerCandles(n int, base, last int64, firstClose, lastClose float64) []domain.Candle {
	day := time.Date(2026, 6, 1, 0, 0, 0, 0, time.UTC)
	out := make([]domain.Candle, n)
	for i := range out {
		out[i] = domain.Candle{
			Date: day.AddDate(0, 0, i),
			Open: 100, High: 100, Low: 100, Close: firstClose, Volume: base,
		}
	}
	out[n-1].Volume = last
	out[n-1].Close = lastClose
	return out
}

func workerRepos(ohlcv *workerOHLCV, signals *workerSignals) usecase.IngestRepos {
	return usecase.IngestRepos{
		Stocks:  &workerStocks{},
		OHLCV:   ohlcv,
		Signals: signals,
	}
}

func TestRunEODEmptyWatchlist(t *testing.T) {
	provider := &workerProvider{}
	results, err := runEOD(context.Background(), &workerWatchlist{}, workerRepos(&workerOHLCV{}, &workerSignals{}),
		provider, nil, domain.DefaultSpikeRule())
	if err != nil {
		t.Fatal(err)
	}
	if len(results) != 0 {
		t.Fatalf("expected no results, got %+v", results)
	}
	if provider.calls != 0 {
		t.Fatalf("provider must not be called, got %d calls", provider.calls)
	}
}

func TestRunEODActionableAndFiltered(t *testing.T) {
	provider := &workerProvider{candles: map[domain.Code][]domain.Candle{
		"BBCA": workerCandles(60, 10_000_000, 35_000_000, 100, 103),
		"TLKM": workerCandles(60, 10_000_000, 35_000_000, 100, 100.5),
	}}
	signals := &workerSignals{}
	watchlist := &workerWatchlist{codes: []domain.Code{"BBCA", "TLKM"}}

	results, err := runEOD(context.Background(), watchlist, workerRepos(&workerOHLCV{}, signals),
		provider, nil, domain.DefaultSpikeRule())
	if err != nil {
		t.Fatal(err)
	}
	if len(results) != 2 {
		t.Fatalf("got %+v", results)
	}
	if results[0].Status != usecase.StatusSpikeActionable {
		t.Fatalf("BBCA: %+v", results[0])
	}
	if results[1].Status != usecase.StatusSpikeFiltered {
		t.Fatalf("TLKM: %+v", results[1])
	}
	if len(signals.data) != 2 {
		t.Fatalf("both spikes must persist, got %+v", signals.data)
	}
}

func TestRunEODNoSpike(t *testing.T) {
	provider := &workerProvider{candles: map[domain.Code][]domain.Candle{
		"TLKM": workerCandles(60, 10_000_000, 10_000_000, 100, 100),
	}}
	signals := &workerSignals{}
	watchlist := &workerWatchlist{codes: []domain.Code{"TLKM"}}

	results, err := runEOD(context.Background(), watchlist, workerRepos(&workerOHLCV{}, signals),
		provider, nil, domain.DefaultSpikeRule())
	if err != nil {
		t.Fatal(err)
	}
	if len(results) != 1 || results[0].Status != usecase.StatusNoSpike {
		t.Fatalf("got %+v", results)
	}
	if len(signals.data) != 0 {
		t.Fatalf("no-spike must not persist, got %+v", signals.data)
	}
}

func TestRunEODListError(t *testing.T) {
	watchlist := &workerWatchlist{err: errors.New("db down")}
	_, err := runEOD(context.Background(), watchlist, workerRepos(&workerOHLCV{}, &workerSignals{}),
		&workerProvider{}, nil, domain.DefaultSpikeRule())
	if err == nil {
		t.Fatal("expected list error")
	}
}

func TestRunEODForwardsHolidays(t *testing.T) {
	row := func(day int) domain.OHLCV {
		return domain.OHLCV{Code: "GAPX", Date: time.Date(2026, 9, day, 12, 0, 0, 0, time.UTC)}
	}
	newRun := func(holidays map[domain.TradingDate]bool) []usecase.SymbolResult {
		t.Helper()
		ohlcv := &workerOHLCV{data: map[domain.Code][]domain.OHLCV{"GAPX": {row(21), row(23)}}}
		watchlist := &workerWatchlist{codes: []domain.Code{"GAPX"}}
		results, err := runEOD(context.Background(), watchlist, workerRepos(ohlcv, &workerSignals{}),
			&workerProvider{}, holidays, domain.DefaultSpikeRule())
		if err != nil {
			t.Fatal(err)
		}
		if len(results) != 1 {
			t.Fatalf("got %+v", results)
		}
		return results
	}

	withoutHolidays := newRun(nil)
	if withoutHolidays[0].Status != usecase.StatusSkipped ||
		withoutHolidays[0].Reason != "gap:2026-09-22 missing" {
		t.Fatalf("without holidays must gap-skip, got %+v", withoutHolidays[0])
	}

	tue, err := domain.ParseTradingDate("2026-09-22")
	if err != nil {
		t.Fatal(err)
	}
	withHoliday := newRun(map[domain.TradingDate]bool{tue: true})
	if withHoliday[0].Status != usecase.StatusSkipped ||
		withHoliday[0].Reason == "gap:2026-09-22 missing" {
		t.Fatalf("holiday must change the outcome, got %+v", withHoliday[0])
	}
}
