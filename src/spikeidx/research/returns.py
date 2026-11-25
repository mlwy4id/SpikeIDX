import numpy as np
import pandas as pd


def forward_returns(df: pd.DataFrame, horizons=(1, 5, 20)) -> pd.DataFrame:
    """Tambah `fwd_{h}` = close[t+h] / open[t+1] - 1 per kode.

    Entry di open t+1 karena sinyal baru diketahui setelah close t. NaN bila bar
    masa depan belum ada atau open entry tidak positif.
    """
    out = df.sort_values(["code", "date"]).reset_index(drop=True)
    by = out.groupby("code")
    entry = by["open"].shift(-1)
    for h in horizons:
        exit_ = by["close"].shift(-h)
        out[f"fwd_{h}"] = np.where(entry > 0, exit_ / entry - 1, np.nan)
    return out


def flag_corporate_actions(df: pd.DataFrame, threshold: float = 0.35) -> pd.Series:
    """True bila close atau open melompat lebih dari `threshold` dari close sebelumnya.

    Di atas batas auto-reject BEI, jadi kemungkinan split/right issue/data rusak.
    Mengikuti urutan baris `df` (harus urut code, date).
    """
    prev = df.groupby("code")["close"].shift(1)
    jump_close = (df["close"] / prev - 1).abs() > threshold
    jump_open = (df["open"] / prev - 1).abs() > threshold
    return (jump_close | jump_open).fillna(False).astype(bool)
