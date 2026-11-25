"""Hari bursa IDX: weekend + libur nasional (Nager.Date). Cuti bersama BEI belum tercakup."""

from datetime import date, datetime, timedelta
from zoneinfo import ZoneInfo

import httpx

WIB = ZoneInfo("Asia/Jakarta")
NAGER_URL = "https://date.nager.at"


def now_wib() -> datetime:
    return datetime.now(WIB)


def fetch_holidays(year: int, http: httpx.Client, base_url: str = NAGER_URL) -> set[date]:
    resp = http.get(f"{base_url}/api/v3/PublicHolidays/{year}/ID", timeout=15)
    resp.raise_for_status()
    out = set()
    for row in resp.json():
        try:
            out.add(date.fromisoformat(row["date"]))
        except (KeyError, TypeError, ValueError):
            continue
    return out


def is_trading_day(d: date, holidays: set[date]) -> bool:
    return d.weekday() < 5 and d not in holidays


def trading_days(start: date, end: date, holidays: set[date]) -> list[date]:
    days = []
    d = start
    while d <= end:
        if is_trading_day(d, holidays):
            days.append(d)
        d += timedelta(days=1)
    return days
