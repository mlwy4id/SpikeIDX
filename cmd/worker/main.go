package main

import (
	"context"
	"fmt"
	"log"
	"time"

	"spikeidx/internal/config"
	"spikeidx/internal/domain"
	"spikeidx/internal/infra"
	"spikeidx/internal/infra/telegram"
	"spikeidx/internal/infra/yahoo"
	"spikeidx/internal/usecase"
)

// Worker-minimal (M-01/M-02): single run, no cron, no Telegram Send.
// Digest goes to stdout; holidays == nil (weekend-only) until G-06.
func main() {
	cfg := config.Load()
	ctx := context.Background()

	repos, err := infra.WireStrict(ctx, cfg.DatabaseURL)
	if err != nil {
		log.Fatalf("wire: %v", err)
	}

	defer repos.Close()

	today := domain.NewTradingDate(time.Now())

	log.Printf("worker: holiday calendar none (weekend-only); public holidays will read as gaps until G-06")

	if !usecase.IsTradingDay(today, nil) {
		log.Printf("worker: %s bukan hari trading, skip", today)
		return
	}

	codes, err := repos.Watchlist.List(ctx, domain.DefaultUser)
	if err != nil {
		log.Fatalf("worker: list watchlist: %v", err)
	}

	if len(codes) == 0 {
		log.Printf("worker: watchlist empty, nothing to do")
		return
	}

	rule := domain.DefaultSpikeRule()
	ingest := usecase.IngestRepos{Stocks: repos.Stocks, OHLCV: repos.OHLCV, Signals: repos.Signals}
	provider := yahoo.New()

	var actionable []domain.Signal

	for i, code := range codes {
		if i > 0 {
			time.Sleep(time.Second)
		}

		for _, res := range usecase.DailyIngest(ctx, provider, ingest, []domain.Code{code}, rule, nil) {
			if !res.HasSpike {
				log.Printf("worker: %s skip (%s)", res.Code, res.Reason)
				continue
			}

			log.Printf("worker: %s SPIKE %.1fx z=%.2f %+.1f%% ADL=%s (filtered=%v)",
				res.Code, res.Signal.Multiple, res.Signal.ZScore,
				res.Signal.PctChange, res.Signal.Interpretation(), res.Signal.IsFiltered)

			if res.Signal.IsActionable() {
				actionable = append(actionable, res.Signal)
			}
		}
	}

	fmt.Printf("worker digest %s\n%s\n", today, telegram.Digest(today.String(), actionable))
}
