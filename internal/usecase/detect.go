package usecase

import (
	"context"
	"sort"

	"spikeidx/internal/domain"
)

func DetectOne(code domain.Code, hist []domain.OHLCV, rule domain.SpikeRule) (domain.Signal, bool, bool, error) {
	sorted := append([]domain.OHLCV(nil), hist...)
	sort.SliceStable(sorted, func(i, j int) bool { return sorted[i].Date.Before(sorted[j].Date) })
	avg, mult, z, pct, ok := Stats(sorted, rule)
	if !ok {
		return domain.Signal{}, false, false, domain.ErrInsufficientData
	}
	spike, filtered := IsSpike(mult, z, pct, rule)
	if !spike {
		return domain.Signal{}, false, false, nil
	}
	adl := AccumulationDistributionLine(sorted)
	last := sorted[len(sorted)-1]
	return domain.Signal{
		Code: code, Date: last.Date, Volume: last.Volume,
		Avg20: avg, Multiple: mult, ZScore: z, Close: last.Close,
		PctChange: pct, ADL: adl[len(adl)-1], ADLSlope5: AccumulationDistributionLineSlope5(adl),
		IsFiltered: filtered,
	}, true, filtered, nil
}

type SymbolResult struct {
	Code   domain.Code
	Spike  bool
	Signal domain.Signal
	Reason string
}

type IngestRepos struct {
	Stocks  domain.StockRepository
	OHLCV   domain.OHLCVRepository
	Signals domain.SignalRepository
}

func DailyIngest(ctx context.Context, provider domain.MarketDataProvider, r IngestRepos, codes []domain.Code, rule domain.SpikeRule) []SymbolResult {
	out := make([]SymbolResult, 0, len(codes))
	for _, code := range codes {
		res := SymbolResult{Code: code}
		if _, err := r.Stocks.Get(ctx, code); err != nil {
			_ = r.Stocks.Ensure(ctx, domain.Stock{Code: code, YahooSymbol: code.YahooSymbol(), Name: string(code)})
		}
		if _, err := Backfill(ctx, provider, r.OHLCV, code); err != nil {
			res.Reason = err.Error()
			out = append(out, res)
			continue
		}
		hist, err := r.OHLCV.History(ctx, code, 60)
		if err != nil {
			res.Reason = err.Error()
			out = append(out, res)
			continue
		}
		sig, spike, _, err := DetectOne(code, hist, rule)
		if err != nil {
			res.Reason = err.Error()
			out = append(out, res)
			continue
		}
		if !spike {
			res.Reason = "no spike"
			out = append(out, res)
			continue
		}
		if err := r.Signals.Upsert(ctx, sig); err != nil {
			res.Reason = err.Error()
			out = append(out, res)
			continue
		}
		res.Spike = true
		res.Signal = sig
		out = append(out, res)
	}
	return out
}
