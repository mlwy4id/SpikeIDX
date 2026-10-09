package idx

import (
	"context"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"spikeidx/internal/domain"
)

const nagerFixture = `[{"date":"2026-01-01","localName":"Tahun Baru Masehi","name":"New Year's Day","countryCode":"ID","fixed":false,"global":true,"counties":null,"launchYear":null,"types":["Public"]},` +
	`{"date":"2026-12-25","localName":"Hari Raya Natal","name":"Christmas Day","countryCode":"ID","fixed":false,"global":true,"counties":null,"launchYear":null,"types":["Public"]},` +
	`{"date":"not-a-date","localName":"Sampah","name":"Bogus","countryCode":"ID","fixed":false,"global":true,"counties":null,"launchYear":null,"types":["Public"]}]`

func TestHolidaysParsesNagerDates(t *testing.T) {
	var gotPath string
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		gotPath = r.URL.Path
		w.Header().Set("Content-Type", "application/json")
		if _, err := w.Write([]byte(nagerFixture)); err != nil {
			t.Errorf("write fixture: %v", err)
		}
	}))
	defer srv.Close()

	c := &Calendar{baseURL: srv.URL, client: srv.Client()}
	holidays, err := c.Holidays(context.Background(), 2026)
	if err != nil {
		t.Fatal(err)
	}

	if gotPath != "/api/v3/PublicHolidays/2026/ID" {
		t.Fatalf("unexpected path %q", gotPath)
	}

	for _, date := range []string{"2026-01-01", "2026-12-25"} {
		d, err := domain.ParseTradingDate(date)
		if err != nil {
			t.Fatal(err)
		}
		if !holidays[d] {
			t.Fatalf("expected %s in holidays", date)
		}
	}

	if len(holidays) != 2 {
		t.Fatalf("bogus row must be skipped, got %d holidays", len(holidays))
	}
}

func TestHolidaysHTTPError(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusInternalServerError)
	}))
	defer srv.Close()

	c := &Calendar{baseURL: srv.URL, client: srv.Client()}
	if _, err := c.Holidays(context.Background(), 2026); err == nil {
		t.Fatal("expected error on http 500")
	}
}

func TestHolidaysMapKeyComparable(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		if _, err := w.Write([]byte(nagerFixture)); err != nil {
			t.Errorf("write fixture: %v", err)
		}
	}))
	defer srv.Close()

	c := &Calendar{baseURL: srv.URL, client: srv.Client()}
	holidays, err := c.Holidays(context.Background(), 2026)
	if err != nil {
		t.Fatal(err)
	}

	// Key from the fetch must match the same day built independently
	// (the lookup path IsTradingDay relies on).
	viaUTC := domain.NewTradingDate(time.Date(2026, 1, 1, 12, 0, 0, 0, time.UTC))
	if !holidays[viaUTC] {
		t.Fatal("2026-01-01 must be found through an independently built TradingDate")
	}
}
