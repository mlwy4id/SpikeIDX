package postgres

import (
	"context"

	"spikeidx/internal/domain"
)

type WatchlistRepo struct{ db *DB }

func NewWatchlistRepo(db *DB) *WatchlistRepo { return &WatchlistRepo{db: db} }

func (r *WatchlistRepo) List(ctx context.Context, user domain.UserID) ([]domain.Code, error) {
	rows, err := r.db.Pool.Query(ctx, `SELECT code FROM watchlist WHERE user_id=$1 ORDER BY code`, string(user))
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out := []domain.Code{}
	for rows.Next() {
		var c string
		if err := rows.Scan(&c); err != nil {
			return nil, err
		}
		out = append(out, domain.Code(c))
	}
	return out, rows.Err()
}

func (r *WatchlistRepo) Add(ctx context.Context, user domain.UserID, code domain.Code) error {
	_, err := r.db.Pool.Exec(ctx,
		`INSERT INTO watchlist (user_id, code) VALUES ($1,$2) ON CONFLICT DO NOTHING`, string(user), string(code))
	return err
}

func (r *WatchlistRepo) Remove(ctx context.Context, user domain.UserID, code domain.Code) error {
	_, err := r.db.Pool.Exec(ctx, `DELETE FROM watchlist WHERE user_id=$1 AND code=$2`, string(user), string(code))
	return err
}

func (r *WatchlistRepo) Count(ctx context.Context, user domain.UserID) (int, error) {
	var n int
	err := r.db.Pool.QueryRow(ctx, `SELECT COUNT(*) FROM watchlist WHERE user_id=$1`, string(user)).Scan(&n)
	return n, err
}
