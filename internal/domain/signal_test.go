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
		{Signal{ADLSlope5: 1.5}, "netral"},
		{Signal{ADLSlope5: -0.1}, "distribusi"},
		{Signal{ADLSlope5: 0}, "netral"},
		{Signal{ADLSlope5: 99, IsFiltered: true}, "terfilter (noise harga)"},
		{Signal{CMF: 0.30, PctChange: 0.5, ADLSlope5: 1}, "akumulasi kuat sideways"},
		{Signal{CMF: 0.30, PctChange: 3.0, ADLSlope5: 1}, "akumulasi kuat markup"},
		{Signal{CMF: 0.10, PctChange: 0.5, ADLSlope5: 1}, "akumulasi lemah sideways"},
		{Signal{CMF: 0.10, PctChange: 3.0, ADLSlope5: 1}, "akumulasi lemah markup"},
		{Signal{CMF: 0.01, PctChange: 0.5, ADLSlope5: -1}, "distribusi"},
		{Signal{CMF: 0.01, PctChange: 0.5, ADLSlope5: 1}, "netral"},
		{Signal{CMF: 0, PctChange: 0.5, ADLSlope5: 1}, "netral"},
		{Signal{CMF: 0, PctChange: 0.5, ADLSlope5: -1}, "distribusi"},
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
