from datetime import date, datetime

import numpy as np
import pandas as pd
import pytest

from spikeidx import experiments as ex
from spikeidx.research.event_study import RESULT_COLUMNS
from spikeidx.signals import Rule


def _results(mean_event: float) -> pd.DataFrame:
    df = pd.DataFrame(
        [[10, 100, mean_event, 0.0, mean_event, 0.6, mean_event, 2.0, np.nan]],
        index=pd.Index([5], name="horizon"),
        columns=RESULT_COLUMNS,
    )
    return df


def _exp(name, fingerprint="fp1", mean=0.01, codes=("BBCA", "TLKM")) -> ex.Experiment:
    return ex.Experiment(
        name=name,
        rule=Rule(multiple_min=1.5),
        horizons=(5,),
        start=date(2023, 11, 1),
        end=date(2026, 11, 24),
        codes=list(codes),
        data_fingerprint=fingerprint,
        results=_results(mean),
        created_at=datetime(2026, 11, 25, 10, 0),
    )


def test_save_and_load_roundtrip(tmp_path):
    path = ex.save(_exp("multiple 1.5"), tmp_path)
    assert path.parent == tmp_path and path.suffix == ".json"

    [loaded] = ex.load_all(tmp_path)
    assert loaded.name == "multiple 1.5"
    assert loaded.rule == Rule(multiple_min=1.5)
    assert loaded.horizons == (5,)
    assert loaded.start == date(2023, 11, 1)
    assert loaded.codes == ["BBCA", "TLKM"]
    assert loaded.results.loc[5, "n_event"] == 10
    assert np.isnan(loaded.results.loc[5, "p_value"])


def test_save_twice_with_same_name_keeps_both(tmp_path):
    ex.save(_exp("a"), tmp_path)
    second = _exp("a")
    second.created_at = datetime(2026, 11, 25, 11, 0)
    ex.save(second, tmp_path)
    assert len(ex.load_all(tmp_path)) == 2


def test_default_dir_honors_env(monkeypatch, tmp_path):
    monkeypatch.setenv("SPIKEIDX_HOME", str(tmp_path))
    assert ex.default_dir() == tmp_path / "experiments"


def test_compare_same_data_has_no_warning():
    table, warnings = ex.compare([_exp("a", mean=0.01), _exp("b", mean=0.03)], horizon=5)
    assert list(table.index) == ["a", "b"]
    assert table.loc["b", "mean_event"] == pytest.approx(0.03)
    assert warnings == []


def test_compare_warns_when_data_differs():
    _, warnings = ex.compare([_exp("a"), _exp("b", fingerprint="fp2")], horizon=5)
    assert len(warnings) == 1 and "b" in warnings[0]


def test_compare_missing_horizon_gives_nan_row():
    table, _ = ex.compare([_exp("a")], horizon=20)
    assert table.loc["a"].isna().all()


def test_fingerprint_changes_with_data():
    df = pd.DataFrame(
        {"code": ["A", "B"], "date": pd.to_datetime(["2026-11-24", "2026-11-24"]), "close": [1, 2]}
    )
    assert ex.fingerprint(df) == ex.fingerprint(df.copy())
    assert ex.fingerprint(df) != ex.fingerprint(df.iloc[:1])
