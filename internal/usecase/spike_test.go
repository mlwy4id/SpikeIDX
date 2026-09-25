package usecase

import (
	"fmt"
	"testing"

	"spikeidx/internal/domain"
)

func mkHist(volumes []int64, close float64) []domain.OHLCV {
	out := make([]domain.OHLCV, len(volumes))
	for i, v := range volumes {
		out[i] = domain.OHLCV{Code: "BBCA", Volume: v, Close: close, High: close, Low: close}
	}
	return out
}

func TestStatsNeeds20Rows(t *testing.T) {
	_, _, _, _, ok := Stats(mkHist(make([]int64, 19), 100), domain.DefaultSpikeRule())
	if ok {
		t.Fatal("expected not ok with <20 rows")
	}
}

func TestIsSpike(t *testing.T) {
	rule := domain.DefaultSpikeRule()
	vols := make([]int64, 20)
	for i := range vols {
		vols[i] = 10_000_000
	}
	hist := mkHist(vols, 100)
	hist[19].Volume = 35_000_000
	hist[19].Close = 103
	hist[18].Close = 100

	avg, mult, z, pct, ok := Stats(hist, rule)
	if !ok {
		t.Fatal("expected ok")
	}
	if mult < 2.0 {
		t.Fatalf("expected multiple>2, got %f (avg %f)", mult, avg)
	}
	spike, filtered := IsSpike(mult, z, pct, rule)
	if !spike || filtered {
		t.Fatalf("expected unfiltered spike, got spike=%v filtered=%v (z=%f pct=%f)", spike, filtered, z, pct)
	}

	hist[19].Close = 100.5
	_, mult, z, pct, _ = Stats(hist, rule)
	spike, filtered = IsSpike(mult, z, pct, rule)
	if !spike || !filtered {
		t.Fatalf("expected filtered spike, got spike=%v filtered=%v", spike, filtered)
	}
	_ = avg
}

func TestIsSpikeFilterDisabled(t *testing.T) {
	rule := domain.DefaultSpikeRule()
	rule.PriceFilterEnabled = false
	spike, filtered := IsSpike(3.0, 2.5, 0.1, rule)
	if !spike || filtered {
		t.Fatalf("expected actionable spike, got spike=%v filtered=%v", spike, filtered)
	}
}

func TestADLAccumulation(t *testing.T) {
	hist := []domain.OHLCV{
		{High: 110, Low: 90, Close: 108, Volume: 1000},
		{High: 110, Low: 90, Close: 108, Volume: 1000},
	}
	adl := AccumulationDistributionLine(hist)
	if len(adl) != 2 || adl[1] <= adl[0] {
		t.Fatalf("expected rising ADL, got %v", adl)
	}
	if got := AccumulationDistributionLineSlope5([]float64{1, 2, 3, 4, 5, 8}); got != 7 {
		t.Fatalf("expected slope 7, got %f", got)
	}
}

func TestNormalizeCode(t *testing.T) {
	if got := domain.NormalizeCode("bbca.jk"); got != "BBCA" {
		t.Fatalf("got %s", got)
	}
	if fmt.Sprint(domain.MaxWatchlist) != "100" {
		t.Fatal("MaxWatchlist changed?")
	}
}
