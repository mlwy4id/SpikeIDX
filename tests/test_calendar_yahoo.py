import json
from datetime import date, datetime

import httpx
import pytest

from spikeidx.calendar import WIB, fetch_holidays, is_trading_day, trading_days
from spikeidx.sources.yahoo import YahooClient, YahooError
from spikeidx.stocks import parse_code


def _client(handler) -> httpx.Client:
    return httpx.Client(transport=httpx.MockTransport(handler))


# --- kode saham ---


@pytest.mark.parametrize("raw,code", [("bbca", "BBCA"), (" BBCA.JK ", "BBCA"), ("TLK", "TLK")])
def test_parse_code_normalizes(raw, code):
    assert parse_code(raw) == code


@pytest.mark.parametrize("raw", ["", "BB", "BBCAX", "BB1A", "^JKSE"])
def test_parse_code_rejects_invalid(raw):
    with pytest.raises(ValueError):
        parse_code(raw)


# --- kalender ---


def test_fetch_holidays_parses_dates_and_skips_garbage():
    def handler(request):
        assert request.url.path == "/api/v3/PublicHolidays/2026/ID"
        return httpx.Response(200, json=[{"date": "2026-01-01"}, {"date": "rusak"}])

    assert fetch_holidays(2026, _client(handler)) == {date(2026, 1, 1)}


def test_fetch_holidays_raises_on_http_error():
    with pytest.raises(httpx.HTTPStatusError):
        fetch_holidays(2026, _client(lambda r: httpx.Response(500)))


def test_is_trading_day_excludes_weekend_and_holiday():
    holidays = {date(2026, 11, 24)}
    assert is_trading_day(date(2026, 11, 25), holidays)
    assert not is_trading_day(date(2026, 11, 24), holidays)
    assert not is_trading_day(date(2026, 11, 28), holidays)  # Sabtu


def test_trading_days_inclusive_range():
    days = trading_days(date(2026, 11, 20), date(2026, 11, 24), {date(2026, 11, 23)})
    assert days == [date(2026, 11, 20), date(2026, 11, 24)]


# --- Yahoo ---


def _ts(d: date) -> int:
    return int(datetime(d.year, d.month, d.day, 9, 0, tzinfo=WIB).timestamp())


def _chart(rows):
    return {
        "chart": {
            "result": [
                {
                    "timestamp": [_ts(r[0]) for r in rows],
                    "indicators": {
                        "quote": [
                            {
                                "open": [r[1] for r in rows],
                                "high": [r[2] for r in rows],
                                "low": [r[3] for r in rows],
                                "close": [r[4] for r in rows],
                                "volume": [r[5] for r in rows],
                            }
                        ]
                    },
                }
            ],
            "error": None,
        }
    }


def test_daily_ohlcv_sends_period_and_symbol():
    seen = {}

    def handler(request):
        seen["path"] = request.url.path
        seen["params"] = dict(request.url.params)
        seen["ua"] = request.headers["user-agent"]
        return httpx.Response(200, json=_chart([]))

    YahooClient(_client(handler)).daily_ohlcv("BBCA", date(2026, 11, 1), date(2026, 11, 24))
    assert seen["path"].endswith("/BBCA.JK")
    start = datetime(2026, 11, 1, tzinfo=WIB).timestamp()
    end = datetime(2026, 11, 25, tzinfo=WIB).timestamp()
    assert seen["params"]["period1"] == str(int(start))
    assert seen["params"]["period2"] == str(int(end))
    assert seen["params"]["interval"] == "1d"
    assert "Mozilla" in seen["ua"]


def test_daily_ohlcv_drops_null_bars_and_uses_wib_dates():
    rows = [
        (date(2026, 11, 20), 100, 102, 99, 101, 1000),
        (date(2026, 11, 23), None, None, None, None, None),
        (date(2026, 11, 24), 101, 103, 100, 102, 2000),
    ]
    df = YahooClient(_client(lambda r: httpx.Response(200, json=_chart(rows)))).daily_ohlcv(
        "BBCA", date(2026, 11, 1), date(2026, 11, 24)
    )
    assert list(df["date"]) == [date(2026, 11, 20), date(2026, 11, 24)]
    assert list(df["volume"]) == [1000, 2000]
    assert set(df["code"]) == {"BBCA"}
    assert list(df.columns) == ["code", "date", "open", "high", "low", "close", "volume"]


def test_daily_ohlcv_retries_429_then_succeeds():
    calls = []
    sleeps = []

    def handler(request):
        calls.append(1)
        return httpx.Response(429) if len(calls) < 3 else httpx.Response(200, json=_chart([]))

    YahooClient(_client(handler), sleep=sleeps.append).daily_ohlcv(
        "BBCA", date(2026, 11, 1), date(2026, 11, 2)
    )
    assert len(calls) == 3
    assert len(sleeps) == 2
    assert 1.0 <= sleeps[0] < 2.0 and 2.0 <= sleeps[1] < 3.0


def test_daily_ohlcv_does_not_retry_404():
    calls = []

    def handler(request):
        calls.append(1)
        return httpx.Response(404)

    with pytest.raises(YahooError, match="404"):
        YahooClient(_client(handler), sleep=lambda s: None).daily_ohlcv(
            "XXXX", date(2026, 11, 1), date(2026, 11, 2)
        )
    assert len(calls) == 1


def test_daily_ohlcv_gives_up_after_three_retries():
    calls = []

    def handler(request):
        calls.append(1)
        return httpx.Response(503)

    with pytest.raises(YahooError, match="503"):
        YahooClient(_client(handler), sleep=lambda s: None).daily_ohlcv(
            "BBCA", date(2026, 11, 1), date(2026, 11, 2)
        )
    assert len(calls) == 4


def test_daily_ohlcv_surfaces_chart_error():
    body = {"chart": {"result": None, "error": {"code": "Not Found", "description": "x"}}}
    with pytest.raises(YahooError, match="Not Found"):
        YahooClient(_client(lambda r: httpx.Response(200, json=body))).daily_ohlcv(
            "BBCA", date(2026, 11, 1), date(2026, 11, 2)
        )


def test_search_keeps_only_valid_jk_symbols():
    body = {
        "quotes": [
            {
                "symbol": "BBCA.JK",
                "shortname": "BCA",
                "longname": "Bank Central Asia",
                "sector": "F",
            },
            {"symbol": "AAPL", "shortname": "Apple"},
            {"symbol": "BB1.JK", "shortname": "rusak"},
            {"symbol": "TLKM.JK", "shortname": "Telkom"},
        ]
    }
    stocks = YahooClient(_client(lambda r: httpx.Response(200, content=json.dumps(body)))).search(
        "bank"
    )
    assert [(s.code, s.name, s.yahoo_symbol) for s in stocks] == [
        ("BBCA", "Bank Central Asia", "BBCA.JK"),
        ("TLKM", "Telkom", "TLKM.JK"),
    ]
