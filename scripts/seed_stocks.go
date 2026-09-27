// Command seed-stocks idempotently seeds stocks_master from CLI codes.
//
// Usage: go run ./scripts/seed_stocks.go BBCA TLKM ...
//
// Each code is validated with domain.ParseCode, its name/sector looked up
// via Yahoo Search, and seeded through postgres.SeedStocks. When Search
// fails or returns no exact .JK match, the code itself is used as the name
// (same code-as-name fallback DailyIngest uses); re-running refreshes it.
// There is intentionally no default stock list: codes come from argv only.
//
// Requires DATABASE_URL (Session mode 5432); fails hard without a reachable DB.
package main

import (
	"context"
	"log"
	"os"
	"time"

	"spikeidx/internal/config"
	"spikeidx/internal/domain"
	"spikeidx/internal/infra"
	"spikeidx/internal/infra/postgres"
	"spikeidx/internal/infra/yahoo"
)

func main() {
	if len(os.Args) < 2 {
		log.Fatalf("usage: go run ./scripts/seed_stocks.go CODE [CODE...]")
	}

	cfg := config.Load()
	ctx := context.Background()

	repos, err := infra.WireStrict(ctx, cfg.DatabaseURL)
	if err != nil {
		log.Fatalf("wire: %v", err)
	}
	defer repos.Close()

	provider := yahoo.New()
	stocks := make([]domain.Stock, 0, len(os.Args)-1)

	for i, arg := range os.Args[1:] {
		if i > 0 {
			time.Sleep(time.Second)
		}
		code, err := domain.ParseCode(arg)
		if err != nil {
			log.Fatalf("seed: %v", err)
		}
		stocks = append(stocks, lookupStock(ctx, provider, code))
	}

	inserted, err := postgres.SeedStocks(ctx, repos.Stocks, stocks)
	if err != nil {
		log.Fatalf("seed: %v", err)
	}
	log.Printf("seed: inserted %d of %d", inserted, len(stocks))
}

func lookupStock(ctx context.Context, provider *yahoo.Client, code domain.Code) domain.Stock {
	fallback := domain.Stock{Code: code, YahooSymbol: code.YahooSymbol(), Name: string(code)}

	res, err := provider.Search(ctx, string(code))
	if err != nil {
		log.Printf("seed: search %s failed (%v); using code-as-name", code, err)
		return fallback
	}
	for _, s := range res {
		if s.Code == code {
			return s
		}
	}
	return fallback
}
