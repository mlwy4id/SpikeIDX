package domain

import (
	"testing"
	"time"
)

func TestParseTradingDateRoundTrip(t *testing.T) {
	d, err := ParseTradingDate("2026-09-18")
	if err != nil {
		t.Fatal(err)
	}

	if got := d.String(); got != "2026-09-18" {
		t.Fatalf("got %q", got)
	}

	if got := d.Time().Format("15:04"); got != "00:00" {
		t.Fatalf("expected midnight, got %q", got)
	}

	if name, _ := d.Time().Zone(); name != "WIB" {
		t.Fatalf("expected WIB zone, got %q", name)
	}
}

func TestParseTradingDateRejects(t *testing.T) {
	for _, s := range []string{"", "18-09-2026", "2026/09/18", "2026-13-01", "2026-09-18T00:00:00Z"} {
		if _, err := ParseTradingDate(s); err == nil {
			t.Errorf("expected error for %q", s)
		}
	}
}

func TestNewTradingDateNormalizesToWIB(t *testing.T) {
	utc := time.Date(2026, 9, 18, 18, 0, 0, 0, time.UTC)

	if got := NewTradingDate(utc).String(); got != "2026-09-19" {
		t.Fatalf("got %q, want 2026-09-19", got)
	}

	yahoo := time.Date(2026, 9, 18, 0, 0, 0, 0, time.UTC)

	if got := NewTradingDate(yahoo).String(); got != "2026-09-18" {
		t.Fatalf("got %q, want 2026-09-18", got)
	}
}

func TestTradingDateMapKeyAcrossConstructions(t *testing.T) {
	a, err := ParseTradingDate("2026-09-22")
	if err != nil {
		t.Fatal(err)
	}

	b := NewTradingDate(time.Date(2026, 9, 22, 12, 0, 0, 0, time.UTC))

	if !a.Equal(b) {
		t.Fatalf("instants differ: %v vs %v", a, b)
	}

	if m := map[TradingDate]bool{a: true}; !m[b] {
		t.Fatal("same-day dates built separately must match as map keys (holiday lookups depend on it)")
	}
}
