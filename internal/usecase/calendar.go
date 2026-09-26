package usecase

import (
	"time"

	"spikeidx/internal/domain"
)

func IsTradingDay(date domain.TradingDate, holidays map[domain.TradingDate]bool) bool {
	if holidays[date] {
		return false
	}

	switch date.Time().Weekday() {
	case time.Saturday, time.Sunday:
		return false
	}

	return true
}
