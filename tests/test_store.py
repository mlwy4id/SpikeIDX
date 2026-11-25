from datetime import date
from decimal import Decimal

import pandas as pd
import pytest

from spikeidx import store
from spikeidx.stocks import Stock


def _bars(code, rows):
    return pd.DataFrame(
        [(code, d, o, h, lo, c, v) for d, o, h, lo, c, v in rows],
        columns=["code", "date", "open", "high", "low", "close", "volume"],
    )


def test_normalize_ohlcv_sorts_casts_and_dedups():
    raw = pd.DataFrame(
        {
            "code": ["TLKM", "BBCA", "BBCA", "BBCA"],
            "date": ["2026-11-24", "2026-11-25", "2026-11-24", "2026-11-24"],
            "open": [Decimal("1"), Decimal("2"), Decimal("3"), Decimal("4")],
            "high": [1, 2, 3, 4],
            "low": [1, 2, 3, 4],
            "close": [1, 2, 3, 4],
            "volume": [10.0, 20.0, 30.0, 40.0],
        }
    )
    df = store.normalize_ohlcv(raw)
    assert list(zip(df["code"], df["date"].dt.date, strict=True)) == [
        ("BBCA", date(2026, 11, 24)),
        ("BBCA", date(2026, 11, 25)),
        ("TLKM", date(2026, 11, 24)),
    ]
    assert df["open"].dtype == "float64"
    assert df["volume"].dtype == "int64"
    assert df.loc[0, "open"] == 4.0  # duplikat: baris terakhir menang


def test_upsert_then_load_ohlcv_filters_and_overwrites(engine):
    store.upsert_ohlcv(engine, _bars("BBCA", [(date(2026, 11, 24), 1, 2, 1, 2, 100)]))
    store.upsert_ohlcv(engine, _bars("BBCA", [(date(2026, 11, 24), 1, 2, 1, 3, 999)]))
    store.upsert_ohlcv(engine, _bars("TLKM", [(date(2026, 11, 25), 1, 2, 1, 2, 5)]))

    df = store.load_ohlcv(engine, codes=["BBCA"])
    assert len(df) == 1 and df.loc[0, "volume"] == 999 and df.loc[0, "close"] == 3.0

    assert len(store.load_ohlcv(engine, start=date(2026, 11, 25))) == 1
    assert len(store.load_ohlcv(engine, end=date(2026, 11, 24))) == 1
    assert len(store.load_ohlcv(engine)) == 2


def test_load_ohlcv_empty_has_columns(engine):
    df = store.load_ohlcv(engine, codes=["NONE"])
    assert df.empty and list(df.columns) == store.OHLCV_COLUMNS


def test_watchlist_requires_known_stock_and_caps(engine, monkeypatch):
    with pytest.raises(store.UnknownStock):
        store.add_watchlist(engine, "BBCA")

    store.ensure_stock(engine, Stock("BBCA", "BBCA.JK", "Bank Central Asia"))
    store.ensure_stock(engine, Stock("TLKM", "TLKM.JK", "Telkom"))
    store.add_watchlist(engine, "TLKM")
    store.add_watchlist(engine, "BBCA")
    store.add_watchlist(engine, "BBCA")  # idempoten
    assert store.list_watchlist(engine) == ["BBCA", "TLKM"]

    monkeypatch.setattr(store, "WATCHLIST_CAP", 2)
    store.ensure_stock(engine, Stock("ASII", "ASII.JK", "Astra"))
    with pytest.raises(store.WatchlistFull):
        store.add_watchlist(engine, "ASII")

    store.remove_watchlist(engine, "BBCA")
    assert store.list_watchlist(engine) == ["TLKM"]


def test_ensure_stock_updates_name(engine):
    store.ensure_stock(engine, Stock("BBCA", "BBCA.JK", "BBCA"))
    store.ensure_stock(engine, Stock("BBCA", "BBCA.JK", "Bank Central Asia", "Financial"))
    assert store.get_stock(engine, "BBCA") == Stock(
        "BBCA", "BBCA.JK", "Bank Central Asia", "Financial"
    )
    assert store.get_stock(engine, "XXXX") is None


def test_upsert_signals_and_read_by_date(engine):
    sig = pd.DataFrame(
        {
            "code": ["BBCA", "TLKM"],
            "date": pd.to_datetime(["2026-11-24", "2026-11-24"]),
            "volume": [300, 200],
            "avg20": [100.0, 100.0],
            "multiple": [3.0, 2.5],
            "zscore": [4.0, 3.0],
            "close": [10.0, 20.0],
            "pct_change": [3.0, 1.0],
            "cmf20": [0.2, -0.3],
            "is_filtered": [False, True],
        }
    )
    store.upsert_signals(engine, sig, rule_version="py-v1")
    store.upsert_signals(engine, sig, rule_version="py-v1")  # idempoten

    actionable = store.signals_on(engine, date(2026, 11, 24))
    assert list(actionable["code"]) == ["BBCA"]
    both = store.signals_on(engine, date(2026, 11, 24), include_filtered=True)
    assert list(both["code"]) == ["BBCA", "TLKM"]  # urut multiple desc
    assert both.loc[0, "cmf20"] == pytest.approx(0.2)

    with engine.connect() as conn:
        row = conn.exec_driver_sql("SELECT adl, adl_slope5, rule_version FROM signals").first()
    assert tuple(row) == (0, 0, "py-v1")


def test_digest_log_roundtrip(engine):
    assert store.digested_dates(engine, since=date(2026, 11, 1)) == set()
    store.record_digest(engine, date(2026, 11, 24), 3)
    store.record_digest(engine, date(2026, 10, 1), 0)
    assert store.digested_dates(engine, since=date(2026, 11, 1)) == {date(2026, 11, 24)}


def test_data_status_per_watchlist_code(engine):
    store.ensure_stock(engine, Stock("BBCA", "BBCA.JK", "BCA"))
    store.ensure_stock(engine, Stock("TLKM", "TLKM.JK", "Telkom"))
    store.add_watchlist(engine, "BBCA")
    store.add_watchlist(engine, "TLKM")
    store.upsert_ohlcv(
        engine,
        _bars("BBCA", [(date(2026, 11, 20), 1, 1, 1, 1, 1), (date(2026, 11, 24), 1, 1, 1, 1, 1)]),
    )
    st = store.data_status(engine).set_index("code")
    assert st.loc["BBCA", "bars"] == 2
    assert st.loc["BBCA", "last"] == date(2026, 11, 24)
    assert st.loc["TLKM", "bars"] == 0
