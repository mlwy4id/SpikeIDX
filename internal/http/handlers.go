package http

import (
	"context"
	"errors"
	stdhttp "net/http"
	"strconv"
	"time"

	"spikeidx/internal/domain"
	"spikeidx/internal/usecase"

	"github.com/gin-gonic/gin"
)

func (d *Dependencies) health(c *gin.Context) {
	c.JSON(stdhttp.StatusOK, gin.H{"status": "ok"})
}

type stockResponse struct {
	Code        string `json:"code"`
	YahooSymbol string `json:"yahoo_symbol"`
	Name        string `json:"name"`
	Sector      string `json:"sector,omitempty"`
}

func (d *Dependencies) search(c *gin.Context) {
	q := c.Query("q")

	if q == "" {
		c.JSON(stdhttp.StatusBadRequest, gin.H{"error": "missing query param q"})
		return
	}

	res, err := usecase.SearchAndCache(c.Request.Context(), d.Primary, d.Fallback, d.Stocks, q)
	if err != nil {
		c.JSON(stdhttp.StatusBadGateway, gin.H{"error": err.Error()})
		return
	}

	out := make([]stockResponse, 0, len(res))

	for _, s := range res {
		out = append(out, stockResponse{
			Code: string(s.Code), YahooSymbol: s.YahooSymbol, Name: s.Name, Sector: s.Sector,
		})
	}

	c.JSON(stdhttp.StatusOK, out)
}

func (d *Dependencies) watchlistList(c *gin.Context) {
	list, err := d.Watchlist.List(c.Request.Context(), domain.DefaultUser)
	if err != nil {
		c.JSON(stdhttp.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	out := make([]string, 0, len(list))

	for _, code := range list {
		out = append(out, string(code))
	}

	c.JSON(stdhttp.StatusOK, out)
}

type watchlistAddRequest struct {
	Code string `json:"code"`
}

func (d *Dependencies) watchlistAdd(c *gin.Context) {
	var req watchlistAddRequest

	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(stdhttp.StatusBadRequest, gin.H{"error": "invalid JSON body"})
		return
	}

	code, err := usecase.AddToWatchlist(c.Request.Context(), d.Stocks, d.Watchlist, req.Code)
	if err != nil {
		switch {
		case errors.Is(err, domain.ErrInvalidCode):
			c.JSON(stdhttp.StatusBadRequest, gin.H{"error": err.Error()})
		case errors.Is(err, domain.ErrStockUnknown):
			c.JSON(stdhttp.StatusNotFound, gin.H{"error": err.Error()})
		case errors.Is(err, domain.ErrWatchlistFull):
			c.JSON(stdhttp.StatusConflict, gin.H{"error": err.Error()})
		default:
			c.JSON(stdhttp.StatusInternalServerError, gin.H{"error": err.Error()})
		}
		return
	}

	// Backfill must survive request completion: detach from request context.
	// Failure is safe to ignore: backfill is idempotent, next ingest retries.
	go func() {
		ctx := context.WithoutCancel(context.Background())
		_, _ = usecase.Backfill(ctx, d.Primary, d.OHLCV, code) //nolint:errcheck // fire-and-forget, nowhere to report
	}()

	c.JSON(stdhttp.StatusCreated, gin.H{"code": string(code), "status": "backfilling"})
}

func (d *Dependencies) watchlistDelete(c *gin.Context) {
	if err := usecase.RemoveFromWatchlist(c.Request.Context(), d.Watchlist, c.Param("code")); err != nil {
		if errors.Is(err, domain.ErrInvalidCode) {
			c.JSON(stdhttp.StatusBadRequest, gin.H{"error": err.Error()})
			return
		}

		c.JSON(stdhttp.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	c.Status(stdhttp.StatusNoContent)
}

type ohlcvResponse struct {
	Code   string  `json:"code"`
	Date   string  `json:"date"`
	Open   float64 `json:"open"`
	High   float64 `json:"high"`
	Low    float64 `json:"low"`
	Close  float64 `json:"close"`
	Volume int64   `json:"volume"`
}

const (
	defaultHistoryLimit = 60
	maxHistoryLimit     = 500
)

func (d *Dependencies) ohlcv(c *gin.Context) {
	code, err := domain.ParseCode(c.Param("code"))
	if err != nil {
		c.JSON(stdhttp.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}
	limit := defaultHistoryLimit
	if s := c.Query("limit"); s != "" {
		limit, err = strconv.Atoi(s)
		if err != nil || limit <= 0 {
			c.JSON(stdhttp.StatusBadRequest, gin.H{"error": "invalid limit, want 1-500"})
			return
		}
		if limit > maxHistoryLimit {
			limit = maxHistoryLimit
		}
	}
	rows, err := d.OHLCV.History(c.Request.Context(), code, limit)
	if err != nil {
		c.JSON(stdhttp.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	out := make([]ohlcvResponse, 0, len(rows))
	for _, o := range rows {
		out = append(out, ohlcvResponse{
			Code: string(o.Code), Date: domain.NewTradingDate(o.Date).String(),
			Open: o.Open, High: o.High, Low: o.Low, Close: o.Close, Volume: o.Volume,
		})
	}
	c.JSON(stdhttp.StatusOK, out)
}

type signalResponse struct {
	Code           string  `json:"code"`
	Date           string  `json:"date"`
	Volume         int64   `json:"volume"`
	Avg20          float64 `json:"avg20"`
	Multiple       float64 `json:"multiple"`
	ZScore         float64 `json:"z_score"`
	Close          float64 `json:"close"`
	PctChange      float64 `json:"pct_change"`
	ADL            float64 `json:"adl"`
	ADLSlope5      float64 `json:"adl_slope5"`
	IsFiltered     bool    `json:"is_filtered"`
	Interpretation string  `json:"interpretation"`
}

func (d *Dependencies) signals(c *gin.Context) {
	var date domain.TradingDate

	if s := c.Query("date"); s == "" {
		date = domain.NewTradingDate(time.Now())
	} else {
		var err error
		date, err = domain.ParseTradingDate(s)
		if err != nil {
			c.JSON(stdhttp.StatusBadRequest, gin.H{"error": "invalid date, want YYYY-MM-DD"})
			return
		}
	}

	shouldIncludeFiltered := false

	if s := c.Query("include_filtered"); s != "" {
		var err error
		shouldIncludeFiltered, err = strconv.ParseBool(s)
		if err != nil {
			c.JSON(stdhttp.StatusBadRequest, gin.H{"error": "invalid include_filtered, want bool"})
			return
		}
	}

	res, err := d.Signals.ByDate(c.Request.Context(), date, shouldIncludeFiltered)
	if err != nil {
		c.JSON(stdhttp.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	out := make([]signalResponse, 0, len(res))
	for _, s := range res {
		out = append(out, signalResponse{
			Code: string(s.Code), Date: domain.NewTradingDate(s.Date).String(),
			Volume: s.Volume, Avg20: s.Avg20, Multiple: s.Multiple, ZScore: s.ZScore,
			Close: s.Close, PctChange: s.PctChange, ADL: s.ADL, ADLSlope5: s.ADLSlope5,
			IsFiltered: s.IsFiltered, Interpretation: s.Interpretation(),
		})
	}

	c.JSON(stdhttp.StatusOK, out)
}
