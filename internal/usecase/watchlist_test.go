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

	if err := stocks.Ensure(ctx, domain.Stock{Code: "BBCA", Name: "Bank Central Asia"}); err != nil {
		t.Fatal(err)
	}

	code, err := AddToWatchlist(ctx, stocks, wl, "bbca.jk")

	if err != nil {
		t.Fatal(err)
	}

	if code != "BBCA" {
		t.Fatalf("got %q", code)
	}

	n, err := wl.Count(ctx, domain.DefaultUser)

	if err != nil {
		t.Fatal(err)
	}

	if n != 1 {
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

		if err := stocks.Ensure(ctx, domain.Stock{Code: code}); err != nil {
			t.Fatal(err)
		}

		if err := wl.Add(ctx, domain.DefaultUser, code); err != nil {
			t.Fatal(err)
		}
	}

	if err := stocks.Ensure(ctx, domain.Stock{Code: "ZZZZ"}); err != nil {
		t.Fatal(err)
	}

	_, err := AddToWatchlist(ctx, stocks, wl, "ZZZZ")

	if !errors.Is(err, domain.ErrWatchlistFull) {
		t.Fatalf("expected ErrWatchlistFull, got %v", err)
	}
}

func TestRemoveFromWatchlist(t *testing.T) {
	ctx := context.Background()
	wl := &fakeWatchlist{}

	if err := wl.Add(ctx, domain.DefaultUser, "BBCA"); err != nil {
		t.Fatal(err)
	}

	if err := RemoveFromWatchlist(ctx, wl, "bbca"); err != nil {
		t.Fatal(err)
	}

	n, err := wl.Count(ctx, domain.DefaultUser)

	if err != nil {
		t.Fatal(err)
	}

	if n != 0 {
		t.Fatalf("count = %d", n)
	}

	if err := RemoveFromWatchlist(ctx, wl, "!!"); !errors.Is(err, domain.ErrInvalidCode) {
		t.Fatalf("expected ErrInvalidCode, got %v", err)
	}
}
