import sqlalchemy as sa

from spikeidx.config import Settings


def sqlalchemy_url(dsn: str) -> str:
    """Ubah DSN libpq (`postgres://`) ke URL SQLAlchemy dengan driver psycopg 3."""
    for scheme in ("postgres://", "postgresql://"):
        if dsn.startswith(scheme):
            return "postgresql+psycopg://" + dsn[len(scheme) :]
    return dsn


def make_engine(settings: Settings) -> sa.Engine:
    return sa.create_engine(sqlalchemy_url(settings.database_url), pool_pre_ping=True)
