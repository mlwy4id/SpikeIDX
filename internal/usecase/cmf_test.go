package usecase

import (
	"testing"

	"spikeidx/internal/domain"
)

func mkCMFHist(n int, high, low, close float64, vol int64) []domain.OHLCV {
	out := make([]domain.OHLCV, n)
	for i := range out {
		out[i] = domain.OHLCV{High: high, Low: low, Close: close, Volume: vol}
	}
	return out
}

func TestCMFStrongAccumulation(t *testing.T) {
	hist := mkCMFHist(20, 110, 90, 108, 1000)
	got := CMF(hist)
	if got <= 0.25 {
		t.Fatalf("expected strong accumulation CMF>0.25, got %f", got)
	}
}

func TestCMFWeakAccumulation(t *testing.T) {
	// MFM = ((102-90)-(110-102))/20 = 0.2 -> CMF 0.2 (lemah: 0.05-0.25)
	hist := mkCMFHist(20, 110, 90, 102, 1000)
	got := CMF(hist)
	if got <= 0.05 || got > 0.25 {
		t.Fatalf("expected weak accumulation 0.05<CMF<=0.25, got %f", got)
	}
}

func TestCMFDistribution(t *testing.T) {
	hist := mkCMFHist(20, 110, 90, 92, 1000)
	got := CMF(hist)
	if got >= 0 {
		t.Fatalf("expected negative CMF for distribution, got %f", got)
	}
}

func TestCMFNeeds20Rows(t *testing.T) {
	if got := CMF(mkCMFHist(19, 110, 90, 108, 1000)); got != 0 {
		t.Fatalf("expected 0 with <20 rows, got %f", got)
	}
}

func TestCMFFlatBarGuard(t *testing.T) {
	hist := mkCMFHist(20, 100, 100, 100, 1000)
	if got := CMF(hist); got != 0 {
		t.Fatalf("expected 0 for High==Low, got %f", got)
	}
}

func TestSidewaysSpikeIsActionable(t *testing.T) {
	rule := domain.DefaultSpikeRule()
	spike, isFiltered := IsSpike(2.5, 2.5, 0.5, rule)
	if !spike || isFiltered {
		t.Fatalf("sideways volume spike must be actionable (klasifikasi, bukan filter): got spike=%v filtered=%v", spike, isFiltered)
	}
}
