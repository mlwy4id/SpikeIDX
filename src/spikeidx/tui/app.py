"""Lab riset SpikeIDX di terminal (Textual).

Tab Lab: ubah rule → Run → event study + daftar event + grafik per event.
Tab Compare: bandingkan 2–4 eksperimen tersimpan per horizon.
Tab Data: status data watchlist, search Yahoo, tambah/hapus watchlist.
Data OHLCV dimuat sekali (r = reload); Run hanya menghitung ulang di memori.
"""

from datetime import datetime
from pathlib import Path

import numpy as np
import pandas as pd
import sqlalchemy as sa
from textual import on, work
from textual.app import App, ComposeResult
from textual.containers import Horizontal, Vertical
from textual.widgets import (
    Button,
    Checkbox,
    DataTable,
    Footer,
    Header,
    Input,
    Label,
    SelectionList,
    Static,
    TabbedContent,
    TabPane,
)
from textual_plotext import PlotextPlot

from spikeidx import experiments as ex
from spikeidx.calendar import WIB
from spikeidx.research.event_study import run_study
from spikeidx.signals import Rule
from spikeidx.tui.forms import RULE_FIELDS, parse_horizons, rule_from_inputs

CONTEXT_BARS = 20
MAX_COMPARE = 4


def _fmt(value, pct=False, digits=2) -> str:
    if value is None or (isinstance(value, float) and np.isnan(value)):
        return "–"
    if pct:
        return f"{value * 100:+.{digits}f}%"
    if isinstance(value, float | np.floating):
        return f"{value:.3f}"
    return str(value)


def _count(value) -> str:
    return "–" if pd.isna(value) else str(int(value))


def _result_row(horizon, row) -> list[str]:
    return [
        str(horizon),
        _count(row["n_event"]),
        _count(row["n_control"]),
        _fmt(row["mean_event"], pct=True),
        _fmt(row["mean_control"], pct=True),
        _fmt(row["diff"], pct=True),
        _fmt(row["hit_rate_event"]),
        _fmt(row["p_value"]),
    ]


RESULT_HEADERS = ("h", "n_evt", "n_ctl", "mean_evt", "mean_ctl", "diff", "hit", "p-value")


class SpikeIDXApp(App):
    TITLE = "SpikeIDX — lab riset"
    CSS = """
    #rule-panel { width: 34; padding: 0 1; }
    #rule-panel Input { margin-bottom: 0; }
    #results { height: 8; }
    #events { width: 1fr; }
    #detail { width: 1fr; }
    #chart-price, #chart-volume { height: 1fr; }
    #compare-panel { width: 40; padding: 0 1; }
    #exp-list { height: 1fr; }
    #status, #search-results { height: 1fr; }
    .row { height: auto; }
    .row Input { width: 1fr; }
    """
    BINDINGS = [("r", "reload", "Reload data"), ("q", "quit", "Keluar")]

    def __init__(self, source, experiments_dir: Path | None = None):
        super().__init__()
        self.source = source
        self.experiments_dir = experiments_dir
        self.ohlcv: pd.DataFrame | None = None
        self.last_run: dict | None = None
        self.events: pd.DataFrame = pd.DataFrame()
        self.search_hits: list = []

    # --- layout ---

    def compose(self) -> ComposeResult:
        yield Header()
        defaults = Rule()
        with TabbedContent(initial="lab"):
            with TabPane("Lab", id="lab"):
                with Horizontal():
                    with Vertical(id="rule-panel"):
                        for name in RULE_FIELDS:
                            yield Label(name)
                            yield Input(str(getattr(defaults, name)), id=name)
                        yield Checkbox("filter harga", defaults.price_filter, id="price_filter")
                        yield Label("horizon (hari)")
                        yield Input("1,5,20", id="horizons")
                        yield Button("Run", id="run", variant="primary")
                        yield Label("nama eksperimen")
                        yield Input(placeholder="mis. baseline", id="exp-name")
                        yield Button("Simpan", id="save")
                        yield Static("memuat data…", id="lab-status")
                    with Vertical():
                        yield DataTable(id="results", cursor_type="row")
                        with Horizontal():
                            yield DataTable(id="events", cursor_type="row")
                            with Vertical(id="detail"):
                                yield PlotextPlot(id="chart-price")
                                yield PlotextPlot(id="chart-volume")
                                yield Static("", id="event-detail")
            with TabPane("Compare", id="compare"):
                with Horizontal():
                    with Vertical(id="compare-panel"):
                        yield SelectionList(id="exp-list")
                        yield Label("horizon")
                        yield Input("5", id="cmp-horizon")
                        yield Button("Bandingkan", id="compare-btn", variant="primary")
                        yield Button("Muat ulang daftar", id="exp-refresh")
                    with Vertical():
                        yield DataTable(id="cmp-table", cursor_type="row")
                        yield Static("", id="cmp-warnings")
            with TabPane("Data", id="data"):
                with Vertical():
                    yield DataTable(id="status", cursor_type="row")
                    with Horizontal(classes="row"):
                        yield Input(placeholder="cari saham (nama/kode)", id="search-q")
                        yield Button("Cari", id="search")
                    yield DataTable(id="search-results", cursor_type="row")
                    with Horizontal(classes="row"):
                        yield Button("Tambah ke watchlist", id="wl-add", variant="success")
                        yield Button("Hapus dari watchlist", id="wl-remove", variant="error")
                    yield Static("", id="data-msg")
        yield Footer()

    def on_mount(self) -> None:
        self.query_one("#results", DataTable).add_columns(*RESULT_HEADERS)
        self.query_one("#cmp-table", DataTable).add_columns("eksperimen", *RESULT_HEADERS[1:])
        self.query_one("#status", DataTable).add_columns(
            "code", "bar pertama", "bar terakhir", "bars"
        )
        self.query_one("#search-results", DataTable).add_columns("code", "nama", "sektor")
        self._refresh_experiments()
        self.action_reload()

    # --- data ---

    def action_reload(self) -> None:
        self._set("#lab-status", "memuat data…")
        self._load()

    @work(thread=True, exclusive=True, group="load")
    def _load(self) -> None:
        try:
            ohlcv = self.source.load_ohlcv()
            status = self.source.data_status()
        except Exception as exc:  # noqa: BLE001 - tampilkan ke user, jangan crash TUI
            self.call_from_thread(self._set, "#lab-status", f"gagal memuat data: {exc}")
            return
        self.call_from_thread(self._loaded, ohlcv, status)

    def _loaded(self, ohlcv: pd.DataFrame, status: pd.DataFrame) -> None:
        self.ohlcv = ohlcv
        self._fill_status(status)
        if ohlcv.empty:
            self._set("#lab-status", "belum ada data; tambah watchlist lalu jalankan backfill")
            return
        self._set(
            "#lab-status",
            f"{len(ohlcv):,} bar · {ohlcv['code'].nunique()} kode · "
            f"{ohlcv['date'].min():%Y-%m-%d}..{ohlcv['date'].max():%Y-%m-%d}",
        )

    def _fill_status(self, status: pd.DataFrame) -> None:
        table = self.query_one("#status", DataTable)
        table.clear()
        for r in status.itertuples(index=False):
            table.add_row(r.code, _fmt(r.first), _fmt(r.last), str(r.bars), key=r.code)

    def _set(self, selector: str, text: str) -> None:
        self.query_one(selector, Static).update(text)

    # --- Lab ---

    @on(Button.Pressed, "#run")
    def run_pressed(self) -> None:
        if self.ohlcv is None or self.ohlcv.empty:
            self._set("#lab-status", "data belum siap")
            return
        try:
            rule = rule_from_inputs(
                {n: self.query_one(f"#{n}", Input).value for n in RULE_FIELDS},
                self.query_one("#price_filter", Checkbox).value,
            )
            horizons = parse_horizons(self.query_one("#horizons", Input).value)
        except ValueError as exc:
            self._set("#lab-status", f"input tidak valid: {exc}")
            return
        self._set("#lab-status", "menghitung…")
        self._study(rule, horizons)

    @work(thread=True, exclusive=True, group="study")
    def _study(self, rule: Rule, horizons: tuple[int, ...]) -> None:
        results, events = run_study(self.ohlcv, rule, horizons)
        self.call_from_thread(self._show_study, rule, horizons, results, events)

    def _show_study(self, rule, horizons, results: pd.DataFrame, events: pd.DataFrame) -> None:
        self.last_run = {"rule": rule, "horizons": horizons, "results": results}
        table = self.query_one("#results", DataTable)
        table.clear()
        for h, row in results.iterrows():
            table.add_row(*_result_row(h, row))

        self.events = events.sort_values("date", ascending=False).reset_index(drop=True)
        ev = self.query_one("#events", DataTable)
        ev.clear(columns=True)
        ev.add_columns(
            "code", "tanggal", "mult", "z", "chg%", "CMF", "label", *[f"fwd{h}" for h in horizons]
        )
        for i, r in self.events.iterrows():
            ev.add_row(
                r["code"],
                f"{r['date']:%Y-%m-%d}",
                f"{r['multiple']:.1f}",
                f"{r['zscore']:.1f}",
                f"{r['pct_change']:+.1f}",
                f"{r['cmf20']:+.2f}",
                r["label"],
                *[_fmt(r[f"fwd_{h}"], pct=True, digits=1) for h in horizons],
                key=str(i),
            )
        self._set(
            "#lab-status",
            f"{len(self.events)} event · catatan: return bisa overlap, universe = watchlist",
        )

    @on(DataTable.RowHighlighted, "#events")
    def event_highlighted(self, message: DataTable.RowHighlighted) -> None:
        if message.row_key is None or self.ohlcv is None:
            return
        event = self.events.loc[int(message.row_key.value)]
        bars = self.ohlcv[self.ohlcv["code"] == event["code"]].reset_index(drop=True)
        pos = bars.index[bars["date"] == event["date"]]
        if len(pos) == 0:
            return
        i = int(pos[0])
        window = bars.iloc[max(0, i - CONTEXT_BARS) : i + CONTEXT_BARS + 1]
        x = list(range(-min(i, CONTEXT_BARS), len(window) - min(i, CONTEXT_BARS)))

        price = self.query_one("#chart-price", PlotextPlot)
        price.plt.clear_figure()
        price.plt.title(f"{event['code']} close (hari ke-0 = event)")
        price.plt.plot(x, window["close"].tolist())
        price.plt.vline(0)
        price.refresh()

        volume = self.query_one("#chart-volume", PlotextPlot)
        volume.plt.clear_figure()
        volume.plt.title("volume")
        volume.plt.bar(x, window["volume"].tolist())
        volume.refresh()

        fwd = " ".join(
            f"fwd{c.split('_')[1]} {_fmt(event[c], pct=True)}"
            for c in event.index
            if c.startswith("fwd_")
        )
        self._set(
            "#event-detail",
            f"{event['code']} {event['date']:%Y-%m-%d} · {event['multiple']:.1f}x avg20 · "
            f"z {event['zscore']:.1f} · CMF {event['cmf20']:+.2f} {event['label']} · {fwd}",
        )

    @on(Button.Pressed, "#save")
    def save_pressed(self) -> None:
        if self.last_run is None or self.ohlcv is None:
            self._set("#lab-status", "jalankan Run dulu sebelum menyimpan")
            return
        name = self.query_one("#exp-name", Input).value.strip()
        if not name:
            self._set("#lab-status", "isi nama eksperimen")
            return
        exp = ex.Experiment(
            name=name,
            rule=self.last_run["rule"],
            horizons=self.last_run["horizons"],
            start=self.ohlcv["date"].min().date(),
            end=self.ohlcv["date"].max().date(),
            codes=sorted(self.ohlcv["code"].unique()),
            data_fingerprint=ex.fingerprint(self.ohlcv),
            results=self.last_run["results"],
            created_at=datetime.now(WIB).replace(tzinfo=None),
        )
        path = ex.save(exp, self.experiments_dir)
        self._set("#lab-status", f"tersimpan: {path.name}")
        self._refresh_experiments()

    # --- Compare ---

    @on(Button.Pressed, "#exp-refresh")
    def _refresh_experiments(self) -> None:
        self.saved = ex.load_all(self.experiments_dir)
        selection = self.query_one("#exp-list", SelectionList)
        selection.clear_options()
        selection.add_options(
            (f"{e.name} · {e.created_at:%m-%d %H:%M}", i) for i, e in enumerate(self.saved)
        )

    @on(Button.Pressed, "#compare-btn")
    def compare_pressed(self) -> None:
        chosen = [self.saved[i] for i in self.query_one("#exp-list", SelectionList).selected]
        warn = self.query_one("#cmp-warnings", Static)
        if not chosen:
            warn.update("pilih minimal satu eksperimen")
            return
        if len(chosen) > MAX_COMPARE:
            warn.update(f"maksimal {MAX_COMPARE} eksperimen")
            return
        try:
            horizon = parse_horizons(self.query_one("#cmp-horizon", Input).value)[0]
        except ValueError as exc:
            warn.update(str(exc))
            return
        table_df, warnings = ex.compare(chosen, horizon)
        table = self.query_one("#cmp-table", DataTable)
        table.clear()
        for name, row in table_df.iterrows():
            table.add_row(name, *_result_row(horizon, row)[1:])
        warn.update("\n".join(warnings) or f"horizon {horizon} hari · data sama")

    # --- Data ---

    @on(Button.Pressed, "#search")
    def search_pressed(self) -> None:
        query = self.query_one("#search-q", Input).value.strip()
        if query:
            self._search(query)

    @work(thread=True, exclusive=True, group="search")
    def _search(self, query: str) -> None:
        try:
            hits = self.source.search(query)
        except Exception as exc:  # noqa: BLE001
            self.call_from_thread(self._set, "#data-msg", f"search gagal: {exc}")
            return
        self.call_from_thread(self._show_hits, hits)

    def _show_hits(self, hits) -> None:
        self.search_hits = hits
        table = self.query_one("#search-results", DataTable)
        table.clear()
        for s in hits:
            table.add_row(s.code, s.name, s.sector or "–", key=s.code)
        self._set("#data-msg", f"{len(hits)} hasil" if hits else "tidak ada saham .JK yang cocok")

    @on(Button.Pressed, "#wl-add")
    def add_pressed(self) -> None:
        table = self.query_one("#search-results", DataTable)
        if not self.search_hits or table.cursor_row < 0:
            self._set("#data-msg", "pilih hasil search dulu")
            return
        code = self.search_hits[table.cursor_row].code
        self._mutate_watchlist(self.source.add, code, "ditambahkan (jalankan backfill)")

    @on(Button.Pressed, "#wl-remove")
    def remove_pressed(self) -> None:
        table = self.query_one("#status", DataTable)
        if table.row_count == 0:
            return
        code = table.coordinate_to_cell_key((table.cursor_row, 0)).row_key.value
        self._mutate_watchlist(self.source.remove, code, "dihapus")

    def _mutate_watchlist(self, action, code: str, done: str) -> None:
        try:
            action(code)
            self._fill_status(self.source.data_status())
            self._set("#data-msg", f"{code} {done}")
        except Exception as exc:  # noqa: BLE001
            self._set("#data-msg", f"{code}: {exc}")


def run(engine: sa.Engine) -> int:
    from spikeidx.tui.source import DbSource

    SpikeIDXApp(DbSource(engine)).run()
    return 0
