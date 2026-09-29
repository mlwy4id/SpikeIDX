package usecase

import (
	"context"
	"sort"

	"spikeidx/internal/domain"
)

func DetectOne(code domain.Code, hist []domain.OHLCV, rule domain.SpikeRule) (sig domain.Signal, spike bool, isFiltered bool, err error) {
	sorted := append([]domain.OHLCV(nil), hist...)
	sort.SliceStable(sorted, func(i, j int) bool { return sorted[i].Date.Before(sorted[j].Date) })
	avg, multiple, zScore, pctChange, ok := Stats(sorted, rule)

	if !ok {
		return domain.Signal{}, false, false, domain.ErrInsufficientData
	}

	spike, isFiltered = IsSpike(multiple, zScore, pctChange, rule)

	adl := ADL(sorted)
	last := sorted[len(sorted)-1]

	sig = domain.Signal{
		Code: code, Date: last.Date, Volume: last.Volume,
		Avg20: avg, Multiple: multiple, ZScore: zScore, Close: last.Close,
		PctChange: pctChange, ADL: adl[len(adl)-1], ADLSlope5: ADLSlope5(adl), CMF: CMF(sorted),
		IsFiltered: isFiltered,
	}

	if !spike {
		return sig, false, false, nil
	}

	return sig, true, isFiltered, nil
}

type SymbolResult struct {
	Code     domain.Code
	HasSpike bool
	Signal   domain.Signal
	HasData  bool
	Reason   string
}

type IngestRepos struct {
	Stocks  domain.StockRepository
	OHLCV   domain.OHLCVRepository
	Signals domain.SignalRepository
}

func DailyIngest(ctx context.Context, provider domain.MarketDataProvider, repos IngestRepos, codes []domain.Code, rule domain.SpikeRule, holidays map[domain.TradingDate]bool) []SymbolResult {
	out := make([]SymbolResult, 0, len(codes))

	for _, code := range codes {
		res := SymbolResult{Code: code}

		if _, err := repos.Stocks.Get(ctx, code); err != nil {
			if err := repos.Stocks.Ensure(ctx, domain.Stock{Code: code, YahooSymbol: code.YahooSymbol(), Name: string(code)}); err != nil {
				res.Reason = err.Error()
				out = append(out, res)
				continue
			}
		}

		if _, err := Backfill(ctx, provider, repos.OHLCV, code); err != nil {
			res.Reason = err.Error()
			out = append(out, res)
			continue
		}

		hist, err := repos.OHLCV.History(ctx, code, 60)
		if err != nil {
			res.Reason = err.Error()
			out = append(out, res)
			continue
		}

		if missing, gapped := DetectGap(hist, func(d domain.TradingDate) bool { return IsTradingDay(d, holidays) }); gapped {
			res.Reason = "gap:" + missing.String() + " missing"
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
			res.Signal = sig
			res.HasData = true
			res.Reason = "no spike"
			out = append(out, res)
			continue
		}

		if err := repos.Signals.Upsert(ctx, sig); err != nil {
			res.Reason = err.Error()
			out = append(out, res)
			continue
		}

		res.HasSpike = true
		res.HasData = true
		res.Signal = sig
		out = append(out, res)
	}

	return out
}
