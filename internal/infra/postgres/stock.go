package postgres

import (
	"context"
	"errors"

	"github.com/jackc/pgx/v5"
	"spikeidx/internal/domain"
)

type StockRepo struct{ db *DB }

func NewStockRepo(db *DB) *StockRepo { return &StockRepo{db: db} }

func (r *StockRepo) Ensure(ctx context.Context, s domain.Stock) error {
	_, err := r.db.Pool.Exec(ctx, UpsertStock, string(s.Code), s.YahooSymbol, s.Name, nullIfEmpty(s.Sector))
	return err
}

func (r *StockRepo) Get(ctx context.Context, code domain.Code) (domain.Stock, error) {
	var s domain.Stock
	var dbCode string
	var sector *string
	err := r.db.Pool.QueryRow(ctx,
		`SELECT code, yahoo_symbol, name, sector FROM stocks_master WHERE code=$1`, string(code),
	).Scan(&dbCode, &s.YahooSymbol, &s.Name, &sector)
	if errors.Is(err, pgx.ErrNoRows) {
		return domain.Stock{}, domain.ErrStockUnknown
	}
	if err != nil {
		return domain.Stock{}, err
	}
	s.Code = domain.Code(dbCode)
	if sector != nil {
		s.Sector = *sector
	}
	return s, nil
}

func nullIfEmpty(s string) any {
	if s == "" {
		return nil
	}
	return s
}
