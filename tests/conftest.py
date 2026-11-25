import os
import sqlite3
from datetime import date

import pytest
import sqlalchemy as sa


@pytest.fixture(autouse=True)
def _isolated_environ(monkeypatch):
    # load_dotenv menulis ke os.environ; salinan per test mencegah bocor antar test.
    monkeypatch.setattr(os, "environ", os.environ.copy())


sqlite3.register_adapter(date, date.isoformat)

SCHEMA = [
    "CREATE TABLE stocks_master (code TEXT PRIMARY KEY, yahoo_symbol TEXT NOT NULL,"
    " name TEXT NOT NULL, sector TEXT, updated_at TIMESTAMP)",
    "CREATE TABLE watchlist (user_id TEXT NOT NULL DEFAULT 'default', code TEXT NOT NULL,"
    " created_at TIMESTAMP, PRIMARY KEY (user_id, code))",
    "CREATE TABLE ohlcv (code TEXT, date DATE, open NUMERIC, high NUMERIC, low NUMERIC,"
    " close NUMERIC, volume BIGINT, raw TEXT, PRIMARY KEY (code, date))",
    "CREATE TABLE signals (code TEXT, date DATE, volume BIGINT, avg20 NUMERIC, multiple NUMERIC,"
    " zscore NUMERIC, close NUMERIC, pct_change NUMERIC, adl NUMERIC, adl_slope5 NUMERIC,"
    " cmf NUMERIC, is_filtered BOOLEAN, rule_version TEXT, created_at TIMESTAMP,"
    " PRIMARY KEY (code, date))",
    "CREATE TABLE digest_log (date DATE PRIMARY KEY, sent_at TIMESTAMP, n_signals INT)",
]


@pytest.fixture
def engine():
    eng = sa.create_engine("sqlite://")
    with eng.begin() as conn:
        for ddl in SCHEMA:
            conn.exec_driver_sql(ddl)
    return eng
