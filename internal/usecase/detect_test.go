package usecase

import (
	"context"
	"errors"
	"testing"
	"time"

	"spikeidx/internal/domain"
)

func mkCandles(n int, base, last int64, firstClose, lastClose float64) []domain.Candle {
	day := time.Date(2026, 6, 1, 0, 0, 0, 0, time.UTC)
	out := make([]domain.Candle, n)

	for i := range out {
		out[i] = domain.Candle{
			Date: day.AddDate(0, 0, i),
			Open: 100, High: 100, Low: 100, Close: firstClose, Volume: base,
		}
	}
	
	out[n-1].Volume = last
	out[n-1].Close = lastClose
	
	return out
}

func TestDetectOneSpike(t *testing.T) {
	vols := make([]int64, 20)

	for i := range vols {
		vols[i] = 10_000_000
	}
	
	hist := mkHist(vols, 100)
	hist[19].Volume = 35_000_000
	hist[19].Close = 103
	hist[18].Close = 100

	sig, spike, isFiltered, err := DetectOne("BBCA", hist, domain.DefaultSpikeRule())

	if err != nil {
		t.Fatal(err)
	}
	
	if !spike || isFiltered {
		t.Fatalf("got spike=%v filtered=%v", spike, isFiltered)
	}
	
	if sig.Code != "BBCA" || sig.Volume != 35_000_000 || sig.Multiple < 2.0 {
		t.Fatalf("got %+v", sig)
	}
	
	if !sig.IsActionable() || sig.Interpretation() == "" {
		t.Fatalf("got %+v", sig)
	}
}

func TestDetectOneNoSpike(t *testing.T) {
	vols := make([]int64, 20)

	for i := range vols {
		vols[i] = 10_000_000
	}
	
	_, spike, _, err := DetectOne("BBCA", mkHist(vols, 100), domain.DefaultSpikeRule())

	if err != nil || spike {
		t.Fatalf("got spike=%v err=%v", spike, err)
	}
}

func TestDetectOneShortHistory(t *testing.T) {
	_, _, _, err := DetectOne("BBCA", mkHist(make([]int64, 19), 100), domain.DefaultSpikeRule())

	if !errors.Is(err, domain.ErrInsufficientData) {
		t.Fatalf("expected ErrInsufficientData, got %v", err)
	}
}

func TestDailyIngest(t *testing.T) {
	ctx := context.Background()
	stocks := &fakeStocks{}
	ohlcv := &fakeOHLCV{}
	signals := &fakeSignals{}
	provider := &fakeProvider{
		candles: map[domain.Code][]domain.Candle{
			"BBCA": mkCandles(60, 10_000_000, 35_000_000, 100, 103),
			"TLKM": mkCandles(60, 10_000_000, 10_000_000, 100, 100),
		},
		codeErrs: map[domain.Code]error{"GOTO": errors.New("yahoo: HTTP 429")},
	}

	res := DailyIngest(ctx, provider, IngestRepos{Stocks: stocks, OHLCV: ohlcv, Signals: signals},
		[]domain.Code{"BBCA", "TLKM", "GOTO"}, domain.DefaultSpikeRule())
	
	if len(res) != 3 {
		t.Fatalf("got %+v", res)
	}
	
	if !res[0].HasSpike || res[0].Signal.Code != "BBCA" {
		t.Fatalf("BBCA: %+v", res[0])
	}
	
	if res[1].HasSpike || res[1].Reason != "no spike" {
		t.Fatalf("TLKM: %+v", res[1])
	}

	if res[2].HasSpike || res[2].Reason == "" {
		t.Fatalf("GOTO: %+v", res[2])
	}

	if len(signals.data) != 1 || signals.data[0].Code != "BBCA" {
		t.Fatalf("signals: %+v", signals.data)
	}

	if _, err := stocks.Get(ctx, "BBCA"); err != nil {
		t.Fatalf("expected ensured stock, got %v", err)
	}
}

func TestIsTradingDay(t *testing.T) {
	sat, err := domain.ParseTradingDate("2026-09-19")

	if err != nil {
		t.Fatal(err)
	}

	sun, err := domain.ParseTradingDate("2026-09-20")

	if err != nil {
		t.Fatal(err)
	}

	mon, err := domain.ParseTradingDate("2026-09-21")

	if err != nil {
		t.Fatal(err)
	}

	if IsTradingDay(sat, nil) || IsTradingDay(sun, nil) {
		t.Fatal("weekend must not trade")
	}

	if !IsTradingDay(mon, nil) {
		t.Fatal("monday must trade")
	}

	holidays := map[domain.TradingDate]bool{mon: true}

	if IsTradingDay(mon, holidays) {
		t.Fatal("holiday must not trade")
	}
}
