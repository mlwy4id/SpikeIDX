package usecase

import (
	"context"
	"errors"
	"testing"

	"spikeidx/internal/domain"
)

func TestAddToWatchlist(t *testing.T) {
	ctx := context.Background()
	stocks := &fakeStocks{}
	wl := &fakeWatchlist{}
	_ = stocks.Ensure(ctx, domain.Stock{Code: "BBCA", Name: "Bank Central Asia"})

	code, err := AddToWatchlist(ctx, stocks, wl, "bbca.jk")
	if err != nil {
		t.Fatal(err)
	}
	if code != "BBCA" {
		t.Fatalf("got %q", code)
	}
	if n, _ := wl.Count(ctx, domain.DefaultUser); n != 1 {
		t.Fatalf("count = %d", n)
	}
}

func TestAddToWatchlistUnknown(t *testing.T) {
	ctx := context.Background()
	_, err := AddToWatchlist(ctx, &fakeStocks{}, &fakeWatchlist{}, "BBCA")
	if !errors.Is(err, domain.ErrStockUnknown) {
		t.Fatalf("expected ErrStockUnknown, got %v", err)
	}
}

func TestAddToWatchlistInvalidCode(t *testing.T) {
	ctx := context.Background()
	_, err := AddToWatchlist(ctx, &fakeStocks{}, &fakeWatchlist{}, "BBCA1")
	if !errors.Is(err, domain.ErrInvalidCode) {
		t.Fatalf("expected ErrInvalidCode, got %v", err)
	}
}

func TestAddToWatchlistFull(t *testing.T) {
	ctx := context.Background()
	stocks := &fakeStocks{}
	wl := &fakeWatchlist{}
	for i := 0; i < domain.MaxWatchlist; i++ {
		code := domain.Code(string([]byte{'Q', 'A', byte('A' + i/26), byte('A' + i%26)}))
		_ = stocks.Ensure(ctx, domain.Stock{Code: code})
		_ = wl.Add(ctx, domain.DefaultUser, code)
	}
	_ = stocks.Ensure(ctx, domain.Stock{Code: "ZZZZ"})
	_, err := AddToWatchlist(ctx, stocks, wl, "ZZZZ")
	if !errors.Is(err, domain.ErrWatchlistFull) {
		t.Fatalf("expected ErrWatchlistFull, got %v", err)
	}
}

func TestRemoveFromWatchlist(t *testing.T) {
	ctx := context.Background()
	wl := &fakeWatchlist{}
	_ = wl.Add(ctx, domain.DefaultUser, "BBCA")
	if err := RemoveFromWatchlist(ctx, wl, "bbca"); err != nil {
		t.Fatal(err)
	}
	if n, _ := wl.Count(ctx, domain.DefaultUser); n != 0 {
		t.Fatalf("count = %d", n)
	}
	if err := RemoveFromWatchlist(ctx, wl, "!!"); !errors.Is(err, domain.ErrInvalidCode) {
		t.Fatalf("expected ErrInvalidCode, got %v", err)
	}
}
