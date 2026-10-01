# AGENTS.md — SpikeIDX

Repo: Go monolith modular (binary `api` + `worker`-minimal jadi; worker-penuh/cron v2 menyusul). Baca file ini dulu sebelum mengubah apa pun.

## 1. Gambaran proyek

Deteksi **volume spike saham Indonesia (IDX/BEI), End-of-Day**. Aturan spike v1:
`volume > 2x SMA20` AND `z-score > 2.0`, filter noise `|change| > 2%` (flag `is_filtered`, tetap disimpan).
Indikator konfirmasi: Chaikin A/D Line (murni dari OHLCV, tanpa fetch tambahan).
Label: rasio `slope/(avg20*window)`, window default 20, `>+0.10 akumulasi`, `<-0.10 distribusi`, sisanya `netral`.
Alert: 1 pesan Telegram/hari via worker cron `30 16 * * 1-5 TZ=Asia/Jakarta`.

Dokumen sumber kebenaran:
- `~/Documents/obsidian-vault/4 - Projects/SpikeIDX/architecture.md` — keputusan arsitektur
- `.../volume-spike-user-stories.md` — US-01 s.d. US-09
- `docs/IMPLEMENTATION_REPORT.md` — log perubahan per fitur (§12b–12k)

Status kini: domain + usecase + infra Postgres/Yahoo/Telegram + delivery HTTP (`internal/http` Gin + `cmd/api`) + `cmd/worker`-minimal sudah jadi (§12e–12o);
worker-penuh (cron + Telegram `Send` + kalender libur fetch) menyusul (lihat `docs/IMPLEMENTATION_PLAN.md` §5).

## 2. Stack & dependensi

- Go 1.25, module `spikeidx`. Dependensi luar: `jackc/pgx/v5` (Postgres) + `gin-gonic/gin` (HTTP). Selain itu **stdlib only** — jangan tambah dependency tanpa alasan kuat.
- DB: **cloud Postgres** (Supabase/Neon, Session mode port 5432). Backend satu-satunya via `infra.WireStrict` — **gagal keras tanpa `DATABASE_URL` yang reachable, tanpa fallback diam-diam**. (`STRICT_DB`/fallback-memory sudah dihapus karena no-op: tidak ada yang membacanya.)
- Sumber data: Yahoo Finance `.JK` utama (`BBCA.JK`), IDX `idx.co.id` fallback (masih stub signature-baru).

## 3. Struktur & clean-lite

```
internal/domain/ (entities + repo interfaces, tanpa import luar;
  code.go, errors.go, tradingdate.go, rule.go, stock/watchlist/signal)
internal/usecase/ (pure, wajib ada test;
  spike/adl + watchlist/search/backfill/detect/calendar/gap)
internal/infra/{yahoo,idx,postgres,telegram} | internal/infra/wire.go (WireStrict saja)
internal/http/ (Gin handlers, tipis: validasi + panggil usecase; `Dependencies.Router() *gin.Engine`)
cmd/api/ (WireStrict → Yahoo primary + IDX fallback → Gin `:PORT`)
cmd/worker/ (minimal: single-run EOD ingest + digest stdout; tanpa cron/`Send`; holidays `nil` sampai G-06)
internal/config/ (minimal: DATABASE_URL, Telegram, TZ, PORT — tanpa env rule)
internal/infra/postgres/migrations/ (DDL kanonis; manual: `psql -f internal/infra/postgres/migrations/001_master.sql`)
```

Aturan lapis: `usecase` tidak boleh import `infra`/`http`. Semua detail teknis (provider/DB/notifier) di-inject via interface `domain`.

## 4. Kontrak penting (jangan dilanggar diam-diam)

1. **Wajib search dulu**: `POST /watchlist` return 404 bila kode belum ada di `stocks_master` (di-cache oleh search via `Ensure`). Worker/`DailyIngest` `Ensure` tiap kode sebelum tulis OHLCV (safety net FK). Implementasi aturan di `usecase.AddToWatchlist` (`ErrStockUnknown`).
2. **Volume = lembar mentah** (bukan lot). OHLC Yahoo ter-adjust split/div, volume tidak.
3. **History oldest-first** di semua repo (`History` Postgres query DESC lalu reverse — samakan bila ganti implementasi). `Stats`/`DetectOne` menyortir salinan di dalam fungsi, jadi provider nakal tidak lagi silent-salah.
4. **Stddev populasi (bagi N)** — SELESAI (§12j): Go `usecase.Stats` sejajar `STDDEV_POP` Postgres (`queries.go`). Jangan kembalikan ke `STDDEV()` (sampel, N-1).
5. **Tanggal WIB** — SELESAI (§12f/12i): `domain.NewTradingDate`/`ParseTradingDate` paksa midnight WIB; `Backfill` menormalisasi candle UTC ke WIB sebelum upsert.
6. **`ByDate(ctx, TradingDate, ...)`** + validasi kode `ParseCode` (`^[A-Z]{3,4}$`) — SELESAI (§12f). Jangan pakai string `YYYY-MM-DD` implisit lagi.
7. **Rate limit Yahoo**: sekuensial 1 req/detik + header browser UA wajib (kalau tidak: 429). Belum ada retry/backoff — tetap berlaku untuk worker baru nanti.
8. **Backfill async** (saat delivery dibangun): jalan dengan context independen (`context.Background`/`WithoutCancel`) — jangan pakai request context yang bisa tercancel.

## 5. Perintah

```bash
go test ./... && go vet ./... && go build ./...   # wajib hijau sebelum selesai
gofmt -l . && golangci-lint run ./...             # format + lint wajib bersih (config: .golangci.yml)
make test vet build fmt lint                      # sama via Makefile
cp .env.example .env  # isi DATABASE_URL cloud (?sslmode=require)
DATABASE_URL=... go test -run TestContract ./internal/infra/postgres/  # E2E cloud
go run ./cmd/api      # :8080, gagal keras tanpa DB (WireStrict)
```

Smoke: `GET /health` → `{"status":"ok"}`; `POST /api/v1/watchlist {"code":"X"}` tanpa search → 404.
`go run ./cmd/worker` = single-run EOD (skip bila weekend/watchlist kosong; butuh DB).

## 6. Gotcha yang sudah pernah menggigit

- `go:embed` **tidak boleh** path `..` (FS jadi kosong diam-diam) — migrasi harus di dalam `internal/infra/postgres/migrations/`.
- `docker compose` plugin tidak tersedia di semua mesin — `docker-compose.yml` hanya untuk `api`+`worker` (tanpa service db).
- `pkill -f <nama>` pernah menggantung shell session — bunuh proses via pid file (`kill $(cat /tmp/api.pid)`).
- Nama fungsi ADL yang benar: `ADL` / `ADLSlope(adl, window)` / `ADLSlope5` wrapper (bukan `AccumulationDistributionLine`). Kolom `adl_slope5` dipakai ulang untuk window rule (legacy name, tanpa migrasi).
- `GET /watchlist` dan `/signals` harus return `[]`, bukan `null` (guard `nil` di handler/repo — berlaku lagi saat delivery dibangun; `SearchAndCache`/`DailyIngest` sudah jamin non-nil).
- `STRICT_DB` / fallback-memory di compose/`.env.example` lama adalah no-op (tidak dibaca `config`) — sudah dihapus. Jangan perkenalkan lagi.
- `docs/` kini terversioning (baris `/docs` di `.gitignore` dihapus) — `AGENTS.md` + report wajib ikut commit.
- `TradingDate` hanya `==`-comparable bila berbagi satu `*Location` — `wib()` singleton di `tradingdate.go`; jangan panggil `LoadLocation` per konstruksi (map holiday lookup diam-diam gagal).
- `DATABASE_URL` wajib Session mode 5432: pooler transaksi 6543 + pgx prepared-statement cache = `42P05 already exists` saat startup. Hardening simple-protocol belum diputuskan.

## 7. Backlog (urut saran)

1. Worker-penuh: cron + Telegram `Send` + kalender libur fetch (G-06) di atas worker-minimal — watchlist kosong → exit 0 (sudah). Nanti: tail-staleness check ( histori berhenti beberapa hari tapi consecutive → flag juga).
2. E2E cloud (butuh `DATABASE_URL` user) — migrasi otomatis + smoke Yahoo nyata (search → watchlist → backfill), verifikasi via contract test.
3. Seed `stocks_master`, uji 20–100 simbol, retry/backoff Yahoo + ukur 429.
4. IDX `GetStockSummary` fallback + `GetBrokerSummary` v2 (hanya untuk saham yang spike).
5. Kalender libur BEI penuh, `robfig/cron` bila worker jadi long-running.
6. Sinkron ulang `README.md` + `Dockerfile` saat delivery jadi (keduanya basi).
