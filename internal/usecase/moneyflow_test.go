package usecase

import (
	"testing"

	"spikeidx/internal/domain"
)

func TestMoneyFlowMultiplier(t *testing.T) {
	cases := []struct {
		name     string
		bar      domain.OHLCV
		expected float64
	}{
		{"close near high", domain.OHLCV{High: 110, Low: 90, Close: 108}, 0.8},
		{"close near low", domain.OHLCV{High: 110, Low: 90, Close: 92}, -0.8},
		{"close at midpoint", domain.OHLCV{High: 110, Low: 90, Close: 100}, 0},
		{"flat bar guard", domain.OHLCV{High: 100, Low: 100, Close: 100}, 0},
	}

	for _, tc := range cases {
		if got := moneyFlowMultiplier(tc.bar); got != tc.expected {
			t.Errorf("%s: moneyFlowMultiplier(%+v) = %f, want %f", tc.name, tc.bar, got, tc.expected)
		}
	}
}
