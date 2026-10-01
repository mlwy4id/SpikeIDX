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
	avg := 10_000_000.0
	cases := []struct {
		input    Signal
		expected string
	}{
		{Signal{ADLSlope5: 40_000_000, Avg20: avg}, "akumulasi"},
		{Signal{ADLSlope5: -40_000_000, Avg20: avg}, "distribusi"},
		{Signal{ADLSlope5: 5_000_000, Avg20: avg}, "netral"},
		{Signal{ADLSlope5: 0, Avg20: avg}, "netral"},
		{Signal{ADLSlope5: 1.5, Avg20: 0}, "netral"},
		{Signal{ADLSlope5: 40_000_000, Avg20: avg, IsFiltered: true}, "terfilter (noise harga)"},
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

	if r.CMFWeakMin != 0.05 || r.CMFStrongMin != 0.25 {
		t.Fatalf("CMF thresholds changed: %+v", r)
	}

	if r.ADLSlopeWindow != 20 {
		t.Fatalf("ADLSlopeWindow default must be 20 (CMF standard), got %d", r.ADLSlopeWindow)
	}

	if r.ADLSlopeMinRatio != 0.10 {
		t.Fatalf("ADLSlopeMinRatio default must be 0.10 (CMF standard), got %f", r.ADLSlopeMinRatio)
	}
}

func TestSignalInterpretationWithRule(t *testing.T) {
	rule := DefaultSpikeRule()
	avg := 10_000_000.0
	window := float64(rule.ADLSlopeWindow)

	strongBuy := 0.20 * avg * window
	strongSell := -0.20 * avg * window
	noise := 0.03 * avg * window

	cases := []struct {
		name     string
		input    Signal
		avg      float64
		expected string
	}{
		{"akumulasi kuat", Signal{ADLSlope5: strongBuy, Avg20: avg}, avg, "akumulasi"},
		{"distribusi kuat", Signal{ADLSlope5: strongSell, Avg20: avg}, avg, "distribusi"},
		{"noise positif netral", Signal{ADLSlope5: noise, Avg20: avg}, avg, "netral"},
		{"noise negatif netral", Signal{ADLSlope5: -noise, Avg20: avg}, avg, "netral"},
		{"nol netral", Signal{ADLSlope5: 0, Avg20: avg}, avg, "netral"},
		{"filtered prioritas", Signal{ADLSlope5: strongBuy, Avg20: avg, IsFiltered: true}, avg, "terfilter (noise harga)"},
		{"avg nol netral", Signal{ADLSlope5: strongBuy, Avg20: 0}, 0, "netral"},
	}

	for _, tc := range cases {
		if got := tc.input.InterpretationWithRule(rule, tc.avg); got != tc.expected {
			t.Errorf("%s: InterpretationWithRule(%+v) = %q, want %q", tc.name, tc.input, got, tc.expected)
		}
	}
}
