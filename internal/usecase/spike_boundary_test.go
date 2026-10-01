package usecase

import (
	"testing"
	"time"

	"spikeidx/internal/domain"
)

func TestBoundaries(t *testing.T) {
	rule := domain.DefaultSpikeRule()
	cases := []struct {
		name             string
		inputMultiple    float64
		inputZScore      float64
		inputPctChange   float64
		expectedSpike    bool
		expectedFiltered bool
	}{
		{"multiple pas 2.0 bukan spike", 2.0, 2.5, 5.0, false, false},
		{"multiple 2.1 spike", 2.1, 2.5, 5.0, true, false},
		{"multiple 2.5 spike terfilter pct kecil", 2.5, 2.5, 0.5, true, true},
		{"z pas 2.0 bukan spike", 2.1, 2.0, 5.0, false, false},
		{"pct pas 2.0 actionable", 2.5, 2.5, 2.0, true, false},
		{"pct pas -2.0 actionable", 2.5, 2.5, -2.0, true, false},
		{"pct sideways terfilter (bukan klasifikasi)", 2.5, 2.5, 1.9, true, true},
		{"pct sideways 0.5 terfilter", 2.5, 2.5, 0.5, true, true},
		{"semua di atas batas", 2.1, 2.1, 2.1, true, false},
	}

	for _, tc := range cases {
		spike, isFiltered := IsSpike(tc.inputMultiple, tc.inputZScore, tc.inputPctChange, rule)
		if spike != tc.expectedSpike || isFiltered != tc.expectedFiltered {
			t.Errorf("%s: got spike=%v filtered=%v", tc.name, spike, isFiltered)
		}
	}
}

func TestStats21Rows(t *testing.T) {
	vols := make([]int64, 21)

	for i := range vols {
		vols[i] = 10_000_000
	}

	if _, _, _, _, ok := Stats(mkHist(vols, 100), domain.DefaultSpikeRule()); !ok {
		t.Fatal("expected ok with 21 rows")
	}
}

func TestStatsZeroGuards(t *testing.T) {
	vols := make([]int64, 20)
	_, multiple, zScore, pctChange, ok := Stats(mkHist(vols, 0), domain.DefaultSpikeRule())

	if !ok {
		t.Fatal("expected ok")
	}

	if multiple != 0 || zScore != 0 || pctChange != 0 {
		t.Fatalf("expected zeros, got mult=%v z=%v pct=%v", multiple, zScore, pctChange)
	}

	if spike, _ := IsSpike(multiple, zScore, pctChange, domain.DefaultSpikeRule()); spike {
		t.Fatal("zero stats must not spike")
	}
}

func TestStatsUnsortedHistory(t *testing.T) {
	base := time.Date(2026, 6, 1, 0, 0, 0, 0, time.UTC)
	hist := make([]domain.OHLCV, 20)

	for i := range hist {
		hist[i] = domain.OHLCV{
			Code: "BBCA", Date: base.AddDate(0, 0, i),
			High: 100, Low: 100, Close: 100, Volume: 10_000_000,
		}
	}

	hist[19].Volume = 35_000_000
	hist[19].Close = 103
	shuffled := append([]domain.OHLCV{hist[19]}, hist[:19]...)

	sig, spike, isFiltered, err := DetectOne("BBCA", shuffled, domain.DefaultSpikeRule())
	if err != nil {
		t.Fatal(err)
	}

	if !spike || isFiltered {
		t.Fatalf("got spike=%v filtered=%v", spike, isFiltered)
	}

	if !sig.Date.Equal(base.AddDate(0, 0, 19)) || sig.Volume != 35_000_000 {
		t.Fatalf("signal bar wrong: %+v", sig)
	}
}
