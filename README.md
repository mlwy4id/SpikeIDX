# SpikeIDX — deteksi dan riset volume spike saham Indonesia (EOD)

Pipeline End-of-Day untuk saham IDX: ambil OHLCV dari Yahoo (`.JK`), simpan ke Postgres,
deteksi volume spike dan aliran dana (CMF/ADL), lalu kirim digest Telegram. Lapisan riset
Python menguji apakah sinyal tersebut punya daya prediksi.

| Bagian | Teknologi | Peran |
|---|---|---|
| Produksi | Go 1.25, pgx, Gin | API, worker EOD, satu-satunya penulis DB |
| Riset | Python ≥3.12 (`uv`), pandas, scipy, JupyterLab | Event study/backtest, read-only ke DB |
| DB | Postgres cloud (Supabase/Neon) | Dipakai bersama |

## Layout

```
cmd/api                REST (Gin): search, watchlist, signals, ohlcv, Swagger
cmd/worker             single-run EOD: ingest → deteksi → digest Telegram
internal/domain        entity + interface repo (stdlib only)
internal/usecase       logika murni: spike, ADL, CMF, gap, kalender, ingest
internal/http          handler tipis + openapi.yaml
internal/infra         yahoo (retry 429/5xx), idx (kalender Nager; fallback stub),
                       postgres (pgx + auto-migrate), telegram
scripts/               seed_stocks, fetch_probe
research/              paket Python spikeidx_research, tests, notebooks
```

## Rule sinyal

- Spike: `volume / avg20 > 2.0` **dan** `z-score > 2.0`.
- Terfilter (noise): spike dengan `|perubahan harga| < 2%`; disimpan, tidak masuk digest.
- Label: `akumulasi` / `distribusi` / `netral` dari aliran dana 20 hari.

## Run lokal (Go)

```bash
cp .env.example .env    # isi DATABASE_URL (Session mode port 5432, ?sslmode=require)
make test vet build lint
go run ./cmd/api        # :8080, gagal keras tanpa DB; migrasi otomatis
go run ./cmd/worker     # satu kali EOD; skip weekend/libur/watchlist kosong
```

Cron produksi (eksternal): `30 16 * * 1-5` dengan `TZ=Asia/Jakarta`. Jalankan api dan
worker berurutan, jangan bersamaan, agar migrasi tidak balapan.

Alur wajib search dulu: `GET /api/v1/search?q=BBCA` → `POST /api/v1/watchlist {"code":"BBCA"}`
→ backfill jalan di background.

## Riset (Python)

```bash
make py-sync            # uv sync di research/
make py-test py-lint    # pytest + ruff
make py-notebook        # JupyterLab
```

Detail: [`research/README.md`](research/README.md).

## API

- `GET /health`
- `GET /api/v1/search?q=bank`
- `GET /api/v1/watchlist` · `POST /api/v1/watchlist {"code":"BBCA"}` · `DELETE /api/v1/watchlist/BBCA`
- `GET /api/v1/signals?date=2026-11-24&include_filtered=false`
- `GET /api/v1/ohlcv/BBCA?limit=60` (oldest-first, cap 500)

Swagger UI: `GET /swagger/index.html`; spec: `GET /openapi.yaml`.

## Postgres cloud

1. Buat project Postgres (region terdekat, mis. Singapore).
2. Pakai connection string **Session mode port 5432**. Transaction pooler 6543 bentrok
   dengan prepared statement pgx (`42P05`).
3. Isi `DATABASE_URL` di `.env`. Migrasi diterapkan otomatis saat startup.

## Roadmap

1. Kebenaran sinyal: guard bar basi, baseline volume tanpa bar hari ini, label dari CMF.
2. Backfill histori ≥3 tahun untuk riset.
3. Event study: forward return 1/5/20 hari, spike vs non-spike, dengan uji statistik.
