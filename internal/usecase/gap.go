package usecase

import (
	"sort"

	"spikeidx/internal/domain"
)

// maxGapLookahead caps stepping over non-trading days when searching the
// next expected trading day. It covers long holidays (e.g. Lebaran). When
// exceeded we report no gap: a broken calendar must not block detection.
const maxGapLookahead = 32

// DetectGap scans hist (any order) for the first expected trading day
// missing from the series and returns it. Non-trading rows (weekends,
// holidays known via isTradingDay) never participate: only trading-day
// rows are checked for consecutiveness.
//
// NOTE: with holidays == nil only weekends are skipped, so a long public
// holiday reads as a gap and detection is skipped with an explicit reason
// (fail-closed). Fetching the BEI calendar (G-06) removes this limitation;
// Backfill stays idempotent so healed vendor holes clear automatically.
func DetectGap(hist []domain.OHLCV, isTradingDay func(domain.TradingDate) bool) (domain.TradingDate, bool) {
	rows := make([]domain.TradingDate, 0, len(hist))

	for _, o := range hist {
		if d := domain.NewTradingDate(o.Date); isTradingDay(d) {
			rows = append(rows, d)
		}
	}

	if len(rows) < 2 {
		return domain.TradingDate{}, false
	}

	sort.SliceStable(rows, func(i, j int) bool { return rows[i].Time().Before(rows[j].Time()) })

	prev := rows[0]

	for _, cur := range rows[1:] {
		if cur.Equal(prev) {
			continue
		}

		expected, ok := nextTradingDay(prev, isTradingDay)
		if !ok {
			return domain.TradingDate{}, false
		}

		if !cur.Equal(expected) {
			return expected, true
		}

		prev = cur
	}

	return domain.TradingDate{}, false
}

func nextTradingDay(d domain.TradingDate, isTradingDay func(domain.TradingDate) bool) (domain.TradingDate, bool) {
	next := d

	for range maxGapLookahead {
		next = domain.NewTradingDate(next.Time().AddDate(0, 0, 1))

		if isTradingDay(next) {
			return next, true
		}
	}

	return domain.TradingDate{}, false
}
