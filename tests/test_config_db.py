from pathlib import Path

import pytest
import sqlalchemy as sa

from spikeidx.config import ConfigError, load_settings
from spikeidx.db import sqlalchemy_url
from spikeidx.migrate import apply_migrations


def test_sqlalchemy_url_rewrites_postgres_scheme():
    dsn = "postgres://u:p@h:5432/db?sslmode=require"
    assert sqlalchemy_url(dsn) == "postgresql+psycopg://u:p@h:5432/db?sslmode=require"


def test_sqlalchemy_url_rewrites_postgresql_scheme():
    assert sqlalchemy_url("postgresql://h/db") == "postgresql+psycopg://h/db"


def test_sqlalchemy_url_keeps_explicit_driver():
    assert sqlalchemy_url("postgresql+psycopg://h/db") == "postgresql+psycopg://h/db"


def test_load_settings_requires_database_url(monkeypatch, tmp_path):
    monkeypatch.delenv("DATABASE_URL", raising=False)
    empty = tmp_path / ".env"
    empty.write_text("")
    with pytest.raises(ConfigError, match="DATABASE_URL"):
        load_settings(env_file=empty)


def test_load_settings_reads_env_file_but_os_env_wins(monkeypatch, tmp_path):
    env = tmp_path / ".env"
    env.write_text("DATABASE_URL=postgres://file/db\nTELEGRAM_CHAT_ID=42\n")
    monkeypatch.delenv("TELEGRAM_CHAT_ID", raising=False)
    monkeypatch.delenv("TELEGRAM_BOT_TOKEN", raising=False)
    monkeypatch.setenv("DATABASE_URL", "postgres://os/db")
    s = load_settings(env_file=env)
    assert s.database_url == "postgres://os/db"
    assert s.telegram_chat_id == "42"
    assert s.telegram_bot_token == ""


def _write(directory: Path, name: str, sql: str) -> None:
    (directory / name).write_text(sql)


def test_apply_migrations_runs_in_name_order_once(tmp_path):
    _write(tmp_path, "002_b.sql", "INSERT INTO t (v) VALUES ('b')")
    _write(tmp_path, "001_a.sql", "CREATE TABLE t (v TEXT)")
    engine = sa.create_engine("sqlite://")

    assert apply_migrations(engine, tmp_path) == ["001_a.sql", "002_b.sql"]
    assert apply_migrations(engine, tmp_path) == []

    with engine.connect() as conn:
        assert conn.exec_driver_sql("SELECT v FROM t").scalars().all() == ["b"]


def test_apply_migrations_skips_names_already_recorded(tmp_path):
    _write(tmp_path, "001_a.sql", "CREATE TABLE t (v TEXT)")
    engine = sa.create_engine("sqlite://")
    with engine.begin() as conn:
        conn.exec_driver_sql("CREATE TABLE schema_migrations (name TEXT PRIMARY KEY)")
        conn.exec_driver_sql("INSERT INTO schema_migrations (name) VALUES ('001_a.sql')")

    assert apply_migrations(engine, tmp_path) == []


def test_apply_migrations_failure_records_nothing(tmp_path):
    _write(tmp_path, "001_bad.sql", "THIS IS NOT SQL")
    engine = sa.create_engine("sqlite://")
    with pytest.raises(sa.exc.SQLAlchemyError):
        apply_migrations(engine, tmp_path)
    with engine.connect() as conn:
        assert conn.exec_driver_sql("SELECT count(*) FROM schema_migrations").scalar() == 0
