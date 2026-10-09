package idx

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"time"

	"spikeidx/internal/domain"
)

// nagerBase is the default upstream for public holidays (Indonesia).
// It covers national public holidays only: joint holidays (cuti bersama)
// and exchange-specific days are absent, so a fetched map is a subset of
// the official BEI calendar. Days missing from the map degrade to the
// previous behavior (weekend-only plus fail-closed gap detection).
const nagerBase = "https://date.nager.at"

// Calendar fetches public holidays over HTTP and satisfies
// domain.HolidayProvider. The zero value is not usable; use New.
type Calendar struct {
	baseURL string
	client  *http.Client
}

func NewCalendar() *Calendar {
	return &Calendar{baseURL: nagerBase, client: &http.Client{Timeout: 15 * time.Second}}
}

type nagerHoliday struct {
	Date string `json:"date"`
}

// Holidays returns Indonesian public holidays for the given year as a set
// of trading dates. Rows with unparsable dates are skipped.
func (c *Calendar) Holidays(ctx context.Context, year int) (map[domain.TradingDate]bool, error) {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet,
		fmt.Sprintf("%s/api/v3/PublicHolidays/%d/ID", c.baseURL, year), nil)
	if err != nil {
		return nil, err
	}

	resp, err := c.client.Do(req)
	if err != nil {
		return nil, err
	}
	defer func() { _ = resp.Body.Close() }()

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("idx: holidays http %d", resp.StatusCode)
	}

	var rows []nagerHoliday
	if err := json.NewDecoder(resp.Body).Decode(&rows); err != nil {
		return nil, err
	}

	out := make(map[domain.TradingDate]bool, len(rows))
	for _, row := range rows {
		date, err := domain.ParseTradingDate(row.Date)
		if err != nil {
			continue
		}
		out[date] = true
	}

	return out, nil
}
