import numpy as np
import pandas as pd
import pytest
from textual.widgets import DataTable, SelectionList, TabbedContent

from spikeidx import experiments as ex
from spikeidx.signals import Rule
from spikeidx.stocks import Stock
from spikeidx.tui.app import SpikeIDXApp
from spikeidx.tui.forms import parse_horizons, rule_from_inputs


def test_parse_horizons():
    assert parse_horizons("1, 5,20") == (1, 5, 20)
    assert parse_horizons("5,1,5") == (1, 5)
    for bad in ("", "0", "a", "1,-2", "61"):
        with pytest.raises(ValueError):
            parse_horizons(bad)


def test_rule_from_inputs():
    rule = rule_from_inputs(
        {"multiple_min": "1.5", "z_min": "2", "pct_min": "2", "cmf_min": "0.1"}, True
    )
    assert rule == Rule(multiple_min=1.5)
    with pytest.raises(ValueError, match="multiple_min"):
        rule_from_inputs({"multiple_min": "x", "z_min": "2", "pct_min": "2", "cmf_min": "0"}, True)
    with pytest.raises(ValueError, match="z_min"):
        rule_from_inputs({"multiple_min": "2", "z_min": "-1", "pct_min": "2", "cmf_min": "0"}, True)


def _synthetic() -> pd.DataFrame:
    rng = np.random.default_rng(3)
    frames = []
    for code in ("AAAA", "BBBB"):
        n = 160
        vols = rng.integers(800, 1200, n)
        vols[[50, 100]] = 9000
        closes = 100 + np.cumsum(rng.normal(0, 1, n))
        frames.append(
            pd.DataFrame(
                {
                    "code": code,
                    "date": pd.bdate_range("2026-01-01", periods=n),
                    "open": closes,
                    "high": closes + 1,
                    "low": closes - 1,
                    "close": closes * np.where(np.arange(n) % 50 == 0, 1.05, 1.0),
                    "volume": vols,
                }
            )
        )
    return pd.concat(frames, ignore_index=True)


class FakeSource:
    def __init__(self):
        self.watch = ["AAAA", "BBBB"]
        self.added = []

    def load_ohlcv(self):
        return _synthetic()

    def data_status(self):
        return pd.DataFrame(
            {
                "code": self.watch,
                "first": [None] * len(self.watch),
                "last": [None] * len(self.watch),
                "bars": [160] * len(self.watch),
            }
        )

    def search(self, query):
        return [Stock("TLKM", "TLKM.JK", "Telkom", "Comm")]

    def add(self, code):
        self.added.append(code)
        self.watch.append(code)

    def remove(self, code):
        self.watch.remove(code)


async def _ready(pilot):
    await pilot.app.workers.wait_for_complete()
    await pilot.pause()


async def test_lab_run_fills_results_and_events(tmp_path):
    app = SpikeIDXApp(FakeSource(), experiments_dir=tmp_path)
    async with app.run_test(size=(180, 60)) as pilot:
        await _ready(pilot)
        app.query_one("#price_filter").value = False
        await pilot.click("#run")
        await _ready(pilot)
        results = app.query_one("#results", DataTable)
        events = app.query_one("#events", DataTable)
        assert results.row_count == 3
        assert events.row_count == 4  # 2 kode × 2 spike


async def test_invalid_input_shows_message_and_does_not_run(tmp_path):
    app = SpikeIDXApp(FakeSource(), experiments_dir=tmp_path)
    async with app.run_test(size=(180, 60)) as pilot:
        await _ready(pilot)
        app.query_one("#horizons").value = "abc"
        await pilot.click("#run")
        await _ready(pilot)
        assert app.query_one("#results", DataTable).row_count == 0
        assert "horizon" in str(app.query_one("#lab-status").render()).lower()


async def test_save_then_compare(tmp_path):
    app = SpikeIDXApp(FakeSource(), experiments_dir=tmp_path)
    async with app.run_test(size=(180, 60)) as pilot:
        await _ready(pilot)
        await pilot.click("#run")
        await _ready(pilot)
        app.query_one("#exp-name").value = "baseline"
        await pilot.click("#save")
        await _ready(pilot)
        assert [e.name for e in ex.load_all(tmp_path)] == ["baseline"]

        await pilot.click("#--content-tab-compare")
        await pilot.pause()
        assert app.query_one(TabbedContent).active == "compare"
        selection = app.query_one("#exp-list", SelectionList)
        assert selection.option_count == 1
        selection.select_all()
        app.query_one("#cmp-horizon").value = "5"
        await pilot.click("#compare-btn")
        await _ready(pilot)
        assert app.query_one("#cmp-table", DataTable).row_count == 1


async def test_data_tab_search_and_add(tmp_path):
    source = FakeSource()
    app = SpikeIDXApp(source, experiments_dir=tmp_path)
    async with app.run_test(size=(180, 60)) as pilot:
        await _ready(pilot)
        await pilot.click("#--content-tab-data")
        await pilot.pause()
        assert app.query_one("#status", DataTable).row_count == 2
        app.query_one("#search-q").value = "telkom"
        await pilot.click("#search")
        await _ready(pilot)
        assert app.query_one("#search-results", DataTable).row_count == 1
        await pilot.click("#wl-add")
        await _ready(pilot)
        assert source.added == ["TLKM"]
        assert app.query_one("#status", DataTable).row_count == 3
