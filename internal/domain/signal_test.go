package domain

import "testing"

func TestSignalIsActionable(t *testing.T) {
	if !(Signal{}.IsActionable()) {
		t.Fatal("zero signal should be actionable")
	}
	
	if (Signal{IsFiltered: true}).IsActionable() {
		t.Fatal("filtered signal should not be actionable")
	}
}

func TestSignalInterpretation(t *testing.T) {
	cases := []struct {
		input    Signal
		expected string
	}{
		{Signal{ADLSlope5: 1.5}, "akumulasi"},
		{Signal{ADLSlope5: -0.1}, "distribusi"},
		{Signal{ADLSlope5: 0}, "netral"},
		{Signal{ADLSlope5: 99, IsFiltered: true}, "terfilter (noise harga)"},
	}

	for _, tc := range cases {
		if got := tc.input.Interpretation(); got != tc.expected {
			t.Errorf("Interpretation(%+v) = %q, want %q", tc.input, got, tc.expected)
		}
	}
}

func TestDefaultSpikeRule(t *testing.T) {
	r := DefaultSpikeRule()

	if r.MultipleMin != 2.0 || r.ZScoreMin != 2.0 || r.PctChangeMin != 2.0 {
		t.Fatalf("thresholds changed: %+v", r)
	}

	if !r.IsPriceFilterEnabled {
		t.Fatal("price filter must be enabled by default (US-06)")
	}
}
