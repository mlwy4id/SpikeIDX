"""Satu-satunya sumber rumus sinyal: dipakai worker, event study, dan TUI.

Konvensi tanpa look-ahead:
- baseline volume (`avg20`, `std20`) = `window` bar SEBELUM bar t (rolling lalu shift 1);
- `cmf20` = `window` bar sampai dan termasuk t;
- semua rolling/shift dikerjakan per `code`.
"""

from dataclasses import asdict, dataclass

import numpy as np
import pandas as pd

RULE_VERSION = "py-v1"
FILTERED_LABEL = "terfilter (noise harga)"


@dataclass(frozen=True)
class Rule:
    multiple_min: float = 2.0
    z_min: float = 2.0
    pct_min: float = 2.0
    cmf_min: float = 0.10
    price_filter: bool = True

    def to_dict(self) -> dict:
        return asdict(self)

    @classmethod
    def from_dict(cls, data: dict) -> "Rule":
        return cls(**data)


def add_features(df: pd.DataFrame, window: int = 20) -> pd.DataFrame:
    """Tambah avg20, std20, multiple, zscore, pct_change, cmf20 ke OHLCV (urut code, date)."""
    out = df.sort_values(["code", "date"]).reset_index(drop=True)
    vol = out["volume"].astype("float64")
    by_code = vol.groupby(out["code"])

    avg = by_code.transform(lambda v: v.rolling(window).mean().shift(1))
    std = by_code.transform(lambda v: v.rolling(window).std(ddof=0).shift(1))
    out["avg20"] = avg
    out["std20"] = std
    out["multiple"] = np.where(avg > 0, vol / avg, np.where(avg.isna(), np.nan, 0.0))
    out["zscore"] = np.where(std > 0, (vol - avg) / std, np.where(std.isna(), np.nan, 0.0))

    prev_close = out["close"].groupby(out["code"]).shift(1)
    out["pct_change"] = np.where(
        prev_close > 0,
        (out["close"] - prev_close) / prev_close * 100,
        np.where(prev_close.isna(), np.nan, 0.0),
    )

    rng = out["high"] - out["low"]
    mfm = np.where(rng > 0, ((out["close"] - out["low"]) - (out["high"] - out["close"])) / rng, 0.0)
    mfv = pd.Series(mfm, index=out.index) * vol
    sum_mfv = mfv.groupby(out["code"]).transform(lambda s: s.rolling(window).sum())
    sum_vol = by_code.transform(lambda v: v.rolling(window).sum())
    out["cmf20"] = np.where(sum_vol > 0, sum_mfv / sum_vol, np.where(sum_vol.isna(), np.nan, 0.0))
    return out


def flow_label(cmf: pd.Series, cmf_min: float) -> pd.Series:
    return pd.Series(
        np.select([cmf > cmf_min, cmf < -cmf_min], ["akumulasi", "distribusi"], "netral"),
        index=cmf.index,
    )


def classify(df: pd.DataFrame, rule: Rule) -> pd.DataFrame:
    """Tambah is_spike, is_filtered, is_actionable, label. Operator strict; NaN → bukan spike."""
    out = df.copy()
    spike = (out["multiple"] > rule.multiple_min) & (out["zscore"] > rule.z_min)
    filtered = spike & rule.price_filter & (out["pct_change"].abs() < rule.pct_min)
    out["is_spike"] = spike.astype(bool)
    out["is_filtered"] = filtered.astype(bool)
    out["is_actionable"] = (spike & ~filtered).astype(bool)
    label = flow_label(out["cmf20"], rule.cmf_min)
    out["label"] = label.where(~out["is_filtered"], FILTERED_LABEL)
    return out
