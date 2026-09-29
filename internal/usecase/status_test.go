package usecase

import (
	"context"
	"testing"

	"spikeidx/internal/domain"
)

// RED: non-spike harus tetap mengembalikan Signal status (accum/dist)
// agar worker bisa print keadaan terkini, bukan Signal kosong.
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

// Non-spike dari DailyIngest harus membawa status (HasData+Signal)
// agar worker bisa print accum/dist, dan tidak boleh di-Upsert sebagai sinyal.
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
	if r.HasSpike {
		t.Fatalf("expected no spike, got %+v", r)
	}
	if !r.HasData {
		t.Fatalf("expected HasData for non-spike with history, got %+v", r)
	}
	if r.Signal.Volume == 0 || r.Signal.Interpretation() == "" {
		t.Fatalf("expected status signal, got %+v", r)
	}
	if len(signals.data) != 0 {
		t.Fatalf("non-spike must not upsert signals, got %+v", signals.data)
	}
}
