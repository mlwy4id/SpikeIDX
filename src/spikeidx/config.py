"""Settings dari environment; `.env` di root repo dimuat bila ada (env OS tetap menang)."""

import os
from dataclasses import dataclass
from pathlib import Path

from dotenv import load_dotenv

REPO_ROOT = Path(__file__).resolve().parents[2]
MIGRATIONS_DIR = REPO_ROOT / "internal" / "infra" / "postgres" / "migrations"


class ConfigError(RuntimeError):
    pass


@dataclass(frozen=True)
class Settings:
    database_url: str
    telegram_bot_token: str = ""
    telegram_chat_id: str = ""

    @property
    def telegram_enabled(self) -> bool:
        return bool(self.telegram_bot_token and self.telegram_chat_id)


def load_settings(env_file: Path | None = None) -> Settings:
    load_dotenv(env_file or REPO_ROOT / ".env", override=False)
    dsn = os.environ.get("DATABASE_URL", "").strip()
    if not dsn:
        raise ConfigError("DATABASE_URL belum diisi (env atau .env)")
    return Settings(
        database_url=dsn,
        telegram_bot_token=os.environ.get("TELEGRAM_BOT_TOKEN", "").strip(),
        telegram_chat_id=os.environ.get("TELEGRAM_CHAT_ID", "").strip(),
    )
