package main

import (
	"context"
	"log"

	"spikeidx/internal/config"
	"spikeidx/internal/infra"
	"spikeidx/internal/infra/idx"
	"spikeidx/internal/infra/yahoo"

	apihttp "spikeidx/internal/http"
)

func main() {
	cfg := config.Load()
	ctx := context.Background()
	repos, err := infra.WireStrict(ctx, cfg.DatabaseURL)
	if err != nil {
		log.Fatalf("wire: %v", err)
	}

	defer repos.Close()

	deps := &apihttp.Dependencies{
		Stocks:    repos.Stocks,
		Watchlist: repos.Watchlist,
		OHLCV:     repos.OHLCV,
		Signals:   repos.Signals,
		Primary:   yahoo.New(),
		Fallback:  idx.New(),
	}
	addr := ":" + cfg.Port

	log.Printf("spikeidx api listening on %s", addr)

	if err := deps.Router().Run(addr); err != nil {
		log.Fatalf("serve: %v", err)
	}
}
