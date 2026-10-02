package usecase

import (
	"context"
	"testing"

	"spikeidx/internal/domain"
)

// Non-spike must still return a status Signal (accum/dist)
// so the worker can print the current state instead of an empty Signal.
func TestDetectOneNoSpikeReturnsStatus(t *testing.T) {
	vols := make([]int64, 20)
	for i := range vols {
		vols[i] = 10_000_000
	}

	sig, spike, _, err := DetectOne("BBCA", mkHist(vols, 100), domain.DefaultSpikeRule())
	if err != nil {
		t.Fatal(err)
	}
	if spike {
		t.Fatal("expected no spike")
	}
	if sig.Volume == 0 || sig.Avg20 == 0 {
		t.Fatalf("expected status signal for non-spike, got %+v", sig)
	}
	if sig.Interpretation() == "" {
		t.Fatalf("expected interpretation for non-spike, got %+v", sig)
	}
}

// Non-spike from DailyIngest must carry status (NoSpike+Signal)
// so the worker can print accum/dist, and must not Upsert it as a signal.
func TestDailyIngestNoSpikeHasStatus(t *testing.T) {
	ctx := context.Background()
	stocks := &fakeStocks{}
	ohlcv := &fakeOHLCV{}
	signals := &fakeSignals{}
	provider := &fakeProvider{
		candles: map[domain.Code][]domain.Candle{
			"TLKM": mkCandles(60, 10_000_000, 10_000_000, 100, 100),
		},
	}

	res := DailyIngest(ctx, provider, IngestRepos{Stocks: stocks, OHLCV: ohlcv, Signals: signals},
		[]domain.Code{"TLKM"}, domain.DefaultSpikeRule(), nil)
	if len(res) != 1 {
		t.Fatalf("got %+v", res)
	}
	r := res[0]
	if r.Status != StatusNoSpike {
		t.Fatalf("expected NoSpike, got %+v", r)
	}
	if r.Signal.Volume == 0 || r.Signal.Interpretation() == "" {
		t.Fatalf("expected status signal, got %+v", r)
	}
	if len(signals.data) != 0 {
		t.Fatalf("non-spike must not upsert signals, got %+v", signals.data)
	}
}

func TestShouldPersist(t *testing.T) {
	cases := []struct {
		status IngestStatus
		want   bool
	}{
		{StatusSpikeActionable, true},
		{StatusSpikeFiltered, true},
		{StatusNoSpike, false},
		{StatusSkipped, false},
	}

	for _, tc := range cases {
		if got := (SymbolResult{Status: tc.status}).ShouldPersist(); got != tc.want {
			t.Errorf("ShouldPersist(%v) = %v, want %v", tc.status, got, tc.want)
		}
	}
}
