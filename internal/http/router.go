package http

import (
	"spikeidx/internal/domain"

	"github.com/gin-gonic/gin"
)

// Deps wires delivery to domain interfaces. Handlers stay thin:
// validate input, call usecase, map domain errors to HTTP status.
type Dependencies struct {
	Stocks    domain.StockRepository
	Watchlist domain.WatchlistRepository
	OHLCV     domain.OHLCVRepository
	Signals   domain.SignalRepository
	Primary   domain.MarketDataProvider
	Fallback  domain.MarketDataProvider
}

func (d *Dependencies) Router() *gin.Engine {
	r := gin.New()
	r.Use(gin.Recovery())
	r.GET("/health", d.health)
	r.GET("/openapi.yaml", d.openAPISpecHandler)
	r.GET("/swagger/index.html", d.swaggerUIHandler)
	r.GET("/swagger", swaggerRedirectHandler)
	r.GET("/docs", swaggerRedirectHandler)
	r.GET("/api/v1/search", d.search)
	r.GET("/api/v1/watchlist", d.watchlistList)
	r.POST("/api/v1/watchlist", d.watchlistAdd)
	r.DELETE("/api/v1/watchlist/:code", d.watchlistDelete)
	r.GET("/api/v1/signals", d.signals)
	r.GET("/api/v1/ohlcv/:code", d.ohlcv)
	return r
}
