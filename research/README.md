# SpikeIDX Research (Python)

Lapisan riset untuk menguji apakah sinyal SpikeIDX punya daya prediksi:
event study, forward return, dan eksperimen parameter. Paket ini **tidak**
menulis ke database; Go (`cmd/worker`) tetap satu-satunya penulis `ohlcv` dan
`signals`.

## Setup

```bash
cd research
uv sync                 # buat .venv + install dependensi (termasuk grup dev)
uv run pytest           # test
uv run ruff check .     # lint
uv run jupyter lab notebooks/
```

Atau dari root repo: `make py-sync py-test py-lint py-notebook`.

Koneksi DB memakai `DATABASE_URL` yang sama dengan Go (dibaca dari `../.env`).

## Struktur

```
research/
  pyproject.toml
  spikeidx_research/   # fungsi reusable + teruji (load data, fitur, return, statistik)
  tests/               # pytest; fixture paritas dibaca dari ../testdata/
  notebooks/           # eksplorasi + laporan; logika berulang pindah ke paket
```

## Aturan

- Logika yang dipakai lebih dari satu notebook wajib pindah ke `spikeidx_research/` dan punya test.
- Rumus yang juga ada di Go wajib lolos test paritas (`testdata/parity_*`).
- Tanpa look-ahead: fitur di hari `t` hanya boleh memakai bar `<= t`, dan return hanya memakai bar `> t`.

Rencana kerja: `docs/superpowers/plans/2026-11-25-quant-research-layer.md` (lokal).
