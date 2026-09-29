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
		{Signal{CMF: 0.30, PctChange: 0.5, ADLSlope5: 1}, "akumulasi kuat sideways"},
		{Signal{CMF: 0.30, PctChange: 3.0, ADLSlope5: 1}, "akumulasi kuat markup"},
		{Signal{CMF: 0.10, PctChange: 0.5, ADLSlope5: 1}, "akumulasi lemah sideways"},
		{Signal{CMF: 0.10, PctChange: 3.0, ADLSlope5: 1}, "akumulasi lemah markup"},
		{Signal{CMF: 0.01, PctChange: 0.5, ADLSlope5: -1}, "distribusi"},
		{Signal{CMF: 0.01, PctChange: 0.5, ADLSlope5: 1}, "netral"},
	}

	for _, tc := range cases {
		if got := tc.input.Interpretation(); got != tc.expected {
			t.Errorf("Interpretation(%+v) = %q, want %q", tc.input, got, tc.expected)
		}
	}
}

func TestDefaultSpikeRule(t *testing.T) {
	r := DefaultSpikeRule()

	if r.MultipleMin != 1.5 || r.ZScoreMin != 2.0 {
		t.Fatalf("volume thresholds changed: %+v", r)
	}

	if r.CMFWeakMin != 0.05 || r.CMFStrongMin != 0.25 {
		t.Fatalf("CMF thresholds changed: %+v", r)
	}

	if r.PctChangeMin != 2.0 {
		t.Fatalf("pct regime threshold changed: %+v", r)
	}

	if r.IsPriceFilterEnabled {
		t.Fatal("price filter must be disabled by default (klasifikasi, bukan filter)")
	}
}
