import os

import pytest


@pytest.fixture(autouse=True)
def _isolated_environ(monkeypatch):
    # load_dotenv menulis ke os.environ; salinan per test mencegah bocor antar test.
    monkeypatch.setattr(os, "environ", os.environ.copy())
