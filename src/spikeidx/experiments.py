"""Eksperimen rule tersimpan lokal (JSON per eksperimen), bukan di Postgres."""

import hashlib
import json
import math
import os
import re
from dataclasses import dataclass
from datetime import date, datetime
from pathlib import Path

import numpy as np
import pandas as pd

from spikeidx.research.event_study import RESULT_COLUMNS
from spikeidx.signals import Rule


@dataclass
class Experiment:
    name: str
    rule: Rule
    horizons: tuple[int, ...]
    start: date
    end: date
    codes: list[str]
    data_fingerprint: str
    results: pd.DataFrame
    created_at: datetime


def default_dir() -> Path:
    home = os.environ.get("SPIKEIDX_HOME")
    base = Path(home) if home else Path.home() / ".local" / "share" / "spikeidx"
    return base / "experiments"


def fingerprint(ohlcv: pd.DataFrame) -> str:
    """Sidik data: berubah bila jumlah baris, rentang tanggal, atau daftar kode berubah."""
    parts = [str(len(ohlcv)), ",".join(sorted(ohlcv["code"].unique()))]
    if len(ohlcv):
        parts += [str(ohlcv["date"].min()), str(ohlcv["date"].max())]
    return hashlib.sha1("|".join(parts).encode()).hexdigest()[:12]


def _clean(value):
    if isinstance(value, float | np.floating) and math.isnan(value):
        return None
    if isinstance(value, np.integer):
        return int(value)
    if isinstance(value, np.floating):
        return float(value)
    return value


def _slug(name: str) -> str:
    return re.sub(r"[^a-z0-9]+", "-", name.lower()).strip("-") or "eksperimen"


def save(exp: Experiment, directory: Path | None = None) -> Path:
    directory = directory or default_dir()
    directory.mkdir(parents=True, exist_ok=True)
    results = [
        {"horizon": int(h), **{c: _clean(row[c]) for c in RESULT_COLUMNS}}
        for h, row in exp.results.iterrows()
    ]
    body = {
        "name": exp.name,
        "rule": exp.rule.to_dict(),
        "horizons": list(exp.horizons),
        "start": exp.start.isoformat(),
        "end": exp.end.isoformat(),
        "codes": exp.codes,
        "data_fingerprint": exp.data_fingerprint,
        "results": results,
        "created_at": exp.created_at.isoformat(),
    }
    path = directory / f"{exp.created_at:%Y%m%d-%H%M%S}-{_slug(exp.name)}.json"
    path.write_text(json.dumps(body, indent=2, ensure_ascii=False))
    return path


def _load(path: Path) -> Experiment:
    body = json.loads(path.read_text())
    results = pd.DataFrame(body["results"]).set_index("horizon")
    results = results.reindex(columns=RESULT_COLUMNS).astype("float64")
    results[["n_event", "n_control"]] = results[["n_event", "n_control"]].astype("int64")
    return Experiment(
        name=body["name"],
        rule=Rule.from_dict(body["rule"]),
        horizons=tuple(body["horizons"]),
        start=date.fromisoformat(body["start"]),
        end=date.fromisoformat(body["end"]),
        codes=list(body["codes"]),
        data_fingerprint=body["data_fingerprint"],
        results=results,
        created_at=datetime.fromisoformat(body["created_at"]),
    )


def load_all(directory: Path | None = None) -> list[Experiment]:
    directory = directory or default_dir()
    if not directory.exists():
        return []
    return sorted((_load(p) for p in directory.glob("*.json")), key=lambda e: e.created_at)


def compare(experiments: list[Experiment], horizon: int) -> tuple[pd.DataFrame, list[str]]:
    """Tabel hasil per eksperimen untuk satu horizon + peringatan bila datanya berbeda."""
    rows = {}
    for exp in experiments:
        if horizon in exp.results.index:
            rows[exp.name] = exp.results.loc[horizon, RESULT_COLUMNS]
        else:
            rows[exp.name] = pd.Series(np.nan, index=RESULT_COLUMNS)
    table = pd.DataFrame.from_dict(rows, orient="index", columns=RESULT_COLUMNS)

    warnings = []
    if experiments:
        ref = experiments[0]
        for exp in experiments[1:]:
            same = (exp.data_fingerprint, exp.start, exp.end, sorted(exp.codes)) == (
                ref.data_fingerprint,
                ref.start,
                ref.end,
                sorted(ref.codes),
            )
            if not same:
                warnings.append(
                    f"'{exp.name}' memakai data berbeda dari '{ref.name}';"
                    " perbandingan bisa menyesatkan"
                )
    return table, warnings
