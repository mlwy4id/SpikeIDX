package yahoo

import (
	"os"
	"strings"
	"testing"
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
