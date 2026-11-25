from datetime import date, datetime, timedelta

import pandas as pd
import pytest

from spikeidx import store
from spikeidx.calendar import WIB, trading_days
from spikeidx.signals import Rule
from spikeidx.stocks import Stock
from spikeidx.worker import WorkerDeps, WorkerError, pending_dates, run_once

TODAY = date(2026, 11, 25)  # Rabu
EVENING = datetime(2026, 11, 25, 19, 0, tzinfo=WIB)
AFTERNOON = datetime(2026, 11, 25, 16, 30, tzinfo=WIB)


# --- pending_dates ---


def test_pending_excludes_today_before_final_hour():
    assert pending_dates(AFTERNOON, set(), set(), lookback=2) == [date(2026, 11, 24)]


def test_pending_includes_today_after_final_hour():
    assert pending_dates(EVENING, set(), set(), lookback=2) == [
        date(2026, 11, 24),
        date(2026, 11, 25),
    ]


def test_pending_skips_digested_weekends_and_holidays():
    # jendela 4 hari bursa berakhir 25/11: 19, 20, 24, 25 (23/11 libur, 21–22 weekend)
    got = pending_dates(EVENING, {date(2026, 11, 23)}, {date(2026, 11, 25)}, lookback=4)
    assert got == [date(2026, 11, 19), date(2026, 11, 20), date(2026, 11, 24)]


# --- run_once ---


def _history(code: str, end: date, spike: bool, n: int = 30) -> pd.DataFrame:
    days = trading_days(end - timedelta(days=60), end, set())[-n:]
    vols = [900 if i % 2 == 0 else 1100 for i in range(len(days))]
    closes = [100.0] * len(days)
    if spike:
        vols[-1] = 5000
        closes[-1] = 105.0
    return pd.DataFrame(
        {
            "code": code,
            "date": days,
            "open": closes,
            "high": [c + 2 for c in closes],
            "low": [c - 2 for c in closes],
            "close": closes,
            "volume": vols,
        }
    )


class FakeYahoo:
    def __init__(self, frames=None, fail=()):
        self.frames = frames or {}
        self.fail = set(fail)
        self.calls = []

    def daily_ohlcv(self, code, start, end):
        self.calls.append((code, start, end))
        if code in self.fail:
            raise RuntimeError("boom")
        df = self.frames[code]
        return df[(df["date"] >= start) & (df["date"] <= end)].reset_index(drop=True)


class FakeNotifier:
    def __init__(self, fail=False):
        self.sent = []
        self.fail = fail

    def send(self, text):
        if self.fail:
            raise RuntimeError("telegram: http 500")
        self.sent.append(text)


def _deps(engine, yahoo, notifier, holidays=frozenset()):
    return WorkerDeps(
        engine=engine,
        yahoo=yahoo,
        notifier=notifier,
        holidays=lambda year: set(holidays),
        sleep=lambda s: None,
    )


@pytest.fixture
def watch(engine):
    for code in ("AAAA", "BBBB"):
        store.ensure_stock(engine, Stock(code, f"{code}.JK", code))
        store.add_watchlist(engine, code)
    return engine


def _mark_digested_except(engine, keep: date):
    for d in trading_days(keep - timedelta(days=30), keep - timedelta(days=1), set()):
        store.record_digest(engine, d, 0)


def test_run_sends_one_digest_per_date_and_is_idempotent(watch):
    _mark_digested_except(watch, TODAY)
    yahoo = FakeYahoo(
        {"AAAA": _history("AAAA", TODAY, True), "BBBB": _history("BBBB", TODAY, False)}
    )
    notifier = FakeNotifier()

    report = run_once(_deps(watch, yahoo, notifier), EVENING, Rule())
    assert report.digested == [TODAY]
    assert len(notifier.sent) == 1 and "AAAA" in notifier.sent[0] and "BBBB" not in notifier.sent[0]
    assert TODAY in store.digested_dates(watch, TODAY)
    assert list(store.signals_on(watch, TODAY)["code"]) == ["AAAA"]

    report = run_once(_deps(watch, yahoo, notifier), EVENING + timedelta(hours=3), Rule())
    assert report.digested == []
    assert len(notifier.sent) == 1


def test_telegram_failure_is_retried_next_run(watch):
    _mark_digested_except(watch, TODAY)
    yahoo = FakeYahoo({c: _history(c, TODAY, False) for c in ("AAAA", "BBBB")})

    report = run_once(_deps(watch, yahoo, FakeNotifier(fail=True)), EVENING, Rule())
    assert report.digested == [] and report.send_errors
    assert TODAY not in store.digested_dates(watch, TODAY)

    notifier = FakeNotifier()
    run_once(_deps(watch, yahoo, notifier), EVENING, Rule())
    assert notifier.sent == ["SpikeIDX 2026-11-25: No spike today."]


def test_date_without_any_bar_waits_for_next_run(watch):
    _mark_digested_except(watch, TODAY)
    yesterday = date(2026, 11, 24)
    yahoo = FakeYahoo({c: _history(c, yesterday, False) for c in ("AAAA", "BBBB")})
    notifier = FakeNotifier()

    report = run_once(_deps(watch, yahoo, notifier), EVENING, Rule())
    assert report.digested == [] and report.waiting == [TODAY]
    assert notifier.sent == []
    assert TODAY not in store.digested_dates(watch, TODAY)


def test_code_without_bar_on_date_is_not_judged_on_stale_bar(watch):
    _mark_digested_except(watch, TODAY)
    yesterday = date(2026, 11, 24)
    yahoo = FakeYahoo(
        {"AAAA": _history("AAAA", TODAY, False), "BBBB": _history("BBBB", yesterday, True)}
    )
    report = run_once(_deps(watch, yahoo, FakeNotifier()), EVENING, Rule())
    assert report.missing_bars[TODAY] == ["BBBB"]
    assert store.signals_on(watch, TODAY, include_filtered=True).empty


def test_dry_run_writes_nothing_and_sends_nothing(watch):
    yahoo = FakeYahoo({c: _history(c, TODAY, c == "AAAA") for c in ("AAAA", "BBBB")})
    notifier = FakeNotifier()

    report = run_once(_deps(watch, yahoo, notifier), EVENING, Rule(), dry_run=True)
    assert notifier.sent == []
    assert store.load_ohlcv(watch).empty
    assert store.signals_on(watch, TODAY, include_filtered=True).empty
    assert store.digested_dates(watch, date(2026, 1, 1)) == set()
    assert "AAAA" in report.digests[TODAY]


def test_all_ingest_failures_raise(watch):
    with pytest.raises(WorkerError):
        run_once(_deps(watch, FakeYahoo(fail={"AAAA", "BBBB"}), FakeNotifier()), EVENING, Rule())


def test_partial_ingest_failure_is_reported(watch):
    _mark_digested_except(watch, TODAY)
    yahoo = FakeYahoo({"AAAA": _history("AAAA", TODAY, False)}, fail={"BBBB"})
    report = run_once(_deps(watch, yahoo, FakeNotifier()), EVENING, Rule())
    assert set(report.ingest_errors) == {"BBBB"}
    assert report.digested == [TODAY]


def test_first_run_without_digest_history_sends_only_latest_date(watch):
    yahoo = FakeYahoo({c: _history(c, TODAY, False) for c in ("AAAA", "BBBB")})
    notifier = FakeNotifier()
    report = run_once(_deps(watch, yahoo, notifier), EVENING, Rule())
    assert report.digested == [TODAY]
    assert len(notifier.sent) == 1


def test_empty_watchlist_does_nothing(engine):
    notifier = FakeNotifier()
    report = run_once(_deps(engine, FakeYahoo(), notifier), EVENING, Rule())
    assert report.digested == [] and notifier.sent == []


def test_no_notifier_still_records_digest(watch):
    _mark_digested_except(watch, TODAY)
    yahoo = FakeYahoo({c: _history(c, TODAY, False) for c in ("AAAA", "BBBB")})
    report = run_once(_deps(watch, yahoo, None), EVENING, Rule())
    assert report.digested == [TODAY]
