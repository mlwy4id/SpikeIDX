package main

import (
	"bytes"
	"errors"
	"flag"
	"strings"
	"testing"
	"time"

	"spikeidx/internal/infra/yahoo"
)

func TestClassifyErr(t *testing.T) {
	cases := []struct {
		name string
		err  error
		want string
	}{
		{"nil", nil, "ok"},
		{"429 wrapped", errors.New("yahoo: http 429 for BBCA.JK"), "http_429"},
		{"bare 429", errors.New("unexpected 429"), "http_429"},
		{"500", errors.New("yahoo: http 500 for BBCA.JK"), "http_500"},
		{"404 search", errors.New("yahoo search: http 404"), "http_404"},
		{"generic", errors.New("connection reset"), "error"},
	}
	for _, tc := range cases {
		if got := classifyErr(tc.err); got != tc.want {
			t.Errorf("%s: classifyErr = %q, want %q", tc.name, got, tc.want)
		}
	}
}

func TestPercentileMs(t *testing.T) {
	if got := percentileMs(nil, 95); got != 0 {
		t.Errorf("empty = %d, want 0", got)
	}
	ms := []int64{
		10, 20, 30, 40, 50, 60, 70, 80, 90, 100,
		110, 120, 130, 140, 150, 160, 170, 180, 190, 200,
	}
	if got := percentileMs(ms, 95); got != 190 {
		t.Errorf("p95 = %d, want 190", got)
	}
	if got := percentileMs(ms, 50); got != 100 {
		t.Errorf("p50 = %d, want 100", got)
	}
	if got := percentileMs(ms, 100); got != 200 {
		t.Errorf("max = %d, want 200", got)
	}
	if got := percentileMs([]int64{42}, 95); got != 42 {
		t.Errorf("single = %d, want 42", got)
	}
}

func TestSummarize(t *testing.T) {
	results := []probeResult{
		{Code: "BBCA", LatencyMs: 100, Status: "ok"},
		{Code: "BMRI", LatencyMs: 200, Status: "ok"},
		{Code: "BBRI", LatencyMs: 300, Status: "http_429"},
		{Code: "XXXX", LatencyMs: 0, Status: "invalid"},
	}
	s := summarize(results)
	if s.Total != 4 || s.OK != 2 || s.RateLimited != 1 || s.Other != 1 {
		t.Fatalf("counts wrong: %+v", s)
	}
	// invalid row (latency 0) must not pollute latency stats: p50 over
	// [100 200 300] is 200, not 100.
	if s.P50Ms != 200 {
		t.Errorf("p50 = %d, want 200", s.P50Ms)
	}
	if s.P95Ms != 300 || s.MaxMs != 300 {
		t.Errorf("p95/max = %d/%d, want 300/300", s.P95Ms, s.MaxMs)
	}
	if !strings.Contains(s.String(), "http_429=1") || !strings.Contains(s.String(), "p95_ms=") {
		t.Errorf("summary string missing 429/p95: %q", s.String())
	}
}

func TestWriteCSV(t *testing.T) {
	var buf bytes.Buffer
	results := []probeResult{
		{Code: "BBCA", LatencyMs: 123, Status: "ok"},
		{Code: "BBRI", LatencyMs: 456, Status: "http_429"},
	}
	if err := writeCSV(&buf, results); err != nil {
		t.Fatal(err)
	}
	want := "code,latency_ms,status\nBBCA,123,ok\nBBRI,456,http_429\n"
	if buf.String() != want {
		t.Errorf("csv = %q, want %q", buf.String(), want)
	}
}

func TestPlanArgs(t *testing.T) {
	args := []string{"A", "B", "C"}
	if got := planArgs(args, 0); len(got) != 3 {
		t.Errorf("n=0 = %v, want all", got)
	}
	if got := planArgs(args, 2); len(got) != 2 || got[1] != "B" {
		t.Errorf("n=2 = %v, want [A B]", got)
	}
	if got := planArgs(args, 99); len(got) != 3 {
		t.Errorf("n=99 = %v, want all", got)
	}
}

// run with no codes must still exit 0 and print usage, never probe.
func TestRunNoArgsExitsZero(t *testing.T) {
	var stdout, stderr bytes.Buffer
	code := run(flag.NewFlagSet("fetch-probe", flag.ContinueOnError), nil, &stdout, &stderr, yahoo.New(), func(time.Duration) {})
	if code != 0 {
		t.Fatalf("exit = %d, want 0", code)
	}
	if !strings.Contains(stderr.String(), "usage") {
		t.Errorf("stderr missing usage: %q", stderr.String())
	}
}
