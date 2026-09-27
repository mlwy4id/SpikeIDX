package http

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	stdhttp "net/http"
	"net/http/httptest"
	"os"
	"strings"
	"testing"
	"time"

	"spikeidx/internal/domain"

	"github.com/gin-gonic/gin"
)

func TestMain(m *testing.M) {
	gin.SetMode(gin.TestMode)
	os.Exit(m.Run())
}

type fakeStocks struct{ data map[domain.Code]domain.Stock }

func (f *fakeStocks) Ensure(_ context.Context, s domain.Stock) error {
	if f.data == nil {
		f.data = map[domain.Code]domain.Stock{}
	}

	f.data[s.Code] = s
	return nil
}

func (f *fakeStocks) Get(_ context.Context, code domain.Code) (domain.Stock, error) {
	s, ok := f.data[code]

	if !ok {
		return domain.Stock{}, domain.ErrStockUnknown
	}

	return s, nil
}

type fakeWatchlist struct{ codes []domain.Code }

func (f *fakeWatchlist) List(_ context.Context, _ domain.UserID) ([]domain.Code, error) {
	return append([]domain.Code(nil), f.codes...), nil
}

func (f *fakeWatchlist) Add(_ context.Context, _ domain.UserID, code domain.Code) error {
	f.codes = append(f.codes, code)
	return nil
}

func (f *fakeWatchlist) Remove(_ context.Context, _ domain.UserID, code domain.Code) error {
	out := f.codes[:0]

	for _, c := range f.codes {
		if c != code {
			out = append(out, c)
		}
	}

	f.codes = out
	return nil
}

func (f *fakeWatchlist) Count(_ context.Context, _ domain.UserID) (int, error) {
	return len(f.codes), nil
}

type fakeOHLCV struct {
	rows     []domain.OHLCV
	gotLimit int
}

func (fakeOHLCV) UpsertBatch(_ context.Context, _ []domain.OHLCV) error { return nil }
func (f *fakeOHLCV) History(_ context.Context, _ domain.Code, limit int) ([]domain.OHLCV, error) {
	f.gotLimit = limit
	rows := append([]domain.OHLCV(nil), f.rows...)
	if limit > 0 && len(rows) > limit {
		rows = rows[len(rows)-limit:]
	}
	return rows, nil
}

type fakeSignals struct{}

func (fakeSignals) Upsert(_ context.Context, _ domain.Signal) error { return nil }
func (fakeSignals) ByDate(_ context.Context, _ domain.TradingDate, _ bool) ([]domain.Signal, error) {
	return nil, nil
}

type fakeProvider struct {
	stocks []domain.Stock
	err    error
}

func (f *fakeProvider) Search(_ context.Context, _ string) ([]domain.Stock, error) {
	return f.stocks, f.err
}

func (f *fakeProvider) DailyOHLCV(_ context.Context, _ domain.Code) ([]domain.Candle, error) {
	return nil, nil
}

func testDeps() *Dependencies {
	return &Dependencies{
		Stocks: &fakeStocks{}, Watchlist: &fakeWatchlist{},
		OHLCV: &fakeOHLCV{}, Signals: fakeSignals{},
		Primary: &fakeProvider{}, Fallback: nil,
	}
}

func TestHealth(t *testing.T) {
	d := testDeps()
	req := httptest.NewRequest("GET", "/health", nil)
	rec := httptest.NewRecorder()
	d.Router().ServeHTTP(rec, req)

	if rec.Code != 200 || !strings.Contains(rec.Body.String(), `"status":"ok"`) {
		t.Fatalf("got %d %s", rec.Code, rec.Body.String())
	}
}

func TestWatchlistEmptyIsArray(t *testing.T) {
	d := testDeps()
	req := httptest.NewRequest("GET", "/api/v1/watchlist", nil)
	rec := httptest.NewRecorder()
	d.Router().ServeHTTP(rec, req)

	if rec.Code != 200 {
		t.Fatalf("got %d", rec.Code)
	}

	if strings.TrimSpace(rec.Body.String()) != "[]" {
		t.Fatalf("want [], got %s", rec.Body.String())
	}
}

func TestWatchlistAddUnknownIs404(t *testing.T) {
	d := testDeps()
	req := httptest.NewRequest("POST", "/api/v1/watchlist",
		strings.NewReader(`{"code":"BBCA"}`))
	rec := httptest.NewRecorder()
	d.Router().ServeHTTP(rec, req)

	if rec.Code != stdhttp.StatusNotFound {
		t.Fatalf("got %d %s", rec.Code, rec.Body.String())
	}
}

func TestWatchlistAddInvalidIs400(t *testing.T) {
	d := testDeps()

	for _, body := range []string{`{"code":"!!!"}`, `{"code":""}`, `not-json`} {
		req := httptest.NewRequest("POST", "/api/v1/watchlist",
			strings.NewReader(body))
		rec := httptest.NewRecorder()
		d.Router().ServeHTTP(rec, req)

		if rec.Code != stdhttp.StatusBadRequest {
			t.Fatalf("body %s: got %d %s", body, rec.Code, rec.Body.String())
		}
	}
}

func TestWatchlistAddThenList(t *testing.T) {
	d := testDeps()
	stocks := d.Stocks.(*fakeStocks)

	if err := stocks.Ensure(context.Background(), domain.Stock{Code: "BBCA", YahooSymbol: "BBCA.JK", Name: "BBCA"}); err != nil {
		t.Fatal(err)
	}

	req := httptest.NewRequest("POST", "/api/v1/watchlist",
		strings.NewReader(`{"code":"bbca.jk"}`))
	rec := httptest.NewRecorder()
	d.Router().ServeHTTP(rec, req)

	if rec.Code != stdhttp.StatusCreated {
		t.Fatalf("got %d %s", rec.Code, rec.Body.String())
	}

	req = httptest.NewRequest("GET", "/api/v1/watchlist", nil)
	rec = httptest.NewRecorder()
	d.Router().ServeHTTP(rec, req)
	var list []string

	if err := json.Unmarshal(rec.Body.Bytes(), &list); err != nil {
		t.Fatal(err)
	}

	if len(list) != 1 || list[0] != "BBCA" {
		t.Fatalf("got %v", list)
	}
}

func TestWatchlistFullIs409(t *testing.T) {
	d := testDeps()
	stocks := d.Stocks.(*fakeStocks)

	if err := stocks.Ensure(context.Background(), domain.Stock{Code: "ZZZZ", YahooSymbol: "ZZZZ.JK"}); err != nil {
		t.Fatal(err)
	}

	wl := d.Watchlist.(*fakeWatchlist)

	for i := 0; i < domain.MaxWatchlist; i++ {
		wl.codes = append(wl.codes, domain.Code(fmt.Sprintf("C%03d", i)))
	}

	req := httptest.NewRequest("POST", "/api/v1/watchlist",
		strings.NewReader(`{"code":"ZZZZ"}`))
	rec := httptest.NewRecorder()
	d.Router().ServeHTTP(rec, req)

	if rec.Code != stdhttp.StatusConflict {
		t.Fatalf("got %d %s", rec.Code, rec.Body.String())
	}
}

func TestWatchlistDelete(t *testing.T) {
	d := testDeps()
	wl := d.Watchlist.(*fakeWatchlist)
	wl.codes = []domain.Code{"BBCA"}

	req := httptest.NewRequest("DELETE", "/api/v1/watchlist/BBCA", nil)
	rec := httptest.NewRecorder()
	d.Router().ServeHTTP(rec, req)

	if rec.Code != stdhttp.StatusNoContent {
		t.Fatalf("got %d %s", rec.Code, rec.Body.String())
	}

	if len(wl.codes) != 0 {
		t.Fatalf("got %v", wl.codes)
	}

	req = httptest.NewRequest("DELETE", "/api/v1/watchlist/!!!", nil)
	rec = httptest.NewRecorder()
	d.Router().ServeHTTP(rec, req)

	if rec.Code != stdhttp.StatusBadRequest {
		t.Fatalf("got %d %s", rec.Code, rec.Body.String())
	}
}

func TestSignalsBadDate(t *testing.T) {
	d := testDeps()
	req := httptest.NewRequest("GET", "/api/v1/signals?date=bad", nil)
	rec := httptest.NewRecorder()
	d.Router().ServeHTTP(rec, req)

	if rec.Code != stdhttp.StatusBadRequest {
		t.Fatalf("got %d %s", rec.Code, rec.Body.String())
	}
}

func TestSignalsDefaultDateIsArray(t *testing.T) {
	d := testDeps()
	req := httptest.NewRequest("GET", "/api/v1/signals", nil)
	rec := httptest.NewRecorder()
	d.Router().ServeHTTP(rec, req)

	if rec.Code != 200 || strings.TrimSpace(rec.Body.String()) != "[]" {
		t.Fatalf("got %d %s", rec.Code, rec.Body.String())
	}
}

func TestSearchCachesMaster(t *testing.T) {
	d := testDeps()
	d.Primary = &fakeProvider{stocks: []domain.Stock{{Code: "GOTO", YahooSymbol: "GOTO.JK", Name: "GoTo"}}}
	req := httptest.NewRequest("GET", "/api/v1/search?q=goto", nil)
	rec := httptest.NewRecorder()
	d.Router().ServeHTTP(rec, req)

	if rec.Code != 200 || !strings.Contains(rec.Body.String(), "GOTO") {
		t.Fatalf("got %d %s", rec.Code, rec.Body.String())
	}

	if _, err := d.Stocks.Get(context.Background(), "GOTO"); err != nil {
		t.Fatalf("master not cached: %v", err)
	}
}

func TestSearchBothFailIs502(t *testing.T) {
	d := testDeps()
	d.Primary = &fakeProvider{err: errors.New("boom")}
	d.Fallback = &fakeProvider{err: errors.New("boom")}
	req := httptest.NewRequest("GET", "/api/v1/search?q=x", nil)
	rec := httptest.NewRecorder()
	d.Router().ServeHTTP(rec, req)

	if rec.Code != stdhttp.StatusBadGateway {
		t.Fatalf("got %d %s", rec.Code, rec.Body.String())
	}
}

func TestSearchFallback(t *testing.T) {
	d := testDeps()
	d.Primary = &fakeProvider{err: errors.New("429")}
	d.Fallback = &fakeProvider{stocks: []domain.Stock{{Code: "BBRI", YahooSymbol: "BBRI.JK"}}}
	req := httptest.NewRequest("GET", "/api/v1/search?q=bbri", nil)
	rec := httptest.NewRecorder()
	d.Router().ServeHTTP(rec, req)

	if rec.Code != 200 || !strings.Contains(rec.Body.String(), "BBRI") {
		t.Fatalf("got %d %s", rec.Code, rec.Body.String())
	}
}

func TestSearchMissingQ(t *testing.T) {
	d := testDeps()
	req := httptest.NewRequest("GET", "/api/v1/search", nil)
	rec := httptest.NewRecorder()
	d.Router().ServeHTTP(rec, req)

	if rec.Code != stdhttp.StatusBadRequest {
		t.Fatalf("got %d %s", rec.Code, rec.Body.String())
	}
}

func ohlcvRow(day int, close float64, volume int64) domain.OHLCV {
	return domain.OHLCV{
		Code: "BBCA", Date: time.Date(2026, 9, day, 12, 0, 0, 0, time.UTC),
		Open: close - 1, High: close + 1, Low: close - 2, Close: close, Volume: volume,
	}
}

func TestOHLCVReturnsRows(t *testing.T) {
	d := testDeps()
	d.OHLCV.(*fakeOHLCV).rows = []domain.OHLCV{ohlcvRow(17, 100, 1000), ohlcvRow(18, 102, 2000)}
	req := httptest.NewRequest("GET", "/api/v1/ohlcv/BBCA", nil)
	rec := httptest.NewRecorder()
	d.Router().ServeHTTP(rec, req)

	if rec.Code != 200 {
		t.Fatalf("got %d %s", rec.Code, rec.Body.String())
	}

	var out []map[string]any
	if err := json.Unmarshal(rec.Body.Bytes(), &out); err != nil {
		t.Fatal(err)
	}

	if len(out) != 2 || out[0]["date"] != "2026-09-17" || out[1]["date"] != "2026-09-18" {
		t.Fatalf("got %v", out)
	}

	if out[1]["close"] != 102.0 || out[1]["volume"] != 2000.0 {
		t.Fatalf("got %v", out[1])
	}
}

func TestOHLCVEmptyIsArray(t *testing.T) {
	d := testDeps()
	req := httptest.NewRequest("GET", "/api/v1/ohlcv/BBCA", nil)
	rec := httptest.NewRecorder()
	d.Router().ServeHTTP(rec, req)

	if rec.Code != 200 || strings.TrimSpace(rec.Body.String()) != "[]" {
		t.Fatalf("got %d %s", rec.Code, rec.Body.String())
	}
}

func TestOHLCVInvalidCode(t *testing.T) {
	d := testDeps()
	req := httptest.NewRequest("GET", "/api/v1/ohlcv/!!!", nil)
	rec := httptest.NewRecorder()
	d.Router().ServeHTTP(rec, req)

	if rec.Code != stdhttp.StatusBadRequest {
		t.Fatalf("got %d %s", rec.Code, rec.Body.String())
	}
}

func TestOHLCVDefaultLimit60(t *testing.T) {
	d := testDeps()
	req := httptest.NewRequest("GET", "/api/v1/ohlcv/BBCA", nil)
	rec := httptest.NewRecorder()
	d.Router().ServeHTTP(rec, req)

	if rec.Code != 200 {
		t.Fatalf("got %d %s", rec.Code, rec.Body.String())
	}

	if got := d.OHLCV.(*fakeOHLCV).gotLimit; got != 60 {
		t.Fatalf("got limit %d, want 60", got)
	}
}

func TestOHLCVLimitParam(t *testing.T) {
	d := testDeps()
	d.OHLCV.(*fakeOHLCV).rows = []domain.OHLCV{ohlcvRow(17, 100, 1000), ohlcvRow(18, 102, 2000)}
	req := httptest.NewRequest("GET", "/api/v1/ohlcv/BBCA?limit=1", nil)
	rec := httptest.NewRecorder()
	d.Router().ServeHTTP(rec, req)

	if rec.Code != 200 {
		t.Fatalf("got %d %s", rec.Code, rec.Body.String())
	}

	var out []map[string]any
	if err := json.Unmarshal(rec.Body.Bytes(), &out); err != nil {
		t.Fatal(err)
	}

	if len(out) != 1 || out[0]["date"] != "2026-09-18" {
		t.Fatalf("got %v", out)
	}
}

func TestOHLCVLimitCapped(t *testing.T) {
	d := testDeps()
	req := httptest.NewRequest("GET", "/api/v1/ohlcv/BBCA?limit=9999", nil)
	rec := httptest.NewRecorder()
	d.Router().ServeHTTP(rec, req)

	if rec.Code != 200 {
		t.Fatalf("got %d %s", rec.Code, rec.Body.String())
	}

	if got := d.OHLCV.(*fakeOHLCV).gotLimit; got != 500 {
		t.Fatalf("got limit %d, want 500", got)
	}
}

func TestOHLCVBadLimit(t *testing.T) {
	d := testDeps()

	for _, q := range []string{"?limit=abc", "?limit=-1"} {
		req := httptest.NewRequest("GET", "/api/v1/ohlcv/BBCA"+q, nil)
		rec := httptest.NewRecorder()
		d.Router().ServeHTTP(rec, req)

		if rec.Code != stdhttp.StatusBadRequest {
			t.Fatalf("%s: got %d %s", q, rec.Code, rec.Body.String())
		}
	}
}
