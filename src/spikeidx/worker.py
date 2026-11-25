"""Satu run worker EOD yang aman dijalankan berkali-kali sehari.

Alur: ingest watchlist dari Yahoo → tentukan tanggal bursa tertunda (belum ada di
`digest_log`, bar sudah final) → hitung sinyal PADA tanggal itu → simpan spike →
kirim satu digest per tanggal → catat di `digest_log`. Telegram gagal berarti tanggal
itu tidak dicatat, sehingga dicoba lagi di run berikutnya.
"""

import logging
import time
from collections.abc import Callable
from dataclasses import dataclass, field
from datetime import date, datetime, timedelta

import pandas as pd
import sqlalchemy as sa

from spikeidx import store
from spikeidx.calendar import trading_days
from spikeidx.notify import format_digest
from spikeidx.signals import RULE_VERSION, Rule, add_features, classify

log = logging.getLogger("spikeidx.worker")

FETCH_DAYS = 100  # ±3 bulan kalender, cukup untuk baseline 20 bar
HISTORY_DAYS = 150


class WorkerError(RuntimeError):
    pass


@dataclass
class WorkerDeps:
    engine: sa.Engine
    yahoo: object  # punya daily_ohlcv(code, start, end) -> DataFrame
    notifier: object | None  # punya send(text); None = Telegram tidak dikonfigurasi
    holidays: Callable[[int], set[date]]
    sleep: Callable[[float], None] = time.sleep


@dataclass
class RunReport:
    digested: list[date] = field(default_factory=list)
    waiting: list[date] = field(default_factory=list)
    missing_bars: dict[date, list[str]] = field(default_factory=dict)
    ingest_errors: dict[str, str] = field(default_factory=dict)
    send_errors: dict[date, str] = field(default_factory=dict)
    digests: dict[date, str] = field(default_factory=dict)


def pending_dates(
    now: datetime,
    holidays: set[date],
    digested: set[date],
    lookback: int = 10,
    final_hour: int = 18,
) -> list[date]:
    """Tanggal bursa dalam jendela `lookback` hari bursa yang berakhir hari ini, minus
    yang sudah di-digest.

    Hari ini baru ikut setelah `final_hour` WIB, supaya volume parsial tidak dinilai.
    """
    today = now.date()
    window = trading_days(today - timedelta(days=lookback * 2 + 14), today, holidays)[-lookback:]
    return [d for d in window if d not in digested and (d < today or now.hour >= final_hour)]


def _holidays(deps: WorkerDeps, years: set[int]) -> set[date]:
    out: set[date] = set()
    for year in sorted(years):
        try:
            out |= deps.holidays(year)
        except Exception as exc:  # noqa: BLE001 - kalender opsional, turun ke weekend-only
            log.warning("libur %s tidak tersedia (%s); weekend-only", year, exc)
    return out


def _ingest(
    deps: WorkerDeps, codes: list[str], today: date, dry_run: bool, report: RunReport
) -> list[pd.DataFrame]:
    frames = []
    for i, code in enumerate(codes):
        if i:
            deps.sleep(1.0)
        try:
            df = deps.yahoo.daily_ohlcv(code, today - timedelta(days=FETCH_DAYS), today)
            if not dry_run:
                store.upsert_ohlcv(deps.engine, df)
            frames.append(df)
            log.info("ingest %s: %d bar", code, len(df))
        except Exception as exc:  # noqa: BLE001 - kegagalan per kode tidak menghentikan run
            report.ingest_errors[code] = str(exc)
            log.warning("ingest %s gagal: %s", code, exc)
    return frames


def run_once(deps: WorkerDeps, now: datetime, rule: Rule, dry_run: bool = False) -> RunReport:
    report = RunReport()
    today = now.date()
    codes = store.list_watchlist(deps.engine)
    if not codes:
        log.info("watchlist kosong, tidak ada yang diproses")
        return report

    frames = _ingest(deps, codes, today, dry_run, report)
    if not frames:
        raise WorkerError(f"semua ingest gagal ({len(codes)} kode)")

    since = today - timedelta(days=45)
    try:
        digested = store.digested_dates(deps.engine, since)
    except sa.exc.SQLAlchemyError:
        if not dry_run:
            raise
        log.warning("digest_log belum ada (migrasi 005?); dry-run menganggap kosong")
        digested = set()

    holidays = _holidays(deps, {today.year, since.year})
    pending = pending_dates(now, holidays, digested)
    if not digested and len(pending) > 1:
        # Run pertama (atau setelah lama mati): jangan banjiri Telegram dengan backlog.
        log.info("belum ada riwayat digest; hanya memproses %s", pending[-1])
        pending = pending[-1:]
    if not pending:
        log.info("tidak ada tanggal tertunda")
        return report

    history = store.load_ohlcv(deps.engine, codes, start=today - timedelta(days=HISTORY_DAYS))
    data = store.normalize_ohlcv(pd.concat([history, *frames], ignore_index=True))
    classified = classify(add_features(data), rule)

    for day in pending:
        rows = classified[classified["date"].dt.date == day]
        if rows.empty:
            report.waiting.append(day)
            log.info("%s: belum ada bar, tunggu run berikutnya", day)
            continue
        missing = sorted(set(codes) - set(rows["code"]))
        if missing:
            report.missing_bars[day] = missing
            log.warning("%s: tanpa bar untuk %s", day, ", ".join(missing))

        valid = rows[rows["avg20"].notna()]
        spikes = valid[valid["is_spike"]]
        actionable = valid[valid["is_actionable"]]
        text = format_digest(day, actionable)
        report.digests[day] = text
        log.info("%s: %d spike (%d actionable)", day, len(spikes), len(actionable))

        if dry_run:
            log.info("[dry-run] digest %s:\n%s", day, text)
            continue

        store.upsert_signals(deps.engine, spikes, RULE_VERSION)
        if deps.notifier is not None:
            try:
                deps.notifier.send(text)
            except Exception as exc:  # noqa: BLE001 - dicoba lagi di run berikutnya
                report.send_errors[day] = str(exc)
                log.error("%s: kirim Telegram gagal: %s", day, exc)
                continue
        else:
            log.info("Telegram tidak dikonfigurasi; digest %s:\n%s", day, text)
        store.record_digest(deps.engine, day, len(actionable))
        report.digested.append(day)

    return report
