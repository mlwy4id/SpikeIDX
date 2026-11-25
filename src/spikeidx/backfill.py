"""Ambil histori panjang (default 3 tahun) untuk riset. Aman diulang (upsert)."""

import logging
from collections.abc import Callable
from datetime import date

import sqlalchemy as sa

from spikeidx import store
from spikeidx.stocks import Stock, yahoo_symbol

log = logging.getLogger("spikeidx.backfill")


def _years_ago(today: date, years: int) -> date:
    try:
        return today.replace(year=today.year - years)
    except ValueError:  # 29 Februari
        return today.replace(year=today.year - years, day=28)


def _ensure(engine: sa.Engine, yahoo, code: str) -> None:
    if store.get_stock(engine, code) is not None:
        return
    try:
        match = next((s for s in yahoo.search(code) if s.code == code), None)
    except Exception as exc:  # noqa: BLE001 - nama hanya kosmetik; kode tetap valid
        log.warning("search %s gagal (%s); kode dipakai sebagai nama", code, exc)
        match = None
    store.ensure_stock(engine, match or Stock(code, yahoo_symbol(code), code))


def backfill(
    engine: sa.Engine,
    yahoo,
    codes: list[str],
    years: int,
    today: date,
    sleep: Callable[[float], None],
) -> dict[str, int | str]:
    """Hasil per kode: jumlah bar yang di-upsert, atau pesan error."""
    start = _years_ago(today, years)
    result: dict[str, int | str] = {}
    for i, code in enumerate(codes):
        if i:
            sleep(1.0)
        try:
            _ensure(engine, yahoo, code)
            result[code] = store.upsert_ohlcv(engine, yahoo.daily_ohlcv(code, start, today))
            log.info("backfill %s: %d bar sejak %s", code, result[code], start)
        except Exception as exc:  # noqa: BLE001 - satu kode gagal tidak menghentikan batch
            result[code] = str(exc)
            log.warning("backfill %s gagal: %s", code, exc)
    return result
