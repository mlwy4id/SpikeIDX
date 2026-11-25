"""Event study: apakah forward return setelah event berbeda dari hari non-spike?

Batasan yang harus selalu dilaporkan bersama hasil: return antar event bisa
overlap (t-test cenderung terlalu optimistis), universe hanya saham yang masih
listing (survivorship), dan ukuran sampel kecil untuk rule yang ketat.
"""

import numpy as np
import pandas as pd
from scipy import stats

from spikeidx.research.returns import flag_corporate_actions, forward_returns
from spikeidx.signals import Rule, add_features, classify

RESULT_COLUMNS = [
    "n_event",
    "n_control",
    "mean_event",
    "mean_control",
    "median_event",
    "hit_rate_event",
    "diff",
    "t_stat",
    "p_value",
]


def _any_within(flag: pd.Series, codes: pd.Series, before: int, after: int) -> pd.Series:
    """True bila ada flag di [t-before, t+after] pada kode yang sama."""
    f = flag.astype(float)
    g = f.groupby(codes)
    past = g.transform(lambda s: s.rolling(before + 1, min_periods=1).max())
    future = g.transform(lambda s: s[::-1].rolling(after + 1, min_periods=1).max()[::-1])
    return (past > 0) | (future > 0)


def event_study(
    df: pd.DataFrame,
    event_col: str = "is_actionable",
    horizons=(1, 5, 20),
    exclusion: int = 20,
) -> pd.DataFrame:
    """Butuh kolom code, date, open, close, avg20, is_spike, `event_col`, dan fwd_{h}."""
    data = df.sort_values(["code", "date"]).reset_index(drop=True)
    flags = flag_corporate_actions(data)
    base = data["avg20"].notna()
    events = base & data[event_col].astype(bool)
    controls = base & ~data["is_spike"].astype(bool)

    rows = {}
    for h in horizons:
        clean = ~_any_within(flags, data["code"], exclusion, h)
        fwd = data[f"fwd_{h}"]
        ev = fwd[events & clean].dropna()
        ct = fwd[controls & clean].dropna()
        enough = len(ev) >= 2 and len(ct) >= 2
        t_stat, p_value = stats.ttest_ind(ev, ct, equal_var=False) if enough else (np.nan, np.nan)
        mean_ev = ev.mean() if len(ev) else np.nan
        mean_ct = ct.mean() if len(ct) else np.nan
        rows[h] = {
            "n_event": len(ev),
            "n_control": len(ct),
            "mean_event": mean_ev,
            "mean_control": mean_ct,
            "median_event": ev.median() if len(ev) else np.nan,
            "hit_rate_event": (ev > 0).mean() if len(ev) else np.nan,
            "diff": mean_ev - mean_ct,
            "t_stat": float(t_stat),
            "p_value": float(p_value),
        }
    res = pd.DataFrame.from_dict(rows, orient="index", columns=RESULT_COLUMNS)
    res.index.name = "horizon"
    return res


def run_study(
    ohlcv: pd.DataFrame,
    rule: Rule,
    horizons=(1, 5, 20),
    event_col: str = "is_actionable",
) -> tuple[pd.DataFrame, pd.DataFrame]:
    """OHLCV mentah → (tabel hasil per horizon, baris event lengkap dengan fitur + fwd)."""
    data = forward_returns(classify(add_features(ohlcv), rule), horizons)
    results = event_study(data, event_col, horizons)
    events = data[data[event_col] & data["avg20"].notna()].reset_index(drop=True)
    return results, events
