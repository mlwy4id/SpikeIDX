"""Yahoo Finance `.JK`: chart harian + search.

Retry 429/5xx/timeout maksimal 3 kali (backoff base*2^n + jitter); status lain gagal
langsung. Pemanggil batch tetap wajib memberi jeda 1 detik antar kode.
"""

import random
import time
from collections.abc import Callable
from datetime import date, datetime, timedelta
from datetime import time as dtime

import httpx
import pandas as pd

from spikeidx.calendar import WIB
from spikeidx.stocks import Stock, parse_code, yahoo_symbol

CHART_URL = "https://query1.finance.yahoo.com/v8/finance/chart/"
SEARCH_URL = "https://query1.finance.yahoo.com/v1/finance/search"
USER_AGENT = "Mozilla/5.0 (X11; Linux x86_64) SpikeIDX/0.2"
OHLCV_COLUMNS = ["code", "date", "open", "high", "low", "close", "volume"]


class YahooError(RuntimeError):
    pass


def _retryable(status: int) -> bool:
    return status == 429 or 500 <= status <= 599


class YahooClient:
    def __init__(
        self,
        http: httpx.Client | None = None,
        sleep: Callable[[float], None] = time.sleep,
        max_retries: int = 3,
        base_backoff: float = 1.0,
    ):
        self._http = http or httpx.Client(timeout=20)
        self._sleep = sleep
        self._max_retries = max_retries
        self._base = base_backoff

    def _get(self, url: str, params: dict) -> dict:
        attempts = self._max_retries + 1
        for attempt in range(attempts):
            last = attempt + 1 == attempts
            try:
                resp = self._http.get(url, params=params, headers={"User-Agent": USER_AGENT})
            except httpx.TimeoutException:
                if last:
                    raise
                self._backoff(attempt)
                continue
            if resp.status_code == 200:
                return resp.json()
            if not _retryable(resp.status_code) or last:
                raise YahooError(f"yahoo: http {resp.status_code} for {url}")
            self._backoff(attempt)
        raise AssertionError("unreachable")

    def _backoff(self, attempt: int) -> None:
        self._sleep(self._base * 2**attempt + random.uniform(0, self._base))

    def daily_ohlcv(self, code: str, start: date, end: date) -> pd.DataFrame:
        """Bar harian `start..end` (inklusif, tanggal WIB). Bar dengan nilai null dibuang."""
        period1 = datetime.combine(start, dtime(), WIB)
        period2 = datetime.combine(end + timedelta(days=1), dtime(), WIB)
        body = self._get(
            CHART_URL + yahoo_symbol(code),
            {
                "period1": int(period1.timestamp()),
                "period2": int(period2.timestamp()),
                "interval": "1d",
                "events": "div|split",
            },
        )
        chart = body.get("chart") or {}
        if chart.get("error"):
            err = chart["error"]
            raise YahooError(f"yahoo: {err.get('code')} {err.get('description')}")
        results = chart.get("result") or []
        if not results:
            raise YahooError(f"yahoo: empty result for {code}")
        res = results[0]
        quote = (res.get("indicators", {}).get("quote") or [{}])[0]
        df = pd.DataFrame(
            {
                "date": [datetime.fromtimestamp(ts, WIB).date() for ts in res.get("timestamp", [])],
                **{k: quote.get(k, []) for k in ("open", "high", "low", "close", "volume")},
            }
        )
        df = df.dropna()
        df.insert(0, "code", code)
        df["volume"] = df["volume"].astype("int64")
        for col in ("open", "high", "low", "close"):
            df[col] = df[col].astype("float64")
        return df[OHLCV_COLUMNS].reset_index(drop=True)

    def search(self, query: str) -> list[Stock]:
        body = self._get(SEARCH_URL, {"q": query})
        out = []
        for q in body.get("quotes", []):
            symbol = q.get("symbol", "")
            if not symbol.endswith(".JK"):
                continue
            try:
                code = parse_code(symbol)
            except ValueError:
                continue
            name = q.get("longname") or q.get("shortname") or code
            out.append(Stock(code, yahoo_symbol(code), name, q.get("sector") or ""))
        return out
