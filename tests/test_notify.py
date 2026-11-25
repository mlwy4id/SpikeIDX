from datetime import date

import httpx
import pandas as pd
import pytest

from spikeidx.notify import MAX_MESSAGE, TelegramNotifier, compact, format_digest


@pytest.mark.parametrize(
    "v,s",
    [
        (999, "999"),
        (45_000, "45rb"),
        (45_000_000, "45jt"),
        (80_500_000, "80.5jt"),
        (2_300_000_000, "2.3M"),
    ],
)
def test_compact_volume(v, s):
    assert compact(v) == s


def _signals(n=2):
    return pd.DataFrame(
        {
            "code": ["BBCA", "TLKM"][:n],
            "volume": [45_000_000, 80_500_000][:n],
            "multiple": [3.1, 2.5][:n],
            "pct_change": [2.4, -3.2][:n],
            "label": ["akumulasi", "distribusi"][:n],
        }
    )


def test_format_digest_empty():
    assert format_digest(date(2026, 11, 24), _signals(0)) == "SpikeIDX 2026-11-24: No spike today."


def test_format_digest_lines():
    text = format_digest(date(2026, 11, 24), _signals())
    assert text.startswith("🚨 Spike 2026-11-24 (2):\n")
    assert "BBCA 45jt (3.1x avg20) +2.4% | CMF akumulasi" in text
    assert "TLKM 80.5jt (2.5x avg20) -3.2% | CMF distribusi" in text


def test_format_digest_truncates_to_telegram_limit():
    many = pd.concat([_signals(1)] * 400, ignore_index=True)
    text = format_digest(date(2026, 11, 24), many)
    assert len(text) <= MAX_MESSAGE
    assert text.rstrip().endswith("lainnya")


def test_send_posts_form_and_raises_on_error():
    seen = {}

    def handler(request):
        seen["url"] = str(request.url)
        seen["body"] = request.content.decode()
        return httpx.Response(200, json={"ok": True})

    n = TelegramNotifier("tok", "chat", httpx.Client(transport=httpx.MockTransport(handler)))
    n.send("halo")
    assert seen["url"] == "https://api.telegram.org/bottok/sendMessage"
    assert "chat_id=chat" in seen["body"] and "text=halo" in seen["body"]

    bad = TelegramNotifier(
        "tok", "chat", httpx.Client(transport=httpx.MockTransport(lambda r: httpx.Response(400)))
    )
    with pytest.raises(RuntimeError, match="400") as exc:
        bad.send("x")
    assert "tok" not in str(exc.value)  # token tidak bocor ke log
