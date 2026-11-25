# SpikeIDX — riset volume spike saham Indonesia (EOD)

Apakah volume spike di saham IDX punya daya prediksi, dan dengan rule seperti apa?
SpikeIDX mengambil OHLCV harian dari Yahoo (`.JK`), menyimpannya di Postgres, menghitung
sinyal volume dan aliran dana (CMF), mengirim digest Telegram harian, dan menyediakan
**lab riset di terminal** untuk mengubah rule lalu menguji hasilnya dengan event study.

```
Yahoo ──► worker (Docker, tiap 3 jam) ──► Postgres ──► TUI lab riset (laptop)
                    │                                    rule → event study → compare
                    └──► Telegram (1 digest per tanggal bursa)
```

Semua rumus ada di satu modul (`src/spikeidx/signals.py`) dan dipakai bersama oleh worker,
event study, dan TUI.

## Rule sinyal

| Bagian | Definisi |
|---|---|
| Baseline volume | rata-rata dan std (populasi) **20 bar sebelum** hari yang dinilai |
| Spike | `volume / avg20 > 2.0` **dan** `z-score > 2.0` (baseline flat → z = 0) |
| Terfilter (noise) | spike dengan `\|perubahan harga\| < 2%` |
| Label | CMF 20 bar: `> 0.10` akumulasi, `< -0.10` distribusi, selain itu netral |

Semua ambang bisa diubah di TUI.

## Event study

- Entry di **open hari berikutnya** (sinyal baru diketahui setelah close); `fwd_h = close[t+h] / open[t+1] - 1`.
- Event vs kontrol (hari non-spike), Welch t-test, hit rate; h default 1/5/20 hari.
- Event dibuang bila ada lompatan harga > 35% di `[t-20, t+h]` (dugaan corporate action).
- Batasan: return bisa overlap, universe = watchlist (survivorship), sampel kecil untuk rule ketat.

## Mulai

```bash
uv sync                          # Python ≥3.12
cp .env.example .env             # DATABASE_URL (Postgres, Session mode 5432), TELEGRAM_*
uv run spikeidx migrate          # terapkan migrasi SQL yang belum tercatat
uv run spikeidx backfill --years 3          # histori untuk riset (default: seluruh watchlist)
uv run spikeidx tui              # lab riset
uv run spikeidx worker --dry-run # satu run worker tanpa tulis DB / Telegram
```

### TUI

| Tab | Isi |
|---|---|
| Lab | ubah rule + horizon → Run → tabel event study, daftar event, grafik harga/volume ±20 hari; simpan eksperimen |
| Compare | pilih 1–4 eksperimen → tabel berdampingan per horizon; peringatan bila datanya berbeda |
| Data | status data per kode, search Yahoo, tambah/hapus watchlist |

Eksperimen disimpan sebagai JSON di `~/.local/share/spikeidx/experiments/` (ubah lewat `SPIKEIDX_HOME`).

### Worker di Docker (mis. PC Windows dengan Docker Desktop)

```bash
docker compose up -d --build worker-py
```

- supercronic menjalankan `spikeidx worker` tiap 3 jam (WIB).
- Bar hari ini baru diproses setelah **18:00 WIB**. Digest dikirim **sekali per tanggal bursa**
  (dicatat di `digest_log`); kalau Telegram gagal, tanggal itu dicoba lagi di run berikutnya.
- Default `--dry-run`. Untuk mengaktifkan: `docker compose run --rm worker-py spikeidx migrate`,
  isi `WORKER_ARGS=` (kosong) di `.env`, lalu `docker compose up -d worker-py`.
- Di Windows: aktifkan *Start Docker Desktop when you sign in* dan matikan sleep.

## Pengembangan

```bash
make py-test py-lint   # pytest + ruff
make py-notebook       # JupyterLab di notebooks/
```

| Path | Isi |
|---|---|
| `src/spikeidx/signals.py` | rumus fitur, rule, label (satu sumber) |
| `src/spikeidx/research/` | forward return, event study |
| `src/spikeidx/worker.py` | satu run EOD idempoten |
| `src/spikeidx/tui/` | aplikasi Textual |
| `src/spikeidx/store.py` | akses Postgres |
| `src/spikeidx/sources/yahoo.py` | Yahoo chart/search (retry 429/5xx) |
| `testdata/` | fixture golden yang bisa dicek manual |
| `internal/infra/postgres/migrations/` | migrasi SQL (dipakai bersama Go dan Python) |

## Status migrasi dari Go

Kode Go (`cmd/`, `internal/`) masih ada sebagai sistem lama (profil compose `legacy`) selama
worker Python berjalan paralel dalam mode dry-run. Setelah worker Python mengambil alih,
kode Go dihapus dan folder migrasi dipindah ke root.
