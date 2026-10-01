# Implementation Report — SpikeIDX Scaffold v1

Tanggal: 2026-09-18 | Lokasi: `/home/twstdgrnge/Kuliah/project/SpikeIDX` | Go 1.25.5
Status: **scaffold compilable, `go vet` + `go test` + `go build` lolos.** Belum ada integrasi DB nyata (masih in-memory).

Dokumen acuan: `~/Documents/obsidian-vault/4 - Projects/SpikeIDX/architecture.md` dan `volume-spike-user-stories.md`.

---

## 1. Modul & struktur folder

| Item | Detail | Status |
|---|---|---|
| `go.mod` | module `spikeidx`, go 1.25, **tanpa dependency eksternal** (stdlib only) | ✅ |
| `cmd/api`, `cmd/worker` | 2 binary sesuai architecture §3 | ✅ |
| `internal/domain`, `internal/usecase` | clean-lite, tanpa import keluar | ✅ |
| `internal/infra/{yahoo,idx,memory,postgres,telegram}` | provider + repo + notifier | ✅ |
| `internal/config`, `internal/http` | env loader + handlers stdlib mux | ✅ |
| `db/migrations/001-003` | DDL Supabase-compatible | ✅ |
| `docker-compose.yml`, `Dockerfile`, `.env.example`, `.gitignore`, `README.md` | tahap 1 (Postgres polos) | ✅ |

**Review kamu:** apakah module name `spikeidx` cukup, atau mau `github.com/<user>/spikeidx` agar siap di-push? stdlib-only saya pilih agar build offline lolos — dependency (`chi`, `pgx`, `robfig/cron`) ditambah belakangan sesuai README.

---

## 2. Domain (`internal/domain/`) — US-01/02/03

- `stock.go` — `Stock{Code, YahooSymbol, Name, Sector}`, `Candle`, `OHLCV`.
- `watchlist.go` — `MaxWatchlist = 100`, `NormalizeCode()` (`bbca.jk` → `BBCA`), `YahooSymbol()` (`BBCA` → `BBCA.JK`).
- `signal.go` — `Signal` dengan field `Avg20, Multiple, ZScore, PctChange, ADL, ADLSlope5, IsFiltered`.
- `repository.go` — interface `MarketDataProvider`, `OHLCVRepository`, `SignalRepository`, `WatchlistRepository`. Usecase hanya bergantung pada ini.

**Review kamu:** konvensi volume = lembar mentah (bukan lot). Kalau mau lot BEI (1 lot = 100 lembar), konversi harus diputuskan sekarang sebelum migrasi.

---

## 3. Usecase (`internal/usecase/`) — US-05/06/07

- `spike.go` — `DefaultSpikeRule()` = `{MultipleMin: 2.0, ZScoreMin: 2.0, PctChangeMin: 2.0}` persis architecture §4.3. `Stats()` butuh ≥20 bar (oldest-first), hitung avg20/z-score/pct-change. `IsSpike()` return `(spike, filteredByPrice)` — spike yang `|change|<2%` tetap disimpan dengan flag, tidak dikirim ke Telegram.
- `adl.go` — `AccumulationDistributionLine()` (Chaikin, guard `High==Low` → MFM=0) + `AccumulationDistributionLineSlope5()`.
- `spike_test.go` — 4 test: butuh-20-row, spike tak-terfilter, spike terfilter harga, ADL naik + slope, `NormalizeCode`.

**Review kamu:** threshold 2.0/2.0/2% ini final untuk v1? `Stats()` pakai pembagi-N (populasi) untuk stddev — konsisten dengan `STDDEV_POP` vs `STDDEV` Postgres perlu disamakan saat pindah ke SQL (lihat §7).

---

## 4. Infra Yahoo (`internal/infra/yahoo/client.go`) — US-03/04

- `DailyOHLCV(BBCA.JK)` → `query1.../v8/finance/chart/` + `period1/period2` 3 bulan + `interval=1d`, header browser UA (wajib, kalau tidak 429), timeout 20s.
- `Search(query)` → `/v1/finance/search`, **filter hanya simbol `.JK`**, map ke `domain.Stock`.
- Volume dibiarkan mentah (tidak di-adjust), OHLC ter-adjust split/div — sesuai catatan architecture §8.

**Review kamu:** belum ada retry/backoff dan cookie-crumb handling (Yahoo kadang 401 tanpa crumb). Untuk 100 saham sekuensial 1 req/detik (~100 detik) masih aman — tapi perlu kamu putuskan: tambah retry sekarang atau nanti saat uji 100 simbol?

---

## 5. Infra IDX (`internal/infra/idx/client.go`) — fallback + v2

- Stub yang return error jelas (`not implemented`), agar worker/API tidak diam-diam gagal.
- Kontrak `GetStockSummary` (fallback) dan `GetBrokerSummary` (US-09 v2) didokumentasikan di komentar + architecture.

**Review kamu:** setuju fallback IDX dikerjakan setelah Yahoo terbukti stabil untuk 100 simbol?

---

## 6. Infra memory + postgres (`internal/infra/memory/`, `infra/postgres/queries.go`)

- `memory.go` — `Watchlist`, `OHLCVStore` (upsert by code+date, sort oldest-first), `SignalStore`, mutex-safe. Dipakai `cmd/api` + `cmd/worker` agar jalan tanpa DB.
- `queries.go` — SQL contract `UpsertOHLCV` (`ON CONFLICT DO UPDATE`) + `SpikeCandidates` (window function, mirror `usecase.Stats`).

**Review kamu (penting):**
1. `STDDEV()` di Postgres = sample-stddev (N-1), sedangkan Go pakai populasi (N). Pilih salah satu sebelum data campur.
2. Worker saat ini in-memory → tiap run mulai kosong. Ini disengaja untuk validasi fetch+detect; wiring `pgx` adalah next step.

---

## 7. HTTP API (`internal/http/handlers.go`) — US-01/02 + baca sinyal

| Endpoint | Perilaku |
|---|---|
| `GET /api/v1/search?q=` | Yahoo, fallback IDX bila error |
| `GET /api/v1/watchlist` | list kode |
| `POST /api/v1/watchlist {"code"}` | normalisasi, tolak bila ≥100 (`409`), trigger backfill async |
| `DELETE /api/v1/watchlist/{code}` | hapus |
| `GET /api/v1/signals?date=&include_filtered=` | baca sinyal |
| `GET /health` | `{"status":"ok"}` |

**Temuan smoke test (perlu review kamu):**
1. `GET /watchlist` kosong return JSON `null`, bukan `[]`. Sudah benar di `/signals` (`if res == nil`), tapi belum di watchlist. Fix 1 baris — mau saya samakan ke `[]`?
2. Backfill async memakai `r.Context()` — berisiko tercancel saat request selesai. Seharusnya `context.Background()`. Mau saya perbaiki sekalian?

---

## 8. Worker (`cmd/worker/main.go`) — US-04/05/08

- Skip Sabtu/Minggu, dev-default `BBCA/TLKM/BBRI` bila watchlist kosong, sleep 1 detik antar simbol (rate limit), cetak digest ke stdout + kirim Telegram bila dikonfigurasi.
- Dijalankan sekali per tick cron: `30 16 * * 1-5 TZ=Asia/Jakarta ./worker`.

**Review kamu:** kalender libur BEI belum ada (masih weekday-check). Sumber libur mau hardcode list tanggal, atau fetch dari IDX nanti?

---

## 9. Telegram (`internal/infra/telegram/notifier.go`) — US-08

- `Digest()` format `🚨 Spike 17 Sep (2): BBCA 45jt (3.1x avg20) +2.4% | ADL naik`, kosong → `No spike today`. `compact()` (jt/rb). `Send()` via Bot API, `Enabled()` guard bila token kosong.
- 2 test: digest kosong + format.

---

## 10. DB & deploy — `db/migrations/`, `docker-compose.yml`, `Dockerfile`

- `001_master.sql` — `stocks_master` + `watchlist` + `pg_trgm` index untuk search fuzzy.
- `002_ohlcv.sql` — PK `(code,date)` + index `(code,date DESC)`.
- `003_signals.sql` — hasil deteksi + `is_filtered` + index tanggal.
- Compose tahap 1: hanya `db + api + worker` (tanpa full Supabase, sesuai keputusan RAM).
- `.env.example`: `DATABASE_URL, DB_PASSWORD, TELEGRAM_BOT_TOKEN/CHAT_ID, TZ, PORT`.

---

## 11. Verifikasi yang sudah dijalankan

- `go vet ./...` ✅ | `go test ./...` ✅ (`usecase`, `telegram` ok; paket lain belum ada test — disengaja di scaffold)
- `go build ./...` ✅
- Smoke `GET /health` → `{"status":"ok"}` ✅ ; `GET /watchlist` → `null` (catatan §7)

## 12b. Update 2026-09-18 — StockRepository (review domain temuan #1)

- `domain/repository.go`: tambah `StockRepository{Ensure, Get}` + `ErrStockUnknown`.
- `infra/memory`: tambah `StockStore` (2 test baru: idempotent-refresh, unknown → `ErrStockUnknown`).
- `infra/postgres/queries.go`: tambah `UpsertStock` (`ON CONFLICT DO UPDATE`).
- Aturan **wajib search dulu**: `GET /search` cache tiap hasil via `Ensure`; `POST /watchlist` return `404` bila kode belum ada di master; worker `Ensure` tiap kode sebelum upsert OHLCV (safety net FK).
- Smoke test: `POST /watchlist BBCA` tanpa search → `404` ✅; `GET /search?q=BBCA` → `BBCA.JK / PT Bank Central Asia Tbk` ✅; `POST` ulang → `201 backfilling` ✅; `GET /watchlist` → `["BBCA"]` ✅.
- Perbaikan sambil jalan: `cmd/worker` diselaraskan ke nama fungsi asli `AccumulationDistributionLine(Slope5)`; `gofmt` bersih; `vet+test+build` lolos.

## 12c. Update 2026-09-18 — Postgres pindah full cloud

- Keputusan: batal self-host (Docker Compose lokal / Supabase self-host). DB = cloud Postgres (Supabase/Neon, Session mode 5432).
- `docker-compose.yml`: service `db` + volume `pgdata` dihapus; tersisa `api` + `worker` dengan `STRICT_DB=1`.
- `.env.example`: template URL Supabase/Neon + catatan port 5432 vs pooler 6543.
- Migrasi: kanonis di `internal/infra/postgres/migrations/` (syarat `go:embed` tanpa `..`); `db/migrations/` jadi symlink agar `psql -f` manual tetap jalan.
- `README.md`: tambah panduan cloud + auto-migrate.
- Perlu kredensial cloud untuk E2E penuh (belum dijalankan — tinggal isi `DATABASE_URL` lalu `go run ./cmd/api`, migrasi otomatis).

## 12d. Update 2026-09-18 — Migrasi HTTP ke Gin

- `gin-gonic/gin v1.12.0` ditambahkan (`go mod tidy` merapikan sisanya).
- `internal/http/handlers.go`: rewrite ke `gin.Context` (`c.Query/c.Param/c.ShouldBindJSON/c.JSON`); route daftar via `Deps.Router()` (`DELETE /watchlist/:code` ganti `PathValue` stdlib).
- Bonus dari `binding:"required"`: validasi body kosong + cek kode-spasi-saja (`NormalizeCode==""` → 400) — menutup sebagian temuan review #3.
- `cmd/api`: `deps.Router().Run(addr)` (Gin logger+recovery bawaan).
- Smoke test Gin: health ✅, watchlist `[]` ✅, 404-tanpa-search ✅, search GOTO ✅, add 201 ✅, list ✅, signals `[]` ✅, delete 204 ✅.

## 12e. Update 2026-09-25 — Reset ke domain + application layer

- Keputusan user: presentation/infra (`cmd/`, `internal/http/`, `internal/infra/`, `internal/config/`, `db/`) dihapus dari working tree agar review fokus ke `internal/domain/` + `internal/usecase/` (7 file).
- Backup reversibel di `/tmp/spikeidx-backup-20260925/` (20 file + 3 symlink `db/migrations/*`); restore = `cp -a` balik. Tidak ada yang di-commit/dihapus dari git history (file-file tsb mayoritas untracked).
- `go.mod` kembali stdlib-only via `go mod tidy`; `go.sum` kosong dihapus. Verifikasi: `go test ./...` ✅ (usecase ok, domain tanpa test), `go vet` ✅, `go build` ✅, `gofmt -l` kosong ✅.
- Catatan: `docker-compose.yml`, `.env.example`, `Dockerfile`, `README.md` (root) dibiarkan apa adanya — menunjuk ke binary `api`/`worker` yang belum dibangun ulang; akan diselaraskan saat layer delivery ditulis ulang dari usecase baru (rencana Phase B–D).
- Catatan 12:48: `AGENTS.md` + laporan ini dipindah user ke `docs/` (gitignored via `/docs` di `.gitignore`) agar tidak ikut commit.

## 12f. Update 2026-09-25 — Phase A domain hardening (temuan #2–4 tutup)

- Baru: `domain/errors.go` (`ErrStockUnknown/ErrInvalidCode/ErrWatchlistFull/ErrInsufficientData`), `domain/code.go` (`Code` + `ParseCode` enforce `^[A-Z]{3,4}$`, `MustParseCode`, `(Code).YahooSymbol()`), `domain/tradingdate.go` (`TradingDate` midnight WIB + `Parse`/`New`/fallback FixedZone bila tzdata hilang), `domain/rule.go` (`SpikeRule` pindah dari usecase + `PriceFilterEnabled` untuk US-06 on/off).
- `Signal`: tambah `IsActionable()` (= `!IsFiltered`) dan `Interpretation()` (`akumulasi/distribusi/netral/terfilter`, US-07). `Stock.Code`, `OHLCV.Code`, `Signal.Code` kini bertipe `Code`.
- Interface bersih (belum ada implementasi tersisa, jadi bebas ubah): `DailyOHLCV(ctx, Code)`, `WatchlistRepository` pakai `UserID` + `DefaultUser` (bunuh magic string `"default"`), `ByDate(ctx, TradingDate, ...)`.
- `usecase`: `SpikeRule` jadi alias `domain.SpikeRule` (call site lama kompilasi), `IsSpike` hormati `PriceFilterEnabled=false`, komentar `Stats` kunci keputusan stddev populasi + baseline include-today.
- Test baru: `code_test` (13 kasus valid/invalid), `tradingdate_test` (round-trip, reject, flip-tanggal UTC→WIB), `signal_test` (actionable/interpretasi/default rule), `TestIsSpikeFilterDisabled`. Gate: `test` ✅ `vet` ✅ `build` ✅ `gofmt` ✅.

## 12g. Update 2026-09-25 — Phase B usecase orkestrasi + rule di dalam fungsi

- Keputusan user: rule diinstansiasi di dalam fungsi agar konsisten. `Stats(hist)` (param rule memang tak terpakai → hapus) dan `IsSpike(multiple, z, pct)` (pakai `domain.DefaultSpikeRule()` di dalam). Konsekuensi: `TestIsSpikeFilterDisabled` dihapus; on/off filter (US-06) jadi pekerjaan config global nanti, bukan param per-call.
- Baru di `internal/usecase/`: `watchlist.go` (`AddToWatchlist` validasi→search-first→cap-100→add, return `Code` siap backfill; `RemoveFromWatchlist`), `search.go` (`SearchAndCache` primary→fallback→Ensure, jamin `[]` bukan nil), `backfill.go` (`Backfill` fetch→map→upsert, return jumlah baris), `detect.go` (`DetectOne` Stats→IsSpike→ADL→Signal + `ErrInsufficientData`; `DailyIngest` per-kode Ensure-seperlunya→Backfill→History(60)→DetectOne→Upsert bila spike, return `[]SymbolResult` untuk log `success/failed+reason` US-04), `calendar.go` (`IsTradingDay` weekend + peta libur).
- Test dengan fake repo/provider (`fakes_test.go`): watchlist 404/400/full-100/remove, search fallback + both-fail + empty-non-nil, backfill mapping + error-429, `DailyIngest` 3 simbol (spike / no-spike / fetch-error) + upsert-hanya-spike + Ensure otomatis, kalender weekend/libur. Gate: `test` ✅ `vet` ✅ `build` ✅ `gofmt` ✅.
- Sisa ke Phase B: tidak ada — handler/worker tinggal jadi pemanggil tipis saat layer delivery dibangun ulang (sleep 1 req/detik dan cron tetap di worker, bukan usecase).

## 12h. Update 2026-09-25 — Koreksi: rule kembali editable + panduan fungsi kritis

- Koreksi atas §12g: user mengembalikan param `rule` di `Stats`/`IsSpike` agar threshold bisa diedit. `DetectOne`/`DailyIngest` kini menerima `rule` dan meneruskannya ke bawah; test memakai `domain.DefaultSpikeRule()`, dan `TestIsSpikeFilterDisabled` kembali lolos. On/off filter (US-06) hidup lagi via `PriceFilterEnabled`.
- Penjelasan tiap fungsi kritis:
  - `domain.ParseCode(input)` — normalisasi (`bbca.jk` → `BBCA`) lalu cocokkan `^[A-Z]{3,4}$`; gagal → `ErrInvalidCode`. Satu-satunya pintu masuk kode saham, jadi format salah tertolak sebelum menyentuh repo.
  - `domain.NewTradingDate(t)` / `ParseTradingDate(s)` — paksa tiap tanggal ke tengah malam WIB. Penting karena Yahoo mengirim timestamp UTC: misal 18 Sep 18:00 UTC = 19 Sep 01:00 WIB, tanggal kalendernya beda. Tanpa ini sinyal bisa tercatat di hari yang salah.
  - `domain.DefaultSpikeRule()` — angka keramat v1: volume `>2x` rata-rata-20, z-score `>2.0`, filter `|change|>2%`, filter aktif. Satu-satunya tempat threshold didefinisikan.
  - `usecase.Stats(hist)` — ambil 20 bar terakhir (hist harus oldest-first), hitung rata-rata, simpangan baku populasi (bagi N, sejajar `STDDEV_POP` Postgres), lalu `multiple = vol/avg`, `z = (vol-avg)/std`, `pct` dari close kemarin. Guard pembagi-nol: avg/std/close nol → hasil nol, bukan crash. Butuh ≥20 bar, kalau tidak `ok=false`.
  - `usecase.IsSpike(m, z, pct, rule)` — inti deteksi: `m>2 && z>2` → spike; kalau `|pct|<2%` → spike tapi `filtered=true` (tetap disimpan, tidak dikirim ke Telegram). Batas pakai `>` ketat, jadi angka pas 2.0 bukan spike.
  - `usecase.AccumulationDistributionLine(hist)` — Chaikin A/D: tiap bar dihitung Money Flow `((C-L)-(H-C))/(H-L) × Volume`, lalu diakumulasi. Guard `High==Low` → flow 0 (hindari bagi-nol). `...Slope5` = ADL hari ini minus 5 hari lalu; positif artinya akumulasi.
  - `Signal.IsActionable()` / `Interpretation()` — `IsActionable` = tidak terfilter (layak masuk digest Telegram); `Interpretation` menerjemahkan slope ADL jadi `akumulasi/distribusi/netral/terfilter` (US-07).
  - `usecase.DetectOne(code, hist, rule)` — rangkai semuanya untuk 1 saham: Stats → IsSpike → ADL → rakit `Signal`. Hist pendek → `ErrInsufficientData`; tidak spike → return kosong tanpa error.
  - `usecase.DailyIngest(ctx, provider, repos, codes, rule)` — pipeline harian per kode: `Get` master (kalau asing, `Ensure` nama seadanya) → `Backfill` → ambil 60 bar → `DetectOne` → simpan sinyal hanya bila spike. Tiap kode menghasilkan `SymbolResult{Spike, Signal, Reason}` sehingga worker bisa log `success/failed+reason` per simbol (US-04).
  - `usecase.AddToWatchlist(ctx, stocks, wl, input)` — parse kode → wajib sudah ada di master (`ErrStockUnknown` = pesan "search dulu") → cek cap 100 (`ErrWatchlistFull`) → simpan. Return `Code` yang tervalidasi agar pemanggil bisa langsung backfill.
  - `usecase.SearchAndCache(...)` — cari ke provider utama, gagal → fallback; tiap hasil di-`Ensure` ke master; jamin return `[]` bukan nil.
  - `usecase.Backfill(...)` — tarik candle → petakan ke baris OHLCV → upsert; return jumlah baris. Idempoten (tulis ulang tanggal sama menimpa, bukan duplikat).
  - `usecase.IsTradingDay(date, holidays)` — tolak Sabtu/Minggu + tanggal di peta libur BEI; dipakai worker sebelum fetch.
- Gate: `test` ✅ `vet` ✅ `build` ✅ `gofmt` ✅.

## 12i. Update 2026-09-25 — Phase C: kunci semantik + boundary

- `Stats` kini menyortir salinan histori (oldest-first) di dalam fungsi — provider yang mengembalikan urutan acak tidak lagi menghasilkan sinyal dari bar yang salah. Berlaku juga di `DetectOne` (rakitan `Signal` + deret ADL ikut terurut). Kontrak oldest-first tetap, tapi pelanggaran kontrak tidak lagi silent.
- Boundary tests (`spike_boundary_test.go`): `multiple`/`z` pas 2.0 → bukan spike; `|pct|` pas 2.0 → spike tanpa filter; 21 bar → ok; volume+close nol → nol tanpa NaN/crash; histori acak → sinyal tetap dari bar terbaru yang benar.
- `Backfill` normalisasi tanggal ke WIB (`domain.NewTradingDate(c.Date).Time()`): dua candle UTC yang jatuh di hari WIB yang sama kini collapse ke satu bar (benar untuk data EOD), sinyal tak lagi tercatat di tanggal yang salah. Test: candle 18 Sep 18:00 UTC tersimpan sebagai 19 Sep WIB.
- Sisa Phase C: penyamaan `STDDEV` → `STDDEV_POP` di query Postgres — menunggu infra dibangun ulang (backup query di `/tmp/spikeidx-backup-20260925/`).
- Gate: `test` ✅ `vet` ✅ `build` ✅ `gofmt` ✅.

## 12j. Update 2026-09-25 — Rebuild infra (tanpa memory, kontrak baru)

- Keputusan: memory dibuang — backend satu-satunya Postgres via `WireStrict` (gagal keras tanpa DB, tanpa fallback diam-diam). `go.mod` + `pgx/v5` (satu-satunya dep luar baru).
- `infra/yahoo`: `DailyOHLCV(ctx, Code)` (simbol `.JK` via `code.YahooSymbol()`), decode JSON dipisah (`decodeChart`/`decodeSearch`) agar unit-testable; search menolak kode tak-valid via `ParseCode`. Test fixture + live-test opsional (`SPIKEIDX_LIVE_YAHOO=1`).
- `infra/telegram`: logika arah ADL diganti `s.Interpretation()`; format pesan jadi `| ADL akumulasi/distribusi` (bahasa US-07, sebelumnya `naik/turun`).
- `infra/postgres`: DDL 001–003 disalin mentah; 4 repo diadaptasi (`string↔Code`, `date.String()`, `string(user)`, History DESC-reverse); `SpikeCandidates` kini `STDDEV_POP` (tutup hutang Phase C); symlink `db/migrations` dipulihkan.
- `infra/wire.go` strict-only; `internal/config` minimal (tanpa env rule); `infra/idx` stub signature-baru (implementasi asli setelah Postgres hijau).
- Contract suite (`postgres/contract_test.go`, skip tanpa `DATABASE_URL`): Ensure→Get, unknown→`ErrStockUnknown`, watchlist idempoten, upsert-timpa + oldest-first + limit, signal update + filter on/off, remove. **E2E cloud BELUM jalan** — butuh `DATABASE_URL`; jalankan manual: `DATABASE_URL=... go test -run TestContract ./internal/infra/postgres/`.
- Gate lokal: `test` ✅ `vet` ✅ `build` ✅ `gofmt` ✅ (contract skip).

## 12k. Update 2026-09-26 — Sinkron AGENTS ke realita strict-Postgres

- `docs/AGENTS.md` §1–§7 diselaraskan: Gin/memory/`Wire`/fallback/`STRICT_DB` dihapus dari deskripsi; backend satu-satunya `WireStrict`; kontrak §4 poin 4–6 ditandai SELESAI (STDDEV_POP, WIB, TradingDate+ParseCode); §5 tanpa `cmd/` (TBD sampai rebuild delivery); backlog ditulis ulang (delivery → E2E cloud → seed+retry → IDX → libur/cron → README/Dockerfile).
- Temuan sinkron: `STRICT_DB` di compose/`.env.example` adalah no-op (tidak dibaca `config`) → dihapus; `/docs` di-un-ignore agar AGENTS + laporan ikut commit; `README.md`/`Dockerfile` dibiarkan basi dan ditandai TBD agar diff kecil.
- Gate: `test` ✅ `vet` ✅ `build` ✅ `gofmt` ✅ (contract skip tanpa `DATABASE_URL`).

## 12l. Update 2026-09-26 — Presentation layer Gin + `cmd/api` (worker v2 ditunda)

- Keputusan user: handler pakai Gin (`go get github.com/gin-gonic/gin v1.12.0`); worker ditunda ke v2 agar fokus integrasi Yahoo; watchlist kosong (worker v2) → exit 0; `GET /signals` tanpa `date` → hari ini WIB. Rencana dicatat di `docs/implementation_plan.md`.
- Baru: `internal/http/router.go` (`Deps.Router() *gin.Engine`, `gin.New()` + `Recovery`) + `handlers.go` (health/search/watchlist CRUD/signals; map `ErrInvalidCode→400`, `ErrStockUnknown→404`, `ErrWatchlistFull→409`, provider-gagal→502; backfill async via `context.WithoutCancel`; list selalu `[]`).
- Baru: `cmd/api/main.go` (`config.Load` → `WireStrict` → Yahoo primary + IDX fallback → `Router().Run(":"+Port)`).
- Test baru `internal/http/handlers_test.go` (13 kasus, `gin.TestMode`, fake repo lokal): health, `[]`-non-nil, 404/400/409, delete 204, search-cache + fallback + 502, signals default-date/`[]`.
- Docs: `AGENTS.md` §1–§3/§5/§7, `README.md` (memory/`STRICT_DB` basi dibersihkan), plan §1/§2.1 → Gin.
- Diketahui: `Dockerfile` target `./cmd/worker` gagal sampai v2 (build image penuh TBD). E2E cloud + smoke Yahoo nyata menunggu `DATABASE_URL`.
- Gate: `test` ✅ `vet` ✅ `build` ✅ `gofmt` ✅ (contract skip tanpa `DATABASE_URL`).

## 12m. Update 2026-09-26 — Linter + formatter

- Baru: `.golangci.yml` (v2: formatters gofmt/gofumpt/goimports + linters errcheck/govet/ineffassign/misspell/staticcheck/unconvert/unused) + `Makefile` (`test/vet/build/fmt/lint/run-api`).
- Temuan & perbaikan: 4× `defer Close()` → `defer func(){ _ = ... }()` (+ grup import `goimports -local spikeidx`); 22× `_ =` di test → cek error via `t.Fatal`; `detect.go` fail-fast bila `Ensure` gagal; `db.go` migrasi diekstrak ke `applyMigration` (`errors.Join`, sekaligus hapus `defer` di dalam loop).
- Misteri 3 file "gofumpt": akar masalah = `module-path` kosong di config → gofumpt mengira `spikeidx/...` stdlib (aturan `joinStdImports`). Terbukti via eksperimen library (`ModulePath=""` kotor, `="spikeidx"` bersih). Fix: `formatters.settings.gofumpt.module-path: spikeidx` + `lang-version: "1.25"`. (Catatan: cache golangci sempat sajikan temuan basi — `cache clean` dulu bila hasil aneh.)
- Gate: `golangci-lint run` **0 issues** (v2.11.3 sistem + v2.14.0) ✅ `test` ✅ `vet` ✅ `build` ✅ `gofmt` ✅.

## 12n. Update 2026-09-27 — Endpoint `GET /api/v1/ohlcv/:code` (candle observable)

- Baru: handler `ohlcv` (`ParseCode`→400; `limit` default 60, cap 500, invalid→400; `History` → DTO tanggal WIB; kosong→`[]`) + route + 7 test baru (rows oldest-first, `[]`, 400 kode/limit, default 60, cap 500) + `openapi.yaml` (tag `ohlcv`, path, schema `OHLCV`) + assert spec di `swagger_test.go`.
- Perbaikan sambil jalan: kembalikan field `Code` di `signalResponse` (termakan edit) + receiver `*Deps`→`*Dependencies` (ikut rename swagger); hapus trailing-whitespace di `config.go` (`gofmt` bersih lagi).
- Terjawab: candle Yahoo memang sudah bisa di-fetch (unit+live test), tapi baru sekarang observable via API — smoke E2E (search→watchlist→backfill→`GET /ohlcv/BBCA`) menunggu run dengan `DATABASE_URL` user.
- Gate: `test` ✅ `vet` ✅ `build` ✅ `gofmt` ✅ `lint` menyusul.

## 12o. Update 2026-09-27 — `DetectGap` + worker-minimal (G-02/G-05/C-01/M-01–M-03 done)

- Baru: `usecase/gap.go` — `DetectGap(hist, isTradingDay)` (filter baris non-trading → sort copy → cek consecutive; `nextTradingDay` cap 32 langkah anti-hang) + 8 test (`gap_test.go`: clean-week, weekend-skip, Tue-hilang, holiday-map, kosong/1-baris, unsorted, kalender-macet, integrasi `DailyIngest` reason `gap:2026-09-22 missing`).
- `DailyIngest` terima `holidays map[TradingDate]bool`, cek gap sesudah `History(60)` sebelum `DetectOne`; skip → `Reason` eksplisit, tanpa tulis sinyal.
- Temuan besar: `map[TradingDate]bool` **tidak pernah match** lintas konstruksi — `LoadLocation` kembalikan pointer baru tiap panggil (terbukti via probe: instant sama, map-hit false). Artinya cek libur `IsTradingDay` selama ini dead-code. Fix: `wibLocation` singleton di `tradingdate.go` + test regresi map-key di domain. Batasan tersisa: `holidays=nil` → libur panjang terbaca gap (fail-closed, G-06 menyusul).
- Baru: `cmd/worker/main.go` (minimal, M-02: tanpa cron/`Send`) — `WireStrict` → skip weekend → skip watchlist kosong → `DailyIngest` per-kode + sleep 1s → log per-simbol → digest ke stdout. `go build ./...` hijau → hutang Dockerfile (M-03) tertutup di level compile.
- Status plan: G-02/G-05/C-01/M-01–M-03 done; E-01–E-04 ready (butuh run manual + `DATABASE_URL`); G-01/G-06 pending.
- Review pasca-batch (No, with fixes → semua diperbaiki): Critical `.gitignore: worker` membayangi `cmd/worker/` (tak terlacak git!) → jadi `/worker`; Important blast-radius holidays-nil → worker log batasan saat startup; Minor: guard baris-duplikat di `DetectGap`, `limit=0` → 400 (bukan unlimited), digest via stdout (`fmt`), tail-staleness dicatat backlog.
- Gate: `test` ✅ `vet` ✅ `build` ✅ `gofmt` ✅ `lint` 0 issues ✅.

## 12p. Update 2026-10-02 — Label akumulasi/distribusi rasio-CMF + penyelarasan pasca-rebase

- Masalah: `Interpretation()` lama (`ADLSlope5 >0 → akumulasi`) melabel 1 bar jumbo sebagai akumulasi. Spec Wyckoff Range+Spring 2026-09-28 ditolak sebagai terlalu kompleks.
- Baru (desain disetujui user): `SpikeRule` tambah `ADLSlopeWindow=20` + `ADLSlopeMinRatio=0.10`; label = rasio `slope/(avg20*window)` — `>+0.10 akumulasi`, `<-0.10 distribusi` (simetris), sisanya `netral`; `avg<=0`/histori pendek → netral; `IsFiltered` prioritas. L0 tidak berubah (`2x + z>2 + |change|>2%`).
- `usecase/adl.go`: `ADLSlope(adl, window)` generik (`ADLSlope5` jadi wrapper); `DetectOne` pakai window dari rule. Contoh: slope Rp40jt, avg Rp10jt, window 20 → rasio 0.20 → akumulasi; slope Rp5jt → 0.025 → netral.
- Interlude rebase (ringkas): tier `akumulasi kuat/lemah + regime` + `MultipleMin 1.5` + filter-off sempat mendarat, lalu dikembalikan ke desain rasio + test diselaraskan (boundary `2.0`, sideways → terfilter, digest `ADL akumulasi/distribusi`).
- Dipertahankan non-destruktif: `CMF()` + kolom `cmf` (migrasi 004) + field API + `HasData`/status non-spike worker; deskripsi `openapi.yaml` diluruskan ke desain rasio.
- Gate: `test` ✅ `vet` ✅ `build` ✅ `gofmt` ✅ `lint` 0 issues ✅.

## 12. Usulan next step (pilih urutan)

1. E2E cloud via contract test (`DATABASE_URL` user) + smoke Yahoo nyata (search → watchlist → backfill).
2. Seed `stocks_master` + uji fetch Yahoo untuk 20–100 simbol + ukur 429 + retry/backoff.
3. `cmd/worker` v2 + IDX `GetStockSummary` fallback + `GetBrokerSummary` v2 + kalender libur penuh + Telegram E2E.
