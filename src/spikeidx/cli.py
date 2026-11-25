"""Entry point `spikeidx <perintah>`: migrate, worker, backfill, tui."""

import argparse
import logging
import sys
import time

import httpx
import sqlalchemy as sa

from spikeidx import store
from spikeidx.calendar import fetch_holidays, now_wib
from spikeidx.config import MIGRATIONS_DIR, ConfigError, load_settings
from spikeidx.db import make_engine

log = logging.getLogger("spikeidx")


def build_parser() -> argparse.ArgumentParser:
    p = argparse.ArgumentParser(prog="spikeidx", description="SpikeIDX: worker, riset, dan TUI")
    sub = p.add_subparsers(dest="command", required=True)
    sub.add_parser("migrate", help="terapkan migrasi SQL yang belum tercatat")
    w = sub.add_parser("worker", help="satu run EOD: ingest, sinyal, digest")
    w.add_argument("--dry-run", action="store_true", help="tanpa tulis DB dan tanpa Telegram")
    b = sub.add_parser("backfill", help="ambil histori panjang untuk riset")
    b.add_argument("--years", type=int, default=3)
    b.add_argument("codes", nargs="*", help="default: seluruh watchlist")
    sub.add_parser("tui", help="buka lab riset di terminal")
    return p


def _migrate(engine: sa.Engine) -> int:
    from spikeidx.migrate import apply_migrations

    applied = apply_migrations(engine, MIGRATIONS_DIR)
    log.info("migrasi diterapkan: %s", ", ".join(applied) or "tidak ada")
    return 0


def _worker(engine: sa.Engine, settings, dry_run: bool) -> int:
    from spikeidx.notify import TelegramNotifier
    from spikeidx.signals import Rule
    from spikeidx.sources.yahoo import YahooClient
    from spikeidx.worker import WorkerDeps, WorkerError, run_once

    notifier = (
        TelegramNotifier(settings.telegram_bot_token, settings.telegram_chat_id)
        if settings.telegram_enabled
        else None
    )
    http = httpx.Client(timeout=15)
    deps = WorkerDeps(
        engine=engine,
        yahoo=YahooClient(),
        notifier=notifier,
        holidays=lambda year: fetch_holidays(year, http),
    )
    try:
        report = run_once(deps, now_wib(), Rule(), dry_run=dry_run)
    except (WorkerError, sa.exc.SQLAlchemyError) as exc:
        log.error("worker gagal: %s", exc)
        return 1
    log.info(
        "selesai: digest=%s tunggu=%s gagal-ingest=%d gagal-kirim=%d",
        [d.isoformat() for d in report.digested],
        [d.isoformat() for d in report.waiting],
        len(report.ingest_errors),
        len(report.send_errors),
    )
    return 1 if report.send_errors else 0


def _backfill(engine: sa.Engine, codes: list[str], years: int) -> int:
    from spikeidx.backfill import backfill
    from spikeidx.sources.yahoo import YahooClient
    from spikeidx.stocks import parse_code

    try:
        codes = [parse_code(c) for c in codes] or store.list_watchlist(engine)
    except ValueError as exc:
        log.error("%s", exc)
        return 2
    result = backfill(engine, YahooClient(), codes, years, now_wib().date(), time.sleep)
    ok = [c for c, v in result.items() if isinstance(v, int)]
    log.info("backfill selesai: %d/%d kode", len(ok), len(codes))
    return 0 if ok or not codes else 1


def main(argv: list[str] | None = None) -> int:
    args = build_parser().parse_args(argv)
    logging.basicConfig(level=logging.INFO, format="%(asctime)s %(levelname)s %(name)s %(message)s")
    try:
        settings = load_settings()
    except ConfigError as exc:
        log.error("%s", exc)
        return 2
    engine = make_engine(settings)

    if args.command == "migrate":
        return _migrate(engine)
    if args.command == "worker":
        return _worker(engine, settings, args.dry_run)
    if args.command == "backfill":
        return _backfill(engine, args.codes, args.years)
    if args.command == "tui":
        from spikeidx.tui.app import run

        return run(engine)
    return 2


if __name__ == "__main__":
    sys.exit(main())
