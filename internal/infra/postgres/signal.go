package postgres

import (
	"context"

	"spikeidx/internal/domain"
)

type SignalRepo struct{ db *DB }

func NewSignalRepo(db *DB) *SignalRepo { return &SignalRepo{db: db} }

func (r *SignalRepo) Upsert(ctx context.Context, s domain.Signal) error {
	_, err := r.db.Pool.Exec(ctx, `
INSERT INTO signals (code, date, volume, avg20, multiple, zscore, close, pct_change, adl, adl_slope5, cmf, is_filtered)
VALUES ($1,$2,$3,$4,$5,$6,$7,$8,$9,$10,$11,$12)
ON CONFLICT (code, date) DO UPDATE SET
  volume=EXCLUDED.volume, avg20=EXCLUDED.avg20, multiple=EXCLUDED.multiple,
  zscore=EXCLUDED.zscore, close=EXCLUDED.close, pct_change=EXCLUDED.pct_change,
  adl=EXCLUDED.adl, adl_slope5=EXCLUDED.adl_slope5, cmf=EXCLUDED.cmf, is_filtered=EXCLUDED.is_filtered`,
		string(s.Code), s.Date, s.Volume, s.Avg20, s.Multiple, s.ZScore,
		s.Close, s.PctChange, s.ADL, s.ADLSlope5, s.CMF, s.IsFiltered)
	return err
}

func (r *SignalRepo) ByDate(ctx context.Context, date domain.TradingDate, shouldIncludeFiltered bool) ([]domain.Signal, error) {
	q := `SELECT code, date, volume, avg20, multiple, zscore, close, pct_change, adl, adl_slope5, cmf, is_filtered
	      FROM signals WHERE date=$1::date`
	if !shouldIncludeFiltered {
		q += ` AND is_filtered=false`
	}
	q += ` ORDER BY multiple DESC`
	rows, err := r.db.Pool.Query(ctx, q, date.String())
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var out []domain.Signal
	for rows.Next() {
		var s domain.Signal
		var dbCode string
		if err := rows.Scan(&dbCode, &s.Date, &s.Volume, &s.Avg20, &s.Multiple, &s.ZScore,
			&s.Close, &s.PctChange, &s.ADL, &s.ADLSlope5, &s.CMF, &s.IsFiltered); err != nil {
			return nil, err
		}
		s.Code = domain.Code(dbCode)
		out = append(out, s)
	}
	return out, rows.Err()
}
