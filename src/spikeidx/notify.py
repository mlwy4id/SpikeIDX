from datetime import date

import httpx
import pandas as pd

MAX_MESSAGE = 4096


def compact(volume: int) -> str:
    """Ringkas volume lembar: rb (ribu), jt (juta), M (miliar)."""
    if volume >= 1_000_000_000:
        return f"{volume / 1_000_000_000:.1f}M"
    if volume >= 1_000_000:
        jt = volume / 1_000_000
        return f"{int(jt)}jt" if jt == int(jt) else f"{jt:.1f}jt"
    if volume >= 1_000:
        return f"{volume // 1_000}rb"
    return str(volume)


def format_digest(day: date, signals: pd.DataFrame) -> str:
    """Digest satu tanggal. `signals` butuh kolom code, volume, multiple, pct_change, label."""
    if signals.empty:
        return f"SpikeIDX {day.isoformat()}: No spike today."
    header = f"🚨 Spike {day.isoformat()} ({len(signals)}):\n"
    lines = [
        f"{r.code} {compact(int(r.volume))} ({r.multiple:.1f}x avg20) {r.pct_change:+.1f}%"
        f" | CMF {r.label}\n"
        for r in signals.itertuples(index=False)
    ]
    text = header
    for i, line in enumerate(lines):
        rest = f"… dan {len(lines) - i} lainnya\n"
        if len(text) + len(line) + len(rest) > MAX_MESSAGE:
            return text + rest
        text += line
    return text


class TelegramNotifier:
    def __init__(self, bot_token: str, chat_id: str, http: httpx.Client | None = None):
        self._token = bot_token
        self._chat_id = chat_id
        self._http = http or httpx.Client(timeout=20)

    def send(self, text: str) -> None:
        resp = self._http.post(
            f"https://api.telegram.org/bot{self._token}/sendMessage",
            data={"chat_id": self._chat_id, "text": text},
        )
        if resp.status_code != 200:
            # Jangan sertakan URL: berisi token bot.
            raise RuntimeError(f"telegram: http {resp.status_code}")
