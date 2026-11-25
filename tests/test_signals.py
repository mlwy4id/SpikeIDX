import json
from pathlib import Path

import numpy as np
import pandas as pd
import pytest

from spikeidx.signals import FILTERED_LABEL, Rule, add_features, classify, flow_label

TESTDATA = Path(__file__).resolve().parents[1] / "testdata"


def _golden() -> pd.DataFrame:
    df = pd.read_csv(TESTDATA / "golden_bars.csv", parse_dates=["date"])
    df.insert(0, "code", "GOLD")
    return df


def _bars(code: str, volumes, closes=None) -> pd.DataFrame:
    n = len(volumes)
    closes = closes if closes is not None else [100.0] * n
    return pd.DataFrame(
        {
            "code": code,
            "date": pd.bdate_range("2026-01-01", periods=n),
            "open": closes,
            "high": [c + 2 for c in closes],
            "low": [c - 2 for c in closes],
            "close": closes,
            "volume": volumes,
        }
    )


def test_golden_fixture_values():
    expected = json.loads((TESTDATA / "golden_expected.json").read_text())
    last = add_features(_golden()).iloc[-1]
    for col, value in expected.items():
        assert last[col] == pytest.approx(value, rel=1e-12), col


def test_needs_window_plus_one_bars():
    feat = add_features(_bars("A", [1000] * 20 + [3000]))
    assert feat["avg20"].iloc[:20].isna().all()
    assert not np.isnan(feat["avg20"].iloc[20])
    assert not classify(feat.iloc[:20], Rule())["is_spike"].any()


def test_features_have_no_lookahead():
    base = _bars("A", [900, 1100] * 15)
    shocked = base.copy()
    shocked.loc[shocked.index[-1], "volume"] = 10**9
    a = add_features(base).iloc[:-1]
    b = add_features(shocked).iloc[:-1]
    pd.testing.assert_frame_equal(a, b)


def test_features_do_not_leak_across_codes():
    df = pd.concat([_bars("AAAA", [900, 1100] * 15), _bars("BBBB", [900, 1100] * 15)])
    feat = add_features(df)
    b = feat[feat["code"] == "BBBB"]
    assert b["avg20"].iloc[:20].isna().all()
    assert np.isnan(b["pct_change"].iloc[0])


def test_flat_baseline_gives_zero_z_and_no_spike():
    feat = classify(add_features(_bars("A", [1000] * 20 + [5000])), Rule())
    last = feat.iloc[-1]
    assert last["multiple"] == 5.0
    assert last["zscore"] == 0.0
    assert not last["is_spike"]


def test_zero_baseline_gives_zero_multiple():
    last = add_features(_bars("A", [0] * 20 + [5000])).iloc[-1]
    assert last["multiple"] == 0.0


def _row(multiple, zscore, pct, cmf=0.0) -> pd.DataFrame:
    return pd.DataFrame(
        {"multiple": [multiple], "zscore": [zscore], "pct_change": [pct], "cmf20": [cmf]}
    )


@pytest.mark.parametrize(
    "multiple,zscore,pct,spike,filtered",
    [
        (2.5, 3.0, 3.0, True, False),
        (2.0, 3.0, 3.0, False, False),  # multiple strict
        (2.5, 2.0, 3.0, False, False),  # z strict
        (2.5, 3.0, 2.0, True, False),  # |pct| tepat 2.0 tidak terfilter
        (2.5, 3.0, -1.9, True, True),
        (np.nan, np.nan, np.nan, False, False),
    ],
)
def test_classify_rule_boundaries(multiple, zscore, pct, spike, filtered):
    out = classify(_row(multiple, zscore, pct), Rule()).iloc[0]
    assert out["is_spike"] == spike
    assert out["is_filtered"] == filtered
    assert out["is_actionable"] == (spike and not filtered)


def test_classify_price_filter_can_be_disabled():
    out = classify(_row(2.5, 3.0, 0.5), Rule(price_filter=False)).iloc[0]
    assert out["is_actionable"]


@pytest.mark.parametrize(
    "cmf,label",
    [
        (0.25, "akumulasi"),
        (-0.25, "distribusi"),
        (0.10, "netral"),
        (-0.10, "netral"),
        (0, "netral"),
    ],
)
def test_flow_label_strict_threshold(cmf, label):
    assert flow_label(pd.Series([cmf]), 0.10).iloc[0] == label


def test_classify_label_filtered_has_priority():
    out = classify(_row(2.5, 3.0, 0.5, cmf=0.5), Rule()).iloc[0]
    assert out["label"] == FILTERED_LABEL


def test_rule_roundtrip_dict():
    rule = Rule(multiple_min=1.5, cmf_min=0.2)
    assert Rule.from_dict(rule.to_dict()) == rule
