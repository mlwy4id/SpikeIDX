package postgres

import (
	"context"

	"github.com/jackc/pgx/v5"
	"spikeidx/internal/domain"
)

type OHLCVRepo struct{ db *DB }

func NewOHLCVRepo(db *DB) *OHLCVRepo { return &OHLCVRepo{db: db} }

func (r *OHLCVRepo) UpsertBatch(ctx context.Context, rows []domain.OHLCV) error {
	if len(rows) == 0 {
		return nil
	}
	batch := &pgx.Batch{}
	for _, o := range rows {
		batch.Queue(UpsertOHLCV, string(o.Code), o.Date, o.Open, o.High, o.Low, o.Close, o.Volume)
	}
	br := r.db.Pool.SendBatch(ctx, batch)
	defer br.Close()
	for range rows {
		if _, err := br.Exec(); err != nil {
			return err
		}
	}
	return nil
}

func (r *OHLCVRepo) History(ctx context.Context, code domain.Code, limit int) ([]domain.OHLCV, error) {
	q := `SELECT code, date, open, high, low, close, volume FROM ohlcv WHERE code=$1 ORDER BY date DESC`
	var rows pgx.Rows
	var err error
	if limit > 0 {
		rows, err = r.db.Pool.Query(ctx, q+` LIMIT $2`, string(code), limit)
	} else {
		rows, err = r.db.Pool.Query(ctx, q, string(code))
	}
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var out []domain.OHLCV
	for rows.Next() {
		var o domain.OHLCV
		var dbCode string
		if err := rows.Scan(&dbCode, &o.Date, &o.Open, &o.High, &o.Low, &o.Close, &o.Volume); err != nil {
			return nil, err
		}
		o.Code = domain.Code(dbCode)
		out = append(out, o)
	}
	if err := rows.Err(); err != nil {
		return nil, err
	}
	for i, j := 0, len(out)-1; i < j; i, j = i+1, j-1 {
		out[i], out[j] = out[j], out[i]
	}
	return out, nil
}
