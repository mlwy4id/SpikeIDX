"""Runner migrasi yang kompatibel dengan `Migrate` di Go.

Keduanya memakai tabel `schema_migrations(name)` dan menerapkan file `*.sql` urut
nama, satu transaksi per file. Migrasi yang sudah tercatat tidak dijalankan lagi,
jadi file lama tidak boleh diedit.
"""

from pathlib import Path

import sqlalchemy as sa


def apply_migrations(engine: sa.Engine, directory: Path) -> list[str]:
    with engine.begin() as conn:
        conn.exec_driver_sql("CREATE TABLE IF NOT EXISTS schema_migrations (name TEXT PRIMARY KEY)")
        done = set(conn.exec_driver_sql("SELECT name FROM schema_migrations").scalars())

    applied = []
    for path in sorted(directory.glob("*.sql")):
        if path.name in done:
            continue
        with engine.begin() as conn:
            conn.exec_driver_sql(path.read_text())
            conn.execute(
                sa.text("INSERT INTO schema_migrations (name) VALUES (:name)"),
                {"name": path.name},
            )
        applied.append(path.name)
    return applied
