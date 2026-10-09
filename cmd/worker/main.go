package main

import (
	"context"
	"fmt"
	"log"
	"time"

	"spikeidx/internal/config"
	"spikeidx/internal/domain"
	"spikeidx/internal/infra"
	"spikeidx/internal/infra/idx"
	"spikeidx/internal/infra/telegram"
	"spikeidx/internal/infra/yahoo"
	"spikeidx/internal/usecase"
)

// Worker EOD single run (cron-external): fetch holidays, skip non-trading
// days and empty watchlists with exit 0, ingest per symbol, print the
// digest to stdout and Send it when Telegram is configured.
func main() {
	cfg := config.Load()
	ctx := context.Background()

	repos, err := infra.WireStrict(ctx, cfg.DatabaseURL)
	if err != nil {
		log.Fatalf("wire: %v", err)
	}

	defer repos.Close()

	today := domain.NewTradingDate(time.Now())

	holidays, err := idx.NewCalendar().Holidays(ctx, today.Time().Year())
	if err != nil {
		log.Printf("worker: holidays unavailable (%v), weekend-only", err)
		holidays = nil
	}

	if !usecase.IsTradingDay(today, holidays) {
		log.Printf("worker: %s bukan hari trading, skip", today)
		return
	}

	rule := domain.DefaultSpikeRule()
	ingest := usecase.IngestRepos{Stocks: repos.Stocks, OHLCV: repos.OHLCV, Signals: repos.Signals}

	results, err := runEOD(ctx, repos.Watchlist, ingest, yahoo.New(), holidays, rule)
	if err != nil {
		log.Fatalf("worker: list watchlist: %v", err)
	}

	if len(results) == 0 {
		log.Printf("worker: watchlist empty, nothing to do")
		return
	}

	var actionable []domain.Signal

	for _, res := range results {
		switch res.Status {
		case usecase.StatusNoSpike:
			log.Printf("worker: %s NO-SPIKE %.1fx z=%.2f %+.1f%% CMF=%.3f ADL=%s",
				res.Code, res.Signal.Multiple, res.Signal.ZScore,
				res.Signal.PctChange, res.Signal.CMF, res.Signal.Interpretation())
		case usecase.StatusSpikeActionable, usecase.StatusSpikeFiltered:
			log.Printf("worker: %s SPIKE %.1fx z=%.2f %+.1f%% CMF=%.3f ADL=%s (filtered=%v)",
				res.Code, res.Signal.Multiple, res.Signal.ZScore,
				res.Signal.PctChange, res.Signal.CMF, res.Signal.Interpretation(), res.Signal.IsFiltered)

			if res.Signal.IsActionable() {
				actionable = append(actionable, res.Signal)
			}
		default:
			log.Printf("worker: %s skip (%s)", res.Code, res.Reason)
		}
	}

	digest := telegram.Digest(today.String(), actionable)
	fmt.Printf("worker digest %s\n%s\n", today, digest)

	notifier := telegram.New(cfg.TelegramBotToken, cfg.TelegramChatID)
	if !notifier.IsEnabled() {
		return
	}

	if err := notifier.Send(ctx, digest); err != nil {
		log.Fatalf("worker: telegram send: %v", err)
	}
}

// runEOD executes one EOD pass over the watchlist and returns per-symbol
// results in watchlist order. A nil holidays map means weekend-only.
func runEOD(ctx context.Context, watchlist domain.WatchlistRepository, repos usecase.IngestRepos, provider domain.MarketDataProvider, holidays map[domain.TradingDate]bool, rule domain.SpikeRule) ([]usecase.SymbolResult, error) {
	codes, err := watchlist.List(ctx, domain.DefaultUser)
	if err != nil {
		return nil, err
	}

	out := make([]usecase.SymbolResult, 0, len(codes))
	for i, code := range codes {
		if i > 0 {
			time.Sleep(time.Second)
		}
		out = append(out, usecase.DailyIngest(ctx, provider, repos, []domain.Code{code}, rule, holidays)...)
	}
	return out, nil
}
