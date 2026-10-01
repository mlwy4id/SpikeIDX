package telegram

import (
	"strings"
	"testing"

	"spikeidx/internal/domain"
)

func TestDigestEmpty(t *testing.T) {
	if got := Digest("18 Sep", nil); !strings.Contains(got, "No spike today") {
		t.Fatalf("got %q", got)
	}
}

func TestDigestFormat(t *testing.T) {
	signals := []domain.Signal{
		{Code: "BBCA", Volume: 45_000_000, Multiple: 3.1, PctChange: 2.4, Avg20: 10_000_000, ADLSlope5: 40_000_000},
		{Code: "TLKM", Volume: 80_500_000, Multiple: 2.5, PctChange: -1.2, Avg20: 10_000_000, ADLSlope5: -40_000_000},
	}
	got := Digest("17 Sep", signals)
	for _, want := range []string{"BBCA 45jt (3.1x avg20) +2.4%", "TLKM 80.5jt (2.5x avg20) -1.2%", "ADL akumulasi", "ADL distribusi"} {
		if !strings.Contains(got, want) {
			t.Errorf("missing %q in %q", want, got)
		}
	}
}

func TestNotifierDisabled(t *testing.T) {
	if New("", "").IsEnabled() {
		t.Fatal("expected disabled")
	}
	if New("tok", "chat").IsEnabled() != true {
		t.Fatal("expected enabled")
	}
	if err := New("", "").Send(t.Context(), "hi"); err == nil {
		t.Fatal("expected error when disabled")
	}
}
