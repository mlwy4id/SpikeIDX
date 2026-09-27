package yahoo

import (
	"context"
	"errors"
	"net/http"
	"net/http/httptest"
	"os"
	"strings"
	"sync/atomic"
	"testing"
	"time"
)

const chartFixture = `{"chart":{"result":[{"timestamp":[1758240000,1758326400],"indicators":{"quote":[{"open":[9400,9450],"high":[9500,9550],"low":[9350,9400],"close":[9480,9520],"volume":[10000000,35000000]}]}}],"error":null}}`

const searchFixture = `{"quotes":[
	{"symbol":"BBCA.JK","shortname":"Bank Central Asia","longname":"PT Bank Central Asia Tbk","sector":"Financial Services"},
	{"symbol":"BBCA","shortname":"Bogus","longname":"","sector":""},
	{"symbol":"TLKM.JK","shortname":"","longname":"Telkom Indonesia","sector":""}
]}`

func TestDecodeChart(t *testing.T) {
	candles, err := decodeChart("BBCA.JK", strings.NewReader(chartFixture))
	if err != nil {
		t.Fatal(err)
	}
	if len(candles) != 2 {
		t.Fatalf("got %+v", candles)
	}
	if candles[1].Close != 9520 || candles[1].Volume != 35000000 {
		t.Fatalf("got %+v", candles[1])
	}
	if candles[0].Date.IsZero() {
		t.Fatal("date not parsed")
	}
}

func TestDecodeChartError(t *testing.T) {
	if _, err := decodeChart("X.JK", strings.NewReader(`{"chart":{"error":{"code":"Not Found","description":"no data"}}}`)); err == nil {
		t.Fatal("expected error")
	}
	if _, err := decodeChart("X.JK", strings.NewReader(`{"chart":{"result":[]}}`)); err == nil {
		t.Fatal("expected error on empty result")
	}
}

func TestDecodeSearch(t *testing.T) {
	stocks, err := decodeSearch(strings.NewReader(searchFixture))
	if err != nil {
		t.Fatal(err)
	}
	if len(stocks) != 2 {
		t.Fatalf("got %+v", stocks)
	}
	if stocks[0].Code != "BBCA" || stocks[0].Name != "PT Bank Central Asia Tbk" {
		t.Fatalf("got %+v", stocks[0])
	}
	if stocks[1].Code != "TLKM" || stocks[1].Name != "Telkom Indonesia" {
		t.Fatalf("got %+v", stocks[1])
	}
}

func TestLiveDailyOHLCV(t *testing.T) {
	if os.Getenv("SPIKEIDX_LIVE_YAHOO") == "" {
		t.Skip("set SPIKEIDX_LIVE_YAHOO=1 to hit Yahoo")
	}
	candles, err := New().DailyOHLCV(t.Context(), "BBCA")
	if err != nil {
		t.Fatal(err)
	}
	if len(candles) < 20 {
		t.Fatalf("expected >=20 candles, got %d", len(candles))
	}
}

// scriptedServer replays a fixed status sequence, then repeats the last
// entry. It counts requests so tests can assert retry behavior.
type scriptedServer struct {
	t        *testing.T
	srv      *httptest.Server
	calls    atomic.Int32
	statuses []int
	body     string
}

func newScriptedServer(t *testing.T, body string, statuses ...int) *scriptedServer {
	t.Helper()
	s := &scriptedServer{t: t, statuses: statuses, body: body}
	s.srv = httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		n := int(s.calls.Add(1))
		code := statuses[len(statuses)-1]
		if n <= len(statuses) {
			code = statuses[n-1]
		}
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(code)
		if code == http.StatusOK {
			if _, err := w.Write([]byte(body)); err != nil {
				t.Errorf("write fixture: %v", err)
			}
		}
	}))
	t.Cleanup(s.srv.Close)
	return s
}

// newTestClient points a Client at the scripted server with no-op sleeps so
// tests run fast without real waiting. It returns the client and a counter
// of backoff sleeps performed.
func newTestClient(s *scriptedServer, sleeps *atomic.Int32) *Client {
	c := New(
		WithHTTPClient(s.srv.Client()),
		WithSleepFunc(func(time.Duration) { sleeps.Add(1) }),
	)
	c.chartBaseURL = s.srv.URL + "/chart/"
	c.searchBaseURL = s.srv.URL + "/search?q="
	return c
}

func TestDailyOHLCV_RetriesOn429ThenSucceeds(t *testing.T) {
	s := newScriptedServer(t, chartFixture, http.StatusTooManyRequests, http.StatusTooManyRequests, http.StatusOK)
	var sleeps atomic.Int32
	c := newTestClient(s, &sleeps)

	candles, err := c.DailyOHLCV(context.Background(), "BBCA")
	if err != nil {
		t.Fatalf("expected success after 429 retries, got: %v", err)
	}
	if len(candles) != 2 {
		t.Fatalf("expected 2 candles, got %+v", candles)
	}
	if got := s.calls.Load(); got != 3 {
		t.Fatalf("expected 3 requests (1+2 retries), got %d", got)
	}
	if got := sleeps.Load(); got != 2 {
		t.Fatalf("expected 2 backoff sleeps, got %d", got)
	}
}

func TestDailyOHLCV_RetriesOn500ThenSucceeds(t *testing.T) {
	s := newScriptedServer(t, chartFixture, http.StatusInternalServerError, http.StatusOK)
	var sleeps atomic.Int32
	c := newTestClient(s, &sleeps)

	candles, err := c.DailyOHLCV(context.Background(), "BBCA")
	if err != nil {
		t.Fatalf("expected success after 500 retry, got: %v", err)
	}
	if len(candles) != 2 {
		t.Fatalf("expected 2 candles, got %+v", candles)
	}
	if got := s.calls.Load(); got != 2 {
		t.Fatalf("expected 2 requests (1+1 retry), got %d", got)
	}
	if got := sleeps.Load(); got != 1 {
		t.Fatalf("expected 1 backoff sleep, got %d", got)
	}
}

func TestDailyOHLCV_NoRetryOn401(t *testing.T) {
	s := newScriptedServer(t, chartFixture, http.StatusUnauthorized)
	var sleeps atomic.Int32
	c := newTestClient(s, &sleeps)

	_, err := c.DailyOHLCV(context.Background(), "BBCA")
	if err == nil {
		t.Fatal("expected final 401 error, got nil")
	}
	if !strings.Contains(err.Error(), "401") {
		t.Fatalf("expected 401 in error, got: %v", err)
	}
	if got := s.calls.Load(); got != 1 {
		t.Fatalf("expected exactly 1 request (no retry on 401), got %d", got)
	}
	if got := sleeps.Load(); got != 0 {
		t.Fatalf("expected 0 backoff sleeps on 401, got %d", got)
	}
}

func TestDailyOHLCV_ExhaustsRetries(t *testing.T) {
	s := newScriptedServer(t, chartFixture, http.StatusServiceUnavailable)
	var sleeps atomic.Int32
	c := newTestClient(s, &sleeps)

	_, err := c.DailyOHLCV(context.Background(), "BBCA")
	if err == nil {
		t.Fatal("expected error after retries exhausted, got nil")
	}
	if !strings.Contains(err.Error(), "503") {
		t.Fatalf("expected 503 in final error, got: %v", err)
	}
	if got := s.calls.Load(); got != 4 {
		t.Fatalf("expected 4 requests (1+3 retries), got %d", got)
	}
	if got := sleeps.Load(); got != 3 {
		t.Fatalf("expected 3 backoff sleeps, got %d", got)
	}
}

func TestDailyOHLCV_RespectsContextCancel(t *testing.T) {
	s := newScriptedServer(t, chartFixture, http.StatusTooManyRequests)
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	var sleeps atomic.Int32
	c := New(
		WithHTTPClient(s.srv.Client()),
		WithSleepFunc(func(time.Duration) {
			sleeps.Add(1)
			cancel()
		}),
	)
	c.chartBaseURL = s.srv.URL + "/chart/"
	c.searchBaseURL = s.srv.URL + "/search?q="

	_, err := c.DailyOHLCV(ctx, "BBCA")
	if err == nil {
		t.Fatal("expected context error, got nil")
	}
	if !errors.Is(err, context.Canceled) {
		t.Fatalf("expected context.Canceled, got: %v", err)
	}
	if got := s.calls.Load(); got != 1 {
		t.Fatalf("expected 1 request (no retry after cancel), got %d", got)
	}
}

func TestSearch_RetriesOn429ThenSucceeds(t *testing.T) {
	s := newScriptedServer(t, searchFixture, http.StatusTooManyRequests, http.StatusOK)
	var sleeps atomic.Int32
	c := newTestClient(s, &sleeps)

	stocks, err := c.Search(context.Background(), "BBCA")
	if err != nil {
		t.Fatalf("expected success after 429 retry, got: %v", err)
	}
	if len(stocks) != 2 {
		t.Fatalf("expected 2 stocks, got %+v", stocks)
	}
	if got := s.calls.Load(); got != 2 {
		t.Fatalf("expected 2 requests (1+1 retry), got %d", got)
	}
}
