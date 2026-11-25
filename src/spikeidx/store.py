"""Akses Postgres (SQL portabel; test memakai SQLite).

Konvensi DataFrame OHLCV: kolom `OHLCV_COLUMNS`, `date` bertipe datetime64,
urut `[code, date]`. Tabel `signals` masih memiliki kolom legacy Go (`adl`,
`adl_slope5`) yang diisi 0 sampai kolom itu dihapus.
"""

from collections.abc import Iterable
from datetime import date

import pandas as pd
import sqlalchemy as sa

from spikeidx.stocks import Stock

OHLCV_COLUMNS = ["code", "date", "open", "high", "low", "close", "volume"]
DEFAULT_USER = "default"
WATCHLIST_CAP = 100


class UnknownStock(LookupError):
    pass


class WatchlistFull(RuntimeError):
    pass


def _as_date(value) -> date:
    return pd.Timestamp(value).date()


def normalize_ohlcv(df: pd.DataFrame) -> pd.DataFrame:
    out = df[OHLCV_COLUMNS].copy()
    out["code"] = out["code"].astype(str)
    out["date"] = pd.to_datetime(out["date"])
    for col in ("open", "high", "low", "close"):
        out[col] = out[col].astype("float64")
    out["volume"] = out["volume"].astype("int64")
    out = out.drop_duplicates(["code", "date"], keep="last")
    return out.sort_values(["code", "date"]).reset_index(drop=True)


def load_ohlcv(
    engine: sa.Engine,
    codes: Iterable[str] | None = None,
    start: date | None = None,
    end: date | None = None,
) -> pd.DataFrame:
    where, params = [], {}
    if codes is not None:
        codes = list(codes)
        if not codes:
            return normalize_ohlcv(pd.DataFrame(columns=OHLCV_COLUMNS))
        names = [f"c{i}" for i in range(len(codes))]
        where.append(f"code IN ({', '.join(':' + n for n in names)})")
        params.update(zip(names, codes, strict=True))
    if start is not None:
        where.append("date >= :start")
        params["start"] = start
    if end is not None:
        where.append("date <= :end")
        params["end"] = end
    sql = "SELECT code, date, open, high, low, close, volume FROM ohlcv"
    if where:
        sql += " WHERE " + " AND ".join(where)
    with engine.connect() as conn:
        rows = conn.execute(sa.text(sql), params).all()
    return normalize_ohlcv(pd.DataFrame(rows, columns=OHLCV_COLUMNS))


_UPSERT_OHLCV = sa.text(
    "INSERT INTO ohlcv (code, date, open, high, low, close, volume)"
    " VALUES (:code, :date, :open, :high, :low, :close, :volume)"
    " ON CONFLICT (code, date) DO UPDATE SET open = excluded.open, high = excluded.high,"
    " low = excluded.low, close = excluded.close, volume = excluded.volume"
)


def upsert_ohlcv(engine: sa.Engine, df: pd.DataFrame) -> int:
    if df.empty:
        return 0
    rows = [
        {
            "code": str(r.code),
            "date": _as_date(r.date),
            "open": float(r.open),
            "high": float(r.high),
            "low": float(r.low),
            "close": float(r.close),
            "volume": int(r.volume),
        }
        for r in normalize_ohlcv(df).itertuples(index=False)
    ]
    with engine.begin() as conn:
        conn.execute(_UPSERT_OHLCV, rows)
    return len(rows)


def ensure_stock(engine: sa.Engine, stock: Stock) -> None:
    with engine.begin() as conn:
        conn.execute(
            sa.text(
                "INSERT INTO stocks_master (code, yahoo_symbol, name, sector)"
                " VALUES (:code, :yahoo_symbol, :name, :sector)"
                " ON CONFLICT (code) DO UPDATE SET yahoo_symbol = excluded.yahoo_symbol,"
                " name = excluded.name, sector = excluded.sector, updated_at = CURRENT_TIMESTAMP"
            ),
            {
                "code": stock.code,
                "yahoo_symbol": stock.yahoo_symbol,
                "name": stock.name,
                "sector": stock.sector or None,
            },
        )


def get_stock(engine: sa.Engine, code: str) -> Stock | None:
    with engine.connect() as conn:
        row = conn.execute(
            sa.text("SELECT code, yahoo_symbol, name, sector FROM stocks_master WHERE code = :c"),
            {"c": code},
        ).first()
    return None if row is None else Stock(row[0], row[1], row[2], row[3] or "")


def list_watchlist(engine: sa.Engine) -> list[str]:
    with engine.connect() as conn:
        return list(
            conn.execute(
                sa.text("SELECT code FROM watchlist WHERE user_id = :u ORDER BY code"),
                {"u": DEFAULT_USER},
            ).scalars()
        )


def add_watchlist(engine: sa.Engine, code: str) -> None:
    if get_stock(engine, code) is None:
        raise UnknownStock(f"{code} belum ada di stocks_master; cari dulu lewat search")
    with engine.begin() as conn:
        exists = conn.execute(
            sa.text("SELECT 1 FROM watchlist WHERE user_id = :u AND code = :c"),
            {"u": DEFAULT_USER, "c": code},
        ).first()
        if exists:
            return
        count = conn.execute(
            sa.text("SELECT count(*) FROM watchlist WHERE user_id = :u"), {"u": DEFAULT_USER}
        ).scalar_one()
        if count >= WATCHLIST_CAP:
            raise WatchlistFull(f"watchlist penuh ({WATCHLIST_CAP})")
        conn.execute(
            sa.text(
                "INSERT INTO watchlist (user_id, code, created_at)"
                " VALUES (:u, :c, CURRENT_TIMESTAMP)"
            ),
            {"u": DEFAULT_USER, "c": code},
        )


def remove_watchlist(engine: sa.Engine, code: str) -> None:
    with engine.begin() as conn:
        conn.execute(
            sa.text("DELETE FROM watchlist WHERE user_id = :u AND code = :c"),
            {"u": DEFAULT_USER, "c": code},
        )


_UPSERT_SIGNAL = sa.text(
    "INSERT INTO signals (code, date, volume, avg20, multiple, zscore, close, pct_change,"
    " adl, adl_slope5, cmf, is_filtered, rule_version)"
    " VALUES (:code, :date, :volume, :avg20, :multiple, :zscore, :close, :pct_change,"
    " 0, 0, :cmf, :is_filtered, :rule_version)"
    " ON CONFLICT (code, date) DO UPDATE SET volume = excluded.volume, avg20 = excluded.avg20,"
    " multiple = excluded.multiple, zscore = excluded.zscore, close = excluded.close,"
    " pct_change = excluded.pct_change, adl = 0, adl_slope5 = 0, cmf = excluded.cmf,"
    " is_filtered = excluded.is_filtered, rule_version = excluded.rule_version"
)


def upsert_signals(engine: sa.Engine, df: pd.DataFrame, rule_version: str) -> int:
    """Simpan baris spike. Kolom yang dibaca: code, date, volume, avg20, multiple,
    zscore, close, pct_change, cmf20, is_filtered."""
    if df.empty:
        return 0
    rows = [
        {
            "code": str(r.code),
            "date": _as_date(r.date),
            "volume": int(r.volume),
            "avg20": float(r.avg20),
            "multiple": float(r.multiple),
            "zscore": float(r.zscore),
            "close": float(r.close),
            "pct_change": float(r.pct_change),
            "cmf": float(r.cmf20),
            "is_filtered": bool(r.is_filtered),
            "rule_version": rule_version,
        }
        for r in df.itertuples(index=False)
    ]
    with engine.begin() as conn:
        conn.execute(_UPSERT_SIGNAL, rows)
    return len(rows)


SIGNAL_COLUMNS = [
    "code",
    "date",
    "volume",
    "avg20",
    "multiple",
    "zscore",
    "close",
    "pct_change",
    "cmf20",
    "is_filtered",
    "rule_version",
]


def signals_on(engine: sa.Engine, day: date, include_filtered: bool = False) -> pd.DataFrame:
    sql = (
        "SELECT code, date, volume, avg20, multiple, zscore, close, pct_change, cmf,"
        " is_filtered, rule_version FROM signals WHERE date = :d"
    )
    params = {"d": day}
    if not include_filtered:
        sql += " AND is_filtered = :f"
        params["f"] = False
    sql += " ORDER BY multiple DESC"
    with engine.connect() as conn:
        rows = conn.execute(sa.text(sql), params).all()
    df = pd.DataFrame(rows, columns=SIGNAL_COLUMNS)
    df["date"] = pd.to_datetime(df["date"])
    for col in ("avg20", "multiple", "zscore", "close", "pct_change", "cmf20"):
        df[col] = df[col].astype("float64")
    df["is_filtered"] = df["is_filtered"].astype(bool)
    return df


def digested_dates(engine: sa.Engine, since: date) -> set[date]:
    with engine.connect() as conn:
        rows = conn.execute(
            sa.text("SELECT date FROM digest_log WHERE date >= :s"), {"s": since}
        ).scalars()
        return {_as_date(d) for d in rows}


def record_digest(engine: sa.Engine, day: date, n_signals: int) -> None:
    with engine.begin() as conn:
        conn.execute(
            sa.text(
                "INSERT INTO digest_log (date, sent_at, n_signals)"
                " VALUES (:d, CURRENT_TIMESTAMP, :n)"
                " ON CONFLICT (date) DO UPDATE SET sent_at = excluded.sent_at,"
                " n_signals = excluded.n_signals"
            ),
            {"d": day, "n": n_signals},
        )


def data_status(engine: sa.Engine) -> pd.DataFrame:
    """Per kode watchlist: bar pertama/terakhir dan jumlah bar di `ohlcv`."""
    with engine.connect() as conn:
        rows = conn.execute(
            sa.text(
                "SELECT w.code, min(o.date), max(o.date), count(o.date) FROM watchlist w"
                " LEFT JOIN ohlcv o ON o.code = w.code WHERE w.user_id = :u"
                " GROUP BY w.code ORDER BY w.code"
            ),
            {"u": DEFAULT_USER},
        ).all()
    df = pd.DataFrame(rows, columns=["code", "first", "last", "bars"])
    for col in ("first", "last"):
        df[col] = [None if v is None else _as_date(v) for v in df[col]]
    df["bars"] = df["bars"].astype(int)
    return df
