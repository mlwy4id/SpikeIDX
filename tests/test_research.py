import numpy as np
import pandas as pd
import pytest

from spikeidx.research.event_study import RESULT_COLUMNS, event_study, run_study
from spikeidx.research.returns import flag_corporate_actions, forward_returns
from spikeidx.signals import Rule


def _ohlc(code, opens, closes):
    n = len(closes)
    return pd.DataFrame(
        {
            "code": code,
            "date": pd.bdate_range("2026-01-01", periods=n),
            "open": opens,
            "high": [max(o, c) + 1 for o, c in zip(opens, closes, strict=True)],
            "low": [min(o, c) - 1 for o, c in zip(opens, closes, strict=True)],
            "close": closes,
            "volume": [1000] * n,
        }
    )


def test_forward_return_enters_next_open():
    df = forward_returns(_ohlc("A", [10, 11, 12], [10.5, 11.5, 13]), horizons=(1,))
    assert df["fwd_1"].iloc[0] == pytest.approx(11.5 / 11 - 1)
    assert df["fwd_1"].iloc[1] == pytest.approx(13 / 12 - 1)
    assert np.isnan(df["fwd_1"].iloc[2])


def test_forward_return_nan_at_tail():
    df = forward_returns(_ohlc("A", [10] * 5, [10, 11, 12, 13, 14]), horizons=(2,))
    assert df["fwd_2"].iloc[:3].notna().all()
    assert df["fwd_2"].iloc[3:].isna().all()
    assert df["fwd_2"].iloc[0] == pytest.approx(12 / 10 - 1)


def test_forward_returns_per_code():
    df = pd.concat([_ohlc("A", [10, 10], [10, 10]), _ohlc("B", [50, 50], [50, 60])])
    out = forward_returns(df, horizons=(1,))
    assert np.isnan(out[out["code"] == "A"]["fwd_1"].iloc[-1])


def test_flag_corporate_action_on_split():
    df = _ohlc("A", [1000, 1000, 200, 200], [1000, 1000, 200, 160])
    flags = flag_corporate_actions(df, threshold=0.35)
    assert list(flags) == [False, False, True, False]


def _study_frame(n, spike_rows, fwd, opens=None, closes=None):
    closes = closes or [100.0] * n
    opens = opens or closes
    df = _ohlc("A", opens, closes)
    df["avg20"] = 1000.0
    df["is_spike"] = False
    df.loc[spike_rows, "is_spike"] = True
    df["is_actionable"] = df["is_spike"]
    df["fwd_1"] = fwd
    return df


def test_event_study_detects_positive_drift():
    rng = np.random.default_rng(7)
    n = 400
    spikes = list(range(0, n, 10))
    fwd = rng.normal(0, 0.005, n)
    fwd[spikes] += 0.01
    res = event_study(_study_frame(n, spikes, fwd), horizons=(1,))
    row = res.loc[1]
    assert row["n_event"] == len(spikes)
    assert row["n_control"] == n - len(spikes)
    assert row["diff"] == pytest.approx(0.01, abs=0.003)
    assert row["p_value"] < 0.05
    assert list(res.columns) == RESULT_COLUMNS


def test_event_study_excludes_corporate_action_window():
    n = 200
    closes = [100.0] * 100 + [30.0] * 100  # lompatan -70% di baris 100
    df = _study_frame(n, [60, 90], np.zeros(n), closes=closes)
    df["fwd_20"] = 0.0
    res = event_study(df, horizons=(20,), exclusion=20)
    assert res.loc[20, "n_event"] == 1  # baris 90: lompatan masuk [t, t+20]


def test_event_study_excludes_event_after_corporate_action():
    n = 200
    closes = [100.0] * 100 + [30.0] * 100
    df = _study_frame(n, [110, 150], np.zeros(n), closes=closes)
    res = event_study(df, horizons=(1,), exclusion=20)
    assert res.loc[1, "n_event"] == 1  # baris 110: lompatan masuk [t-20, t]


def test_event_study_zero_events_returns_nan():
    res = event_study(_study_frame(50, [], np.zeros(50)), horizons=(1,))
    row = res.loc[1]
    assert row["n_event"] == 0
    assert np.isnan(row["mean_event"]) and np.isnan(row["p_value"])


def test_run_study_end_to_end():
    rng = np.random.default_rng(1)
    n = 120
    vols = rng.integers(800, 1200, n)
    vols[[40, 80]] = 10_000
    closes = list(100 + np.cumsum(rng.normal(0, 1, n)))
    df = _ohlc("A", closes, closes)
    df["volume"] = vols
    results, events = run_study(df, Rule(price_filter=False), horizons=(1, 5))
    assert list(results.index) == [1, 5]
    assert results.loc[1, "n_event"] == 2
    assert set(events["date"]) == set(df["date"].iloc[[40, 80]])
    assert {"fwd_1", "fwd_5", "label", "multiple"} <= set(events.columns)
