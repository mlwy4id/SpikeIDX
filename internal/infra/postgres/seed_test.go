package postgres

import (
	"context"
	"errors"
	"testing"

	"spikeidx/internal/domain"
)

type seedFakeStocks struct {
	data   map[domain.Code]domain.Stock
	getErr error
}

func (f *seedFakeStocks) Ensure(_ context.Context, s domain.Stock) error {
	if f.data == nil {
		f.data = map[domain.Code]domain.Stock{}
	}
	f.data[s.Code] = s
	return nil
}

func (f *seedFakeStocks) Get(_ context.Context, code domain.Code) (domain.Stock, error) {
	if f.getErr != nil {
		return domain.Stock{}, f.getErr
	}
	if s, ok := f.data[code]; ok {
		return s, nil
	}
	return domain.Stock{}, domain.ErrStockUnknown
}

func seedFixture() []domain.Stock {
	return []domain.Stock{
		{Code: "BBCA", YahooSymbol: "BBCA.JK", Name: "Bank Central Asia"},
		{Code: "TLKM", YahooSymbol: "TLKM.JK", Name: "Telkom Indonesia"},
	}
}

func TestSeedStocksIdempotent(t *testing.T) {
	ctx := context.Background()
	repo := &seedFakeStocks{}

	first, err := SeedStocks(ctx, repo, seedFixture())
	if err != nil {
		t.Fatalf("first seed: %v", err)
	}
	if first != 2 {
		t.Fatalf("first inserted = %d, want 2", first)
	}

	second, err := SeedStocks(ctx, repo, seedFixture())
	if err != nil {
		t.Fatalf("second seed: %v", err)
	}
	if second != 0 {
		t.Fatalf("second inserted = %d, want 0", second)
	}
}

func TestSeedStocksInvalidCode(t *testing.T) {
	ctx := context.Background()
	repo := &seedFakeStocks{}

	stocks := append(seedFixture(), domain.Stock{Code: "BB CA", YahooSymbol: "BB CA.JK", Name: "Bad"})
	inserted, err := SeedStocks(ctx, repo, stocks)
	if !errors.Is(err, domain.ErrInvalidCode) {
		t.Fatalf("err = %v, want ErrInvalidCode", err)
	}
	if inserted != 0 {
		t.Fatalf("inserted = %d, want 0", inserted)
	}
	if len(repo.data) != 0 {
		t.Fatalf("repo touched on invalid input: %+v", repo.data)
	}
}

func TestSeedStocksGetErrorAborts(t *testing.T) {
	ctx := context.Background()
	boom := errors.New("boom")
	repo := &seedFakeStocks{getErr: boom}

	inserted, err := SeedStocks(ctx, repo, seedFixture())
	if err == nil {
		t.Fatal("expected error, got nil")
	}
	if !errors.Is(err, boom) {
		t.Fatalf("err = %v, want wrapped boom", err)
	}
	if errors.Is(err, domain.ErrInvalidCode) {
		t.Fatalf("err = %v, must not be ErrInvalidCode", err)
	}
	if inserted != 0 {
		t.Fatalf("inserted = %d, want 0", inserted)
	}
}

func TestSeedStocksContract(t *testing.T) {
	db := testDB(t)
	ctx := context.Background()
	repo := NewStockRepo(db)

	for _, q := range []string{
		`DELETE FROM stocks_master WHERE code IN ('ZSX','ZSY')`,
	} {
		if _, err := db.Pool.Exec(ctx, q); err != nil {
			t.Fatalf("cleanup: %v", err)
		}
	}

	stocks := []domain.Stock{
		{Code: "ZSX", YahooSymbol: "ZSX.JK", Name: "Seed Fixture X"},
		{Code: "ZSY", YahooSymbol: "ZSY.JK", Name: "Seed Fixture Y"},
	}

	first, err := SeedStocks(ctx, repo, stocks)
	if err != nil {
		t.Fatal(err)
	}
	if first != 2 {
		t.Fatalf("first inserted = %d, want 2", first)
	}

	second, err := SeedStocks(ctx, repo, stocks)
	if err != nil {
		t.Fatal(err)
	}
	if second != 0 {
		t.Fatalf("second inserted = %d, want 0", second)
	}

	got, err := repo.Get(ctx, "ZSX")
	if err != nil || got.Name != "Seed Fixture X" {
		t.Fatalf("got %+v, %v", got, err)
	}
}
