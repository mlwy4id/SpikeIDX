package yahoo

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"math/rand"
	"net"
	"net/http"
	"net/url"
	"os"
	"time"

	"spikeidx/internal/domain"
)

const chartURL = "https://query1.finance.yahoo.com/v8/finance/chart/"

const searchURL = "https://query1.finance.yahoo.com/v1/finance/search?q="

var httpClient = &http.Client{Timeout: 20 * time.Second}

// HTTPClient is the subset of *http.Client used by Client. *http.Client
// satisfies it, and tests substitute httptest-backed clients.
type HTTPClient interface {
	Do(*http.Request) (*http.Response, error)
}

const (
	defaultMaxRetries  = 3
	defaultBaseBackoff = time.Second
)

// Client fetches Yahoo Finance quotes. The zero-value knobs below are
// unexported on purpose: public signatures (New, DailyOHLCV, Search) stay
// stable, and retry injection happens through Options only.
type Client struct {
	httpClient    HTTPClient
	chartBaseURL  string
	searchBaseURL string
	sleep         func(time.Duration)
	maxRetries    int
	baseBackoff   time.Duration
}

// Option tunes a Client. Only HTTPClient and sleep injection are exposed;
// endpoint and backoff knobs keep production defaults.
type Option func(*Client)

// WithHTTPClient substitutes the transport (tests use httptest servers).
func WithHTTPClient(h HTTPClient) Option {
	return func(c *Client) {
		if h != nil {
			c.httpClient = h
		}
	}
}

// WithSleepFunc substitutes the backoff wait so tests run without real
// waiting. Production uses time.Sleep.
func WithSleepFunc(fn func(time.Duration)) Option {
	return func(c *Client) {
		if fn != nil {
			c.sleep = fn
		}
	}
}

func New(opts ...Option) *Client {
	c := &Client{
		httpClient:    httpClient,
		chartBaseURL:  chartURL,
		searchBaseURL: searchURL,
		sleep:         time.Sleep,
		maxRetries:    defaultMaxRetries,
		baseBackoff:   defaultBaseBackoff,
	}
	for _, opt := range opts {
		opt(c)
	}
	return c
}

// badStatusError carries a final non-200 status from doWithRetry to the
// caller, which renders the same per-method message as before
// ("yahoo: http %d for %s" / "yahoo search: http %d").
type badStatusError struct {
	status int
}

func (e *badStatusError) Error() string {
	return fmt.Sprintf("yahoo: http %d", e.status)
}

// isRetryableStatus reports whether an HTTP status is worth retrying: 429
// rate-limit plus 5xx server transients. Other 4xx (notably 401/404) fail
// immediately. HTTP 200 with an empty body is NOT retriable here — decode
// stays success-with-empty, since empty can be legitimate.
func isRetryableStatus(status int) bool {
	return status == http.StatusTooManyRequests || (status >= 500 && status <= 599)
}

// isRetryableErr reports timeout-transient transport errors. Caller-context
// expiry is handled separately via ctx.Err and never retried as transient.
func isRetryableErr(err error) bool {
	var netErr net.Error
	if errors.As(err, &netErr) && netErr.Timeout() {
		return true
	}
	return os.IsTimeout(err)
}

// backoff returns base*2^attempt plus up to one base of jitter
// (attempt 0: ~1-2s, 1: ~2-3s, 2: ~4-5s at the default 1s base).
func (c *Client) backoff(attempt int) time.Duration {
	return c.baseBackoff<<attempt + time.Duration(rand.Int63n(int64(c.baseBackoff)))
}

// wait sleeps one backoff step, honoring cancellation before and after the
// sleep. An in-progress sleep runs to completion (bounded to ~2*base*2^attempt),
// so callers with tight deadlines should use a context with timeout.
func (c *Client) wait(ctx context.Context, attempt int) error {
	if err := ctx.Err(); err != nil {
		return err
	}
	c.sleep(c.backoff(attempt))
	return ctx.Err()
}

// doWithRetry builds a fresh request per attempt and executes it, retrying
// 429/5xx/timeout-transient failures up to maxRetries times with exponential
// backoff plus jitter. HTTP 200 returns the open response for the caller to
// decode (decode errors are final, never retried).
func (c *Client) doWithRetry(ctx context.Context, buildReq func() (*http.Request, error)) (*http.Response, error) {
	maxAttempts := c.maxRetries + 1
	if maxAttempts < 1 {
		maxAttempts = 1
	}
	var err error
	for attempt := range maxAttempts {
		if ctxErr := ctx.Err(); ctxErr != nil {
			return nil, ctxErr
		}
		var req *http.Request
		req, err = buildReq()
		if err != nil {
			return nil, err
		}
		var resp *http.Response
		resp, err = c.httpClient.Do(req)
		if err != nil {
			if ctx.Err() != nil {
				return nil, ctx.Err()
			}
			if isRetryableErr(err) && attempt+1 < maxAttempts {
				if waitErr := c.wait(ctx, attempt); waitErr != nil {
					return nil, waitErr
				}
				continue
			}
			return nil, err
		}
		if resp.StatusCode == http.StatusOK {
			return resp, nil
		}
		closeBody(resp)
		if !isRetryableStatus(resp.StatusCode) || attempt+1 >= maxAttempts {
			return nil, &badStatusError{status: resp.StatusCode}
		}
		if waitErr := c.wait(ctx, attempt); waitErr != nil {
			return nil, waitErr
		}
	}
	return nil, err
}

// closeBody closes a failed response body. Draining for connection reuse is
// deliberately skipped: at the contractual 1 req/s the extra read adds
// nothing, and draining would need an errcheck exemption for its error.
func closeBody(resp *http.Response) {
	_ = resp.Body.Close()
}

type chartResponse struct {
	Chart struct {
		Result []struct {
			Timestamp  []int64 `json:"timestamp"`
			Indicators struct {
				Quote []struct {
					Open   []*float64 `json:"open"`
					High   []*float64 `json:"high"`
					Low    []*float64 `json:"low"`
					Close  []*float64 `json:"close"`
					Volume []*int64   `json:"volume"`
				} `json:"quote"`
			} `json:"indicators"`
		} `json:"result"`
		Error *struct {
			Code        string `json:"code"`
			Description string `json:"description"`
		} `json:"error"`
	} `json:"chart"`
}

func floatValue(p *float64) float64 {
	if p == nil {
		return 0
	}
	return *p
}

func intValue(p *int64) int64 {
	if p == nil {
		return 0
	}
	return *p
}

func (c *Client) DailyOHLCV(ctx context.Context, code domain.Code) ([]domain.Candle, error) {
	symbol := code.YahooSymbol()
	end := time.Now()
	start := end.AddDate(0, -3, 0)
	u := fmt.Sprintf("%s%s?period1=%d&period2=%d&interval=1d&events=div%%7Csplit",
		c.chartBaseURL, url.PathEscape(symbol), start.Unix(), end.Unix())

	buildReq := func() (*http.Request, error) {
		req, err := http.NewRequestWithContext(ctx, http.MethodGet, u, nil)
		if err != nil {
			return nil, err
		}
		req.Header.Set("User-Agent", "Mozilla/5.0 (X11; Linux x86_64) SpikeIDX/0.1")
		return req, nil
	}
	resp, err := c.doWithRetry(ctx, buildReq)
	if err != nil {
		var statusErr *badStatusError
		if errors.As(err, &statusErr) {
			return nil, fmt.Errorf("yahoo: http %d for %s", statusErr.status, symbol)
		}
		return nil, err
	}
	defer func() { _ = resp.Body.Close() }()
	return decodeChart(symbol, resp.Body)
}

func decodeChart(symbol string, r io.Reader) ([]domain.Candle, error) {
	var cr chartResponse
	if err := json.NewDecoder(r).Decode(&cr); err != nil {
		return nil, err
	}
	if cr.Chart.Error != nil {
		return nil, fmt.Errorf("yahoo: %s %s", cr.Chart.Error.Code, cr.Chart.Error.Description)
	}
	if len(cr.Chart.Result) == 0 {
		return nil, fmt.Errorf("yahoo: empty result for %s", symbol)
	}
	res := cr.Chart.Result[0]
	if len(res.Indicators.Quote) == 0 {
		return nil, fmt.Errorf("yahoo: no quote for %s", symbol)
	}
	q := res.Indicators.Quote[0]

	var out []domain.Candle
	for i, ts := range res.Timestamp {
		if i >= len(q.Close) {
			break
		}
		out = append(out, domain.Candle{
			Date:   time.Unix(ts, 0).UTC(),
			Open:   floatValue(q.Open[i]),
			High:   floatValue(q.High[i]),
			Low:    floatValue(q.Low[i]),
			Close:  floatValue(q.Close[i]),
			Volume: intValue(q.Volume[i]),
		})
	}
	return out, nil
}

type searchResponse struct {
	Quotes []struct {
		Symbol    string `json:"symbol"`
		Shortname string `json:"shortname"`
		Longname  string `json:"longname"`
		Sector    string `json:"sector"`
	} `json:"quotes"`
}

func (c *Client) Search(ctx context.Context, query string) ([]domain.Stock, error) {
	u := c.searchBaseURL + url.QueryEscape(query)
	buildReq := func() (*http.Request, error) {
		req, err := http.NewRequestWithContext(ctx, http.MethodGet, u, nil)
		if err != nil {
			return nil, err
		}
		req.Header.Set("User-Agent", "Mozilla/5.0 (X11; Linux x86_64) SpikeIDX/0.1")
		return req, nil
	}
	resp, err := c.doWithRetry(ctx, buildReq)
	if err != nil {
		var statusErr *badStatusError
		if errors.As(err, &statusErr) {
			return nil, fmt.Errorf("yahoo search: http %d", statusErr.status)
		}
		return nil, err
	}
	defer func() { _ = resp.Body.Close() }()
	return decodeSearch(resp.Body)
}

func decodeSearch(r io.Reader) ([]domain.Stock, error) {
	var sr searchResponse
	if err := json.NewDecoder(r).Decode(&sr); err != nil {
		return nil, err
	}
	var out []domain.Stock
	for _, q := range sr.Quotes {
		if len(q.Symbol) < 3 || q.Symbol[len(q.Symbol)-3:] != ".JK" {
			continue
		}
		name := q.Shortname
		if q.Longname != "" {
			name = q.Longname
		}
		code, err := domain.ParseCode(q.Symbol)
		if err != nil {
			continue
		}
		out = append(out, domain.Stock{
			Code:        code,
			YahooSymbol: q.Symbol,
			Name:        name,
			Sector:      q.Sector,
		})
	}
	if out == nil {
		out = []domain.Stock{}
	}
	return out, nil
}
