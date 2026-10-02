package usecase

import (
	"context"
	"testing"
	"time"

	"spikeidx/internal/domain"
)

// Sep 2026 layout: 21 Mon, 22 Tue, 23 Wed, 24 Thu, 25 Fri, 28 Mon.
// Noon UTC == evening WIB on the same calendar date.
func gapRow(day int) domain.OHLCV {
	return domain.OHLCV{Code: "GAPX", Date: time.Date(2026, 9, day, 12, 0, 0, 0, time.UTC)}
}

func weekendOnly(d domain.TradingDate) bool { return IsTradingDay(d, nil) }

func mustNoGap(t *testing.T, hist []domain.OHLCV, isTradingDay func(domain.TradingDate) bool) {
	t.Helper()

	if missing, gapped := DetectGap(hist, isTradingDay); gapped {
		t.Fatalf("unexpected gap at %s", missing)
	}
}

func TestDetectGapCleanWeek(t *testing.T) {
	mustNoGap(t, []domain.OHLCV{gapRow(21), gapRow(22), gapRow(23), gapRow(24), gapRow(25)}, weekendOnly)
}

func TestDetectGapSkipsWeekend(t *testing.T) {
	mustNoGap(t, []domain.OHLCV{gapRow(25), gapRow(28)}, weekendOnly)
}

func TestDetectGapMissingTuesday(t *testing.T) {
	missing, gapped := DetectGap([]domain.OHLCV{gapRow(21), gapRow(23), gapRow(24)}, weekendOnly)

	if !gapped || missing.String() != "2026-09-22" {
		t.Fatalf("got gap=%v missing=%s", gapped, missing)
	}
}

func TestDetectGapHonorsHolidays(t *testing.T) {
	hist := []domain.OHLCV{gapRow(21), gapRow(23)}
	holidays := map[domain.TradingDate]bool{mustTradingDate(t, "2026-09-22"): true}

	mustNoGap(t, hist, func(d domain.TradingDate) bool { return IsTradingDay(d, holidays) })

	if missing, gapped := DetectGap(hist, weekendOnly); !gapped || missing.String() != "2026-09-22" {
		t.Fatalf("got gap=%v missing=%s", gapped, missing)
	}
}

func TestDetectGapTooShort(t *testing.T) {
	mustNoGap(t, nil, weekendOnly)
	mustNoGap(t, []domain.OHLCV{gapRow(21)}, weekendOnly)
}

func TestDetectGapUnsorted(t *testing.T) {
	mustNoGap(t, []domain.OHLCV{gapRow(24), gapRow(21), gapRow(23), gapRow(22), gapRow(25)}, weekendOnly)

	if missing, gapped := DetectGap([]domain.OHLCV{gapRow(24), gapRow(21), gapRow(23)}, weekendOnly); !gapped || missing.String() != "2026-09-22" {
		t.Fatalf("got gap=%v missing=%s", gapped, missing)
	}
}

func TestDetectGapStuckCalendar(t *testing.T) {
	mustNoGap(t, []domain.OHLCV{gapRow(21), gapRow(23)}, func(domain.TradingDate) bool { return false })
}

func TestDetectGapDuplicateRow(t *testing.T) {
	mustNoGap(t, []domain.OHLCV{gapRow(21), gapRow(21), gapRow(22), gapRow(23)}, weekendOnly)
}

func mustTradingDate(t *testing.T, s string) domain.TradingDate {
	t.Helper()

	d, err := domain.ParseTradingDate(s)
	if err != nil {
		t.Fatal(err)
	}

	return d
}

func TestDailyIngestGap(t *testing.T) {
	ctx := context.Background()
	stocks := &fakeStocks{}
	ohlcv := &fakeOHLCV{data: map[domain.Code][]domain.OHLCV{
		"GAPX": {gapRow(21), gapRow(23)},
	}}
	signals := &fakeSignals{}
	provider := &fakeProvider{}

	res := DailyIngest(ctx, provider, IngestRepos{Stocks: stocks, OHLCV: ohlcv, Signals: signals},
		[]domain.Code{"GAPX"}, domain.DefaultSpikeRule(), nil)

	if len(res) != 1 {
		t.Fatalf("got %+v", res)
	}

	if res[0].Status != StatusSkipped || res[0].Reason != "gap:2026-09-22 missing" {
		t.Fatalf("got %+v", res[0])
	}

	if len(signals.data) != 0 {
		t.Fatalf("signals: %+v", signals.data)
	}
}
