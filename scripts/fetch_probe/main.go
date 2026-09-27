// Command fetch-probe measures Yahoo Finance fetch latency and the 429
// rate over an explicit list of IDX stock codes.
//
// Usage (codes come from argv only — there is intentionally no default
// symbol list in this file):
//
//	go run ./scripts/fetch_probe -n 20 BBCA BMRI BBRI ...
//	go run ./scripts/fetch_probe -n 100 -o /tmp/probe.csv BBCA BMRI ...
//
// Behavior:
//   - Sequential DailyOHLCV fetch with time.Sleep(time.Second) between
//     symbols, per the Yahoo 1 req/s + browser UA contract.
//   - Always exits 0, even when individual fetches fail; per-symbol
//     outcomes go to the CSV file and the end-of-run summary.
//   - CSV columns: code,latency_ms,status where status is one of
//     ok | http_429 | http_NNN | error | invalid.
//   - Stdout ends with a summary line carrying p95 latency (ms) and the
//     429 count — the baseline numbers retry/backoff tuning needs.
package main

import (
	"context"
	"encoding/csv"
	"flag"
	"fmt"
	"io"
	"os"
	"regexp"
	"sort"
	"strings"
	"time"

	"spikeidx/internal/domain"
	"spikeidx/internal/infra/yahoo"
)

type probeResult struct {
	Code      string
	LatencyMs int64
	Status    string
}

var httpStatusPattern = regexp.MustCompile(`http (\d{3})`)

// classifyErr maps a fetch error to a CSV status value. A nil error is ok;
// anything mentioning 429 is http_429 so the summary can count it even
// when the client wraps the status differently.
func classifyErr(err error) string {
	if err == nil {
		return "ok"
	}
	msg := err.Error()
	if strings.Contains(msg, "429") {
		return "http_429"
	}
	if m := httpStatusPattern.FindStringSubmatch(msg); m != nil {
		return "http_" + m[1]
	}
	return "error"
}

// percentileMs returns the p-th percentile (p in 0..100) of ms using the
// nearest-rank method. Empty input yields 0.
func percentileMs(ms []int64, p float64) int64 {
	if len(ms) == 0 {
		return 0
	}
	sorted := append([]int64(nil), ms...)
	sort.Slice(sorted, func(i, j int) bool { return sorted[i] < sorted[j] })
	rank := int((p / 100 * float64(len(sorted))) + 0.999999999)
	if rank < 1 {
		rank = 1
	}
	if rank > len(sorted) {
		rank = len(sorted)
	}
	return sorted[rank-1]
}

type summary struct {
	Total       int
	OK          int
	RateLimited int
	Other       int
	P50Ms       int64
	P95Ms       int64
	MaxMs       int64
}

// summarize counts outcomes and computes latency percentiles over attempted
// fetches only (rows with status "invalid" never hit the network and are
// excluded from the latency stats, but still counted in Total/Other).
func summarize(results []probeResult) summary {
	s := summary{Total: len(results)}
	var latencies []int64
	for _, r := range results {
		switch r.Status {
		case "ok":
			s.OK++
			latencies = append(latencies, r.LatencyMs)
		case "http_429":
			s.RateLimited++
			latencies = append(latencies, r.LatencyMs)
		case "invalid":
			s.Other++
		default:
			s.Other++
			latencies = append(latencies, r.LatencyMs)
		}
	}
	s.P50Ms = percentileMs(latencies, 50)
	s.P95Ms = percentileMs(latencies, 95)
	s.MaxMs = percentileMs(latencies, 100)
	return s
}

func (s summary) String() string {
	return fmt.Sprintf("probe: n=%d ok=%d http_429=%d other=%d p50_ms=%d p95_ms=%d max_ms=%d",
		s.Total, s.OK, s.RateLimited, s.Other, s.P50Ms, s.P95Ms, s.MaxMs)
}

// writeCSV writes results with a code,latency_ms,status header.
func writeCSV(w io.Writer, results []probeResult) error {
	cw := csv.NewWriter(w)
	if err := cw.Write([]string{"code", "latency_ms", "status"}); err != nil {
		return err
	}
	for _, r := range results {
		if err := cw.Write([]string{r.Code, fmt.Sprintf("%d", r.LatencyMs), r.Status}); err != nil {
			return err
		}
	}
	cw.Flush()
	return cw.Error()
}

// planArgs limits argv to the first n entries (n <= 0 means all).
func planArgs(args []string, n int) []string {
	if n <= 0 || n > len(args) {
		return args
	}
	return args[:n]
}

func run(fs *flag.FlagSet, args []string, stdout, stderr io.Writer, client *yahoo.Client, sleep func(time.Duration)) int {
	n := fs.Int("n", 0, "max symbols to probe (default: all codes given)")
	out := fs.String("o", "probe_results.csv", "CSV output path")
	timeoutSec := fs.Int("timeout", 30, "per-symbol fetch timeout in seconds")
	if err := fs.Parse(args); err != nil {
		fmt.Fprintf(stderr, "fetch-probe: %v\nusage: fetch-probe [-n N] [-o PATH] [-timeout SEC] CODE [CODE...]\n", err)
		return 0
	}
	codes := planArgs(fs.Args(), *n)
	if len(codes) == 0 {
		fmt.Fprintln(stderr, "usage: fetch-probe [-n N] [-o PATH] [-timeout SEC] CODE [CODE...]")
		return 0
	}

	results := make([]probeResult, 0, len(codes))
	for i, arg := range codes {
		code, err := domain.ParseCode(arg)
		if err != nil {
			fmt.Fprintf(stderr, "fetch-probe: skip %q: %v\n", arg, err)
			results = append(results, probeResult{Code: strings.ToUpper(strings.TrimSpace(arg)), LatencyMs: 0, Status: "invalid"})
			continue
		}
		ctx, cancel := context.WithTimeout(context.Background(), time.Duration(*timeoutSec)*time.Second)
		start := time.Now()
		candles, ferr := client.DailyOHLCV(ctx, code)
		latency := time.Since(start)
		cancel()
		status := classifyErr(ferr)
		if ferr != nil {
			fmt.Fprintf(stderr, "fetch-probe: %s status=%s latency_ms=%d err=%v\n", code, status, latency.Milliseconds(), ferr)
		} else {
			fmt.Fprintf(stderr, "fetch-probe: %s status=ok latency_ms=%d candles=%d\n", code, latency.Milliseconds(), len(candles))
		}
		results = append(results, probeResult{Code: string(code), LatencyMs: latency.Milliseconds(), Status: status})
		if i < len(codes)-1 {
			sleep(time.Second)
		}
	}

	f, err := os.Create(*out)
	if err != nil {
		fmt.Fprintf(stderr, "fetch-probe: write %s: %v\n", *out, err)
	} else {
		werr := writeCSV(f, results)
		cerr := f.Close()
		if werr != nil {
			fmt.Fprintf(stderr, "fetch-probe: write %s: %v\n", *out, werr)
		} else if cerr != nil {
			fmt.Fprintf(stderr, "fetch-probe: close %s: %v\n", *out, cerr)
		} else {
			fmt.Fprintf(stderr, "fetch-probe: wrote %d rows to %s\n", len(results), *out)
		}
	}

	fmt.Fprintln(stdout, summarize(results).String())
	return 0
}

func main() {
	os.Exit(run(flag.NewFlagSet("fetch-probe", flag.ContinueOnError), os.Args[1:], os.Stdout, os.Stderr, yahoo.New(), time.Sleep))
}
