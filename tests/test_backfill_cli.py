from datetime import date

import pandas as pd

from spikeidx import store
from spikeidx.backfill import backfill
from spikeidx.cli import build_parser
from spikeidx.stocks import Stock


class FakeYahoo:
    def __init__(self):
        self.calls = []

    def daily_ohlcv(self, code, start, end):
        self.calls.append((code, start, end))
        return pd.DataFrame(
            {
                "code": [code],
                "date": [end],
                "open": [1.0],
                "high": [1.0],
                "low": [1.0],
                "close": [1.0],
                "volume": [10],
            }
        )

    def search(self, query):
        return [Stock("BBCA", "BBCA.JK", "Bank Central Asia")] if query == "BBCA" else []


def test_backfill_ensures_unknown_stocks_and_spans_years(engine):
    yahoo = FakeYahoo()
    sleeps = []
    result = backfill(
        engine, yahoo, ["BBCA", "ZZZZ"], years=3, today=date(2026, 11, 25), sleep=sleeps.append
    )
    assert result == {"BBCA": 1, "ZZZZ": 1}
    assert yahoo.calls[0][1] == date(2023, 11, 25)
    assert sleeps == [1.0]
    assert store.get_stock(engine, "BBCA").name == "Bank Central Asia"
    assert store.get_stock(engine, "ZZZZ").name == "ZZZZ"  # fallback: kode sebagai nama
    assert len(store.load_ohlcv(engine)) == 2


def test_backfill_reports_failures_per_code(engine):
    class Failing(FakeYahoo):
        def daily_ohlcv(self, code, start, end):
            if code == "BAD":
                raise RuntimeError("404")
            return super().daily_ohlcv(code, start, end)

    result = backfill(engine, Failing(), ["BAD", "BBCA"], 1, date(2026, 11, 25), lambda s: None)
    assert result["BBCA"] == 1 and isinstance(result["BAD"], str)


def test_cli_parser_commands():
    p = build_parser()
    assert p.parse_args(["worker", "--dry-run"]).dry_run is True
    args = p.parse_args(["backfill", "--years", "5", "bbca", "TLKM"])
    assert args.years == 5 and args.codes == ["bbca", "TLKM"]
    assert p.parse_args(["migrate"]).command == "migrate"
    assert p.parse_args(["tui"]).command == "tui"


def test_configure_logging_silences_http_request_urls():
    # URL Telegram berisi token bot; httpx mencatat URL request di level INFO.
    import logging

    from spikeidx.cli import configure_logging

    configure_logging()
    for name in ("httpx", "httpcore"):
        assert not logging.getLogger(name).isEnabledFor(logging.INFO)
    assert logging.getLogger("spikeidx").isEnabledFor(logging.INFO)
