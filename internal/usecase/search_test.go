package usecase

import (
	"context"
	"errors"
	"testing"
	"time"

	"spikeidx/internal/domain"
)

func day(d int) time.Time {
	return time.Date(2026, 6, d, 0, 0, 0, 0, time.UTC)
}

func TestSearchAndCache(t *testing.T) {
	ctx := context.Background()
	stocks := &fakeStocks{}
	primary := &fakeProvider{search: []domain.Stock{{Code: "BBCA", YahooSymbol: "BBCA.JK", Name: "Bank Central Asia"}}}

	res, err := SearchAndCache(ctx, primary, nil, stocks, "bank")
	if err != nil {
		t.Fatal(err)
	}
	if len(res) != 1 || res[0].Code != "BBCA" {
		t.Fatalf("got %+v", res)
	}
	if _, err := stocks.Get(ctx, "BBCA"); err != nil {
		t.Fatalf("expected cached stock, got %v", err)
	}
}

func TestSearchAndCacheFallback(t *testing.T) {
	ctx := context.Background()
	stocks := &fakeStocks{}
	primary := &fakeProvider{searchErr: errors.New("yahoo down")}
	fallback := &fakeProvider{search: []domain.Stock{{Code: "TLKM", YahooSymbol: "TLKM.JK"}}}

	res, err := SearchAndCache(ctx, primary, fallback, stocks, "telkom")
	if err != nil || len(res) != 1 {
		t.Fatalf("got %+v, %v", res, err)
	}
}

func TestSearchAndCacheBothFail(t *testing.T) {
	ctx := context.Background()
	primary := &fakeProvider{searchErr: errors.New("yahoo down")}
	fallback := &fakeProvider{searchErr: errors.New("idx down")}
	if _, err := SearchAndCache(ctx, primary, fallback, &fakeStocks{}, "x"); err == nil {
		t.Fatal("expected error")
	}
}

func TestSearchAndCacheEmpty(t *testing.T) {
	ctx := context.Background()
	res, err := SearchAndCache(ctx, &fakeProvider{}, nil, &fakeStocks{}, "zzz")
	if err != nil {
		t.Fatal(err)
	}
	if res == nil || len(res) != 0 {
		t.Fatalf("expected empty non-nil slice, got %#v", res)
	}
}

func TestBackfill(t *testing.T) {
	ctx := context.Background()
	store := &fakeOHLCV{}
	provider := &fakeProvider{candles: map[domain.Code][]domain.Candle{
		"BBCA": {
			{Date: day(1), Close: 100, High: 101, Low: 99, Volume: 1000},
			{Date: day(2), Close: 102, High: 103, Low: 101, Volume: 2000},
		},
	}}

	n, err := Backfill(ctx, provider, store, "BBCA")
	if err != nil || n != 2 {
		t.Fatalf("got %d, %v", n, err)
	}
	hist, _ := store.History(ctx, "BBCA", 0)
	if len(hist) != 2 || hist[1].Close != 102 || hist[1].Volume != 2000 {
		t.Fatalf("got %+v", hist)
	}
}

func TestBackfillProviderError(t *testing.T) {
	ctx := context.Background()
	provider := &fakeProvider{candleErr: errors.New("429")}
	if _, err := Backfill(ctx, provider, &fakeOHLCV{}, "BBCA"); err == nil {
		t.Fatal("expected error")
	}
}

func TestBackfillNormalizesWIB(t *testing.T) {
	ctx := context.Background()
	store := &fakeOHLCV{}
	provider := &fakeProvider{candles: map[domain.Code][]domain.Candle{
		"BBCA": {
			{Date: time.Date(2026, 9, 18, 18, 0, 0, 0, time.UTC), Close: 100},
		},
	}}

	if _, err := Backfill(ctx, provider, store, "BBCA"); err != nil {
		t.Fatal(err)
	}
	hist, _ := store.History(ctx, "BBCA", 0)
	if len(hist) != 1 {
		t.Fatalf("got %+v", hist)
	}
	want, _ := domain.ParseTradingDate("2026-09-19")
	if got := domain.NewTradingDate(hist[0].Date); !got.Equal(want) {
		t.Fatalf("got %v, want %v", got, want)
	}
}
