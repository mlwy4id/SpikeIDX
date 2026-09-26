package postgres

import (
	"context"
	"errors"
	"os"
	"testing"
	"time"

	"spikeidx/internal/domain"
)

func testDB(t *testing.T) *DB {
	t.Helper()
	if os.Getenv("DATABASE_URL") == "" {
		t.Skip("DATABASE_URL not set")
	}
	ctx := context.Background()
	db, err := Connect(ctx, os.Getenv("DATABASE_URL"))
	if err != nil {
		t.Skipf("postgres unreachable: %v", err)
	}
	t.Cleanup(db.Close)
	if err := db.Migrate(ctx); err != nil {
		t.Fatalf("migrate: %v", err)
	}
	return db
}

func TestContract(t *testing.T) {
	db := testDB(t)
	ctx := context.Background()
	stocks := NewStockRepo(db)
	wl := NewWatchlistRepo(db)
	ohlcv := NewOHLCVRepo(db)
	signals := NewSignalRepo(db)

	const code = domain.Code("QTC")
	_, _ = db.Pool.Exec(ctx, `DELETE FROM signals WHERE code='QTC'`)
	_, _ = db.Pool.Exec(ctx, `DELETE FROM ohlcv WHERE code='QTC'`)
	_, _ = db.Pool.Exec(ctx, `DELETE FROM watchlist WHERE code='QTC'`)
	_, _ = db.Pool.Exec(ctx, `DELETE FROM stocks_master WHERE code='QTC'`)

	if _, err := stocks.Get(ctx, code); !errors.Is(err, domain.ErrStockUnknown) {
		t.Fatalf("expected ErrStockUnknown, got %v", err)
	}
	if err := stocks.Ensure(ctx, domain.Stock{Code: code, YahooSymbol: "QTC.JK", Name: "Contract Fixture"}); err != nil {
		t.Fatal(err)
	}
	got, err := stocks.Get(ctx, code)
	if err != nil || got.YahooSymbol != "QTC.JK" {
		t.Fatalf("got %+v, %v", got, err)
	}

	if err := wl.Add(ctx, domain.DefaultUser, code); err != nil {
		t.Fatal(err)
	}
	if err := wl.Add(ctx, domain.DefaultUser, code); err != nil {
		t.Fatal(err)
	}
	if n, _ := wl.Count(ctx, domain.DefaultUser); n != 1 {
		t.Fatalf("count = %d", n)
	}
	list, err := wl.List(ctx, domain.DefaultUser)
	if err != nil || len(list) != 1 || list[0] != code {
		t.Fatalf("got %+v, %v", list, err)
	}

	day := func(d int) time.Time { return time.Date(2026, 6, d, 0, 0, 0, 0, time.UTC) }
	rows := []domain.OHLCV{
		{Code: code, Date: day(2), Close: 102, Volume: 2000},
		{Code: code, Date: day(1), Close: 100, Volume: 1000},
		{Code: code, Date: day(1), Close: 101, Volume: 1500},
	}
	if err := ohlcv.UpsertBatch(ctx, rows); err != nil {
		t.Fatal(err)
	}
	hist, err := ohlcv.History(ctx, code, 0)
	if err != nil || len(hist) != 2 {
		t.Fatalf("got %+v, %v", hist, err)
	}
	if !hist[0].Date.Before(hist[1].Date) || hist[0].Close != 101 || hist[0].Volume != 1500 {
		t.Fatalf("oldest-first + overwrite failed: %+v", hist)
	}
	limited, err := ohlcv.History(ctx, code, 1)
	if err != nil || len(limited) != 1 || limited[0].Close != 102 {
		t.Fatalf("got %+v, %v", limited, err)
	}

	sigDate := domain.NewTradingDate(day(2))
	sig := domain.Signal{Code: code, Date: sigDate.Time(), Volume: 2000, Avg20: 1000, Multiple: 2.5, ZScore: 2.5, Close: 102, PctChange: 1.0, IsFiltered: true}
	if err := signals.Upsert(ctx, sig); err != nil {
		t.Fatal(err)
	}
	sig.PctChange = 3.0
	sig.IsFiltered = false
	if err := signals.Upsert(ctx, sig); err != nil {
		t.Fatal(err)
	}
	res, err := signals.ByDate(ctx, sigDate, false)
	if err != nil || len(res) != 1 || res[0].PctChange != 3.0 || res[0].Code != code {
		t.Fatalf("got %+v, %v", res, err)
	}
	sig.IsFiltered = true
	if err := signals.Upsert(ctx, sig); err != nil {
		t.Fatal(err)
	}
	if res, _ := signals.ByDate(ctx, sigDate, false); len(res) != 0 {
		t.Fatalf("filtered leak: %+v", res)
	}
	if res, _ := signals.ByDate(ctx, sigDate, true); len(res) != 1 {
		t.Fatalf("got %+v", res)
	}

	if err := wl.Remove(ctx, domain.DefaultUser, code); err != nil {
		t.Fatal(err)
	}
	if n, _ := wl.Count(ctx, domain.DefaultUser); n != 0 {
		t.Fatalf("count = %d", n)
	}
}
