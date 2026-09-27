# Implementation Plan — SpikeIDX (trackable)

Tanggal: 2026-09-26 | Status: v1 API + Yahoo E2E berjalan lokal; E2E cloud + worker menunggu.

Legenda status: `done` = selesai + gate hijau | `pending` = belum dikerjakan | `blocked` = butuh input/kredensial user.

## 0. Keputusan user (final, mengikat)

| ID | Keputusan | Status |
|---|---|---|
| K-01 | Worker watchlist kosong → exit 0 + log `"watchlist empty"` (tanpa dev-default BBCA/TLKM/BBRI) | done (aturan, worker v2 belum dibangun) |
| K-02 | `GET /signals` tanpa `date` → default hari ini WIB (`domain.NewTradingDate(time.Now())`) | done |
| K-03 | Worker ditunda ke v2; fokus: HTTP + `cmd/api` + Yahoo E2E (search → cache → watchlist → backfill) | done |
| K-04 | HTTP pakai `gin-gonic/gin` (2026-09-26) | done |
| K-05 | Tetap `WireStrict` fail-hard, tanpa fallback lokal/memory | done |
| K-06 | Naming cleanup disetujui per-layer (domain → usecase → infra/http) | done (2026-09-26) |

## 1. Prinsip

| ID | Prinsip | Status |
|---|---|---|
| P-01 | Handler tipis: validasi + panggil usecase + map error; tanpa bisnis baru | done |
| P-02 | `usecase` tidak import `infra`/`http`; inject via interface `domain` | done |
| P-03 | `go test ./... && go vet ./... && go build ./...` hijau sebelum selesai; `gofmt` + `golangci-lint` bersih | done (lokal) |

## 2. Scope v1 — API + Yahoo E2E

### 2.1 `internal/http/` (Gin)

`Dependencies` struct: `Stocks`, `Watchlist`, `OHLCV`, `Signals`, `Primary` + `Fallback`. `Router()` = `gin.New()` + `Recovery` (tanpa `Logger`).

| ID | Endpoint | Usecase | Error mapping | Status |
|---|---|---|---|---|
| H-01 | `GET /health` | — | `{"status":"ok"}` | done |
| H-02 | `GET /api/v1/search?q=` | `SearchAndCache(primary, fallback, stocks, q)` | gagal dua provider → 502 | done |
| H-03 | `GET /api/v1/watchlist` | `List(DefaultUser)` | selalu `[]`, bukan `null` | done |
| H-04 | `POST /api/v1/watchlist {"code"}` | `AddToWatchlist` + backfill async | 400 `ErrInvalidCode`, 404 `ErrStockUnknown`, 409 `ErrWatchlistFull` | done |
| H-05 | `DELETE /api/v1/watchlist/:code` | `RemoveFromWatchlist` | 400 kode invalid | done |
| H-06 | `GET /api/v1/signals?date=&include_filtered=` | `Signals.ByDate` | 400 tanggal invalid; `date` kosong = hari ini WIB | done |
| H-07 | Backfill async via `context.WithoutCancel`, `usecase.Backfill`; semua list guard non-nil (`[]`) | — | — | done |
| H-08 | `GET /api/v1/ohlcv/:code?limit=` | `OHLCV.History` | 400 kode/limit invalid; default 60, cap 500; kosong → `[]`; tanggal encode WIB | done |

### 2.2 `cmd/api/main.go`

| ID | Item | Detail | Status |
|---|---|---|---|
| A-01 | `config.Load()` → `WireStrict(ctx, DatabaseURL)` → Yahoo primary + IDX fallback → `Dependencies` → `Run(":"+Port)` | gagal keras tanpa DB | done |

### 2.3 `internal/http/handlers_test.go`

| ID | Kasus | Ekspektasi | Status |
|---|---|---|---|
| T-01 | health 200 | `{"status":"ok"}` | done |
| T-02 | watchlist kosong | `[]` bukan `null` | done |
| T-03 | POST tanpa search | 404 | done |
| T-04 | POST kode sampah / body kosong | 400 | done |
| T-05 | watchlist penuh ke-101 | 409 | done |
| T-06 | signals `date=bad` | 400 | done |
| T-07 | search-cache + fallback + 502, signals default-date `[]` | sesuai mapping | done |
| T-08 | ohlcv rows oldest-first + tanggal WIB, kosong `[]`, 400 kode/limit, default limit 60, cap 500 | sesuai mapping | done |

### 2.4 Yahoo E2E (unit + live)

| ID | Item | Detail | Status |
|---|---|---|---|
| Y-01 | `SearchAndCache` = Yahoo primary → IDX fallback → `Ensure` master ("wajib search dulu") | kontrak search-first | done (unit) |
| Y-02 | `Backfill` = `DailyOHLCV` → normalisasi WIB → `UpsertBatch` | fixture `decodeChart`; live opsional `SPIKEIDX_LIVE_YAHOO=1` | done (unit) |
| Y-03 | Smoke API (butuh DB): search → watchlist → backfill nyata → `History` terisi | butuh `DATABASE_URL` | blocked |

### 2.5 Sinkron docs

| ID | File | Status |
|---|---|---|
| D-01 | `docs/AGENTS.md` §3/§5 | done (perlu update kecil: `ADL` baru, lihat §6) |
| D-02 | `README.md` | done (bersihkan bagian basi saat delivery jadi) |
| D-03 | `docs/IMPLEMENTATION_REPORT.md` §12l | done |

## 3. Scope v2 (ditunda) — `cmd/worker`

| ID | Item | Detail | Status |
|---|---|---|---|
| W-01 | `config.Load()` → `WireStrict` → `IsTradingDay(today, nil)` → skip weekend | log + exit 0 | pending |
| W-02 | Watchlist kosong → log + exit 0 | tanpa dev-default | pending |
| W-03 | `DailyIngest` per kode + `sleep 1s` antar simbol (rate-limit di delivery) | — | pending |
| W-04 | Log `success/failed+reason` per simbol; filter `IsActionable()` → `Digest` → `Send` bila `IsEnabled()` | rule `DefaultSpikeRule()`; cron `30 16 * * 1-5 Asia/Jakarta` | pending |

## 4. Verifikasi v1

| ID | Perintah | Ekspektasi | Status |
|---|---|---|---|
| V-01 | `go test ./...` | semua hijau (contract skip tanpa `DATABASE_URL`) | done |
| V-02 | `go vet ./... && go build ./...` | bersih | done |
| V-03 | `gofmt -l .` kosong + `golangci-lint run ./...` 0 issues | bersih | done |
| V-04 | `go run ./cmd/api` → `GET /health` → `{"status":"ok"}` | ok | done (dengan DB) |
| V-05 | `POST /watchlist {"code":"X"}` tanpa search → 404 | 404 | done (dengan DB) |

## 5. Hasil grill-me 2026-09-26 (mengikat)

### 5.1 Keputusan kunci

| ID | Keputusan | Status |
|---|---|---|
| G-01 | Next = E2E cloud + smoke; DONE = full loop dry-run | pending (E-01–E-04 ready, butuh run manual + `DATABASE_URL`) |
| G-02 | Gap Yahoo: non-market day → skip; market-day gap → wajib deteksi (jangan `Stats` di window berlubang) | done (`DetectGap` + wiring `DailyIngest`, C-01) |
| G-03 | Rule 2x/z>2/|2%| = heuristik; tuning via manual review | done (rule editable via param) |
| G-04 | Tetap WireStrict fail-hard | done |
| G-05 | Full loop butuh worker-minimal (API-only tidak cukup) | done (`cmd/worker` minimal ada) |
| G-06 | Libur BEI via fetch, bukan hardcode | pending (kini weekend-only; worker + gap pakai `nil`) |

### 5.2 Worker-minimal (bukan v2 penuh)

| ID | Item | Status |
|---|---|---|
| M-01 | `config.Load → WireStrict → IsTradingDay(today,nil) → DailyIngest per-code + sleep 1s → log per-simbol → Digest ke stdout` | done (`cmd/worker/main.go`) |
| M-02 | Tanpa cron, tanpa `robfig/cron`, tanpa Telegram `Send` | done (single run, digest stdout saja) |
| M-03 | Menutup hutang Dockerfile (target `./cmd/worker` kini gagal build) | done (`go build ./...` hijau) |

### 5.3 E2E DONE (butuh `DATABASE_URL` Session 5432)

| ID | Kriteria | Status |
|---|---|---|
| E-01 | `DATABASE_URL=... go test -run TestContract ./internal/infra/postgres/` hijau | ready (URL sudah 5432, tinggal run manual) |
| E-02 | `go run ./cmd/api` auto-migrate 001–003 → `GET /health` ok | ready (run 27 Sep mentok `42P05` di URL lama; ulangi dengan URL 5432) |
| E-03 | Live Yahoo: search → cache master → watchlist → backfill WIB terisi → `GET /signals?date=<today-WIB>` queryable (`[]` bukan null) | ready (plus `GET /ohlcv/:code` untuk inspeksi candle) |
| E-04 | Worker-minimal dry-run 1 simbol → log per-simbol + digest stdout | ready (`go run ./cmd/worker` butuh DB;catatan: weekend → skip) |
| Non-goals | no 20–100 load, no retry/backoff, no Telegram Send | — |

### 5.4 Gap + kalender

| ID | Item | Status |
|---|---|---|
| C-01 | `DetectGap`: tanggal trading hilang dari `History(60)` → skip `DetectOne`, `SymbolResult{Reason:"gap:YYYY-MM-DD missing"}` | done (`usecase/gap.go`, holidays param di `DailyIngest`) |
| C-02 | `Backfill` tetap idempoten (upsert menimpa; next-run sembuhkan gap bila Yahoo kirim candle hilang) | done |
| C-03 | Holiday fetch = backlog terpisah; E2E hanya di hari trading diketahui | pending |

### 5.5 Tuning loop manual

| ID | Item | Status |
|---|---|---|
| U-01 | Rule via param (`DetectOne/DailyIngest(rule)`), jangan hardcode | done |
| U-02 | Review via `signals` + `?include_filtered=true` + `Interpretation()` | done |
| U-03 | Trigger threshold didefinisikan saat E2E hijau (mis. >5 actionable/hari × 2 minggu = noisy) | pending |

### 5.6 Risiko diterima user

| ID | Risiko | Mitigasi | Status |
|---|---|---|---|
| R-01 | DB-down jam 16:30 = miss total | worker-minimal exit non-zero agar cron alert | pending |
| R-02 | Auto-migrate concurrent api+worker race | E2E sekuensial dulu | pending |
| R-03 | 1 req/s × 100 ≈ 100s | ukur latency + 429 saat E2E, tanpa retry dulu | pending |
| R-04 | README/Dockerfile basi | sinkron saat worker-minimal mendarat | pending |

## 6. Naming cleanup 2026-09-26 (sudah diterapkan, gate hijau)

| Layer | Sebelum → Sesudah | Status |
|---|---|---|
| domain | `PriceFilterEnabled` → `IsPriceFilterEnabled`; test `in/want` → `input/expected` | done |
| usecase | `DetectOne` return anonim → `(sig, spike, isFiltered, err)`; `SymbolResult.Spike` → `HasSpike`; `mult` → `multiple`; `z/pct` → `zScore/pctChange`; `AccumulationDistributionLine` → `ADL`; `...Slope5` → `ADLSlope5`; `r/store` → `repos`; boundary table → `input*/expected*` | done |
| infra/http | `Notifier.Enabled()` → `IsEnabled()`; `config.Config` → `Settings`; `infra.Repos` → `Repositories`; `http.Deps` → `Dependencies`; `chartResp/searchResp` → `*Response`; `f64/i64` → `floatValue/intValue`; `includeFiltered` → `shouldIncludeFiltered`; error `HTTP/BOT_TOKEN` → lowercase; migrate error → prefix `postgres:` | done |
| docs hutang | `AGENTS.md` §6 kini tulis nama benar `ADL`/`ADLSlope5` (sinkron 2026-09-27) | done |

## 7. Backlog terurut

| Prioritas | Item | Status |
|---|---|---|
| 1 | E2E cloud + smoke Yahoo nyata | blocked (butuh `DATABASE_URL`) |
| 2 | Seed `stocks_master` + uji 20–100 simbol + retry/backoff + ukur 429 | pending |
| 3 | `cmd/worker` v2 + IDX `GetStockSummary`/`GetBrokerSummary` + kalender penuh + Telegram E2E | pending |
| 4 | `robfig/cron` bila worker long-running | pending |
| 5 | Sinkron `README.md` + `Dockerfile` + `AGENTS.md` gotcha ADL | pending |
