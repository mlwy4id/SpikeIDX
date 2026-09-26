package yahoo

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"time"

	"spikeidx/internal/domain"
)

const chartURL = "https://query1.finance.yahoo.com/v8/finance/chart/"

var httpClient = &http.Client{Timeout: 20 * time.Second}

type Client struct{}

func New() *Client { return &Client{} }

type chartResp struct {
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

func f64(p *float64) float64 {
	if p == nil {
		return 0
	}
	return *p
}

func i64(p *int64) int64 {
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
		chartURL, url.PathEscape(symbol), start.Unix(), end.Unix())

	req, err := http.NewRequestWithContext(ctx, http.MethodGet, u, nil)
	if err != nil {
		return nil, err
	}
	req.Header.Set("User-Agent", "Mozilla/5.0 (X11; Linux x86_64) SpikeIDX/0.1")

	resp, err := httpClient.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("yahoo: HTTP %d for %s", resp.StatusCode, symbol)
	}
	return decodeChart(symbol, resp.Body)
}

func decodeChart(symbol string, r io.Reader) ([]domain.Candle, error) {
	var cr chartResp
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
			Open:   f64(q.Open[i]),
			High:   f64(q.High[i]),
			Low:    f64(q.Low[i]),
			Close:  f64(q.Close[i]),
			Volume: i64(q.Volume[i]),
		})
	}
	return out, nil
}

type searchResp struct {
	Quotes []struct {
		Symbol    string `json:"symbol"`
		Shortname string `json:"shortname"`
		Longname  string `json:"longname"`
		Sector    string `json:"sector"`
	} `json:"quotes"`
}

func (c *Client) Search(ctx context.Context, query string) ([]domain.Stock, error) {
	u := "https://query1.finance.yahoo.com/v1/finance/search?q=" + url.QueryEscape(query)
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, u, nil)
	if err != nil {
		return nil, err
	}
	req.Header.Set("User-Agent", "Mozilla/5.0 (X11; Linux x86_64) SpikeIDX/0.1")

	resp, err := httpClient.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("yahoo search: HTTP %d", resp.StatusCode)
	}
	return decodeSearch(resp.Body)
}

func decodeSearch(r io.Reader) ([]domain.Stock, error) {
	var sr searchResp
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
