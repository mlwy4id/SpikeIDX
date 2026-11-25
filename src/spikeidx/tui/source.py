"""Sumber data TUI di atas Postgres + Yahoo search."""

import pandas as pd
import sqlalchemy as sa

from spikeidx import store
from spikeidx.sources.yahoo import YahooClient
from spikeidx.stocks import Stock


class DbSource:
    def __init__(self, engine: sa.Engine, yahoo: YahooClient | None = None):
        self._engine = engine
        self._yahoo = yahoo or YahooClient()

    def load_ohlcv(self) -> pd.DataFrame:
        """Seluruh histori kode di watchlist (riset memakai data yang sama dengan worker)."""
        return store.load_ohlcv(self._engine, store.list_watchlist(self._engine))

    def data_status(self) -> pd.DataFrame:
        return store.data_status(self._engine)

    def search(self, query: str) -> list[Stock]:
        stocks = self._yahoo.search(query)
        for stock in stocks:
            store.ensure_stock(self._engine, stock)  # aturan "wajib search dulu"
        return stocks

    def add(self, code: str) -> None:
        store.add_watchlist(self._engine, code)

    def remove(self, code: str) -> None:
        store.remove_watchlist(self._engine, code)
