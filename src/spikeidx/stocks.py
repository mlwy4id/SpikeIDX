import re
from dataclasses import dataclass

_CODE = re.compile(r"^[A-Z]{3,4}$")


@dataclass(frozen=True)
class Stock:
    code: str
    yahoo_symbol: str
    name: str
    sector: str = ""


def parse_code(raw: str) -> str:
    """Normalisasi kode saham IDX: trim, uppercase, buang `.JK`, lalu validasi `^[A-Z]{3,4}$`."""
    code = raw.strip().upper().removesuffix(".JK")
    if not _CODE.match(code):
        raise ValueError(f"kode saham tidak valid: {raw!r}")
    return code


def yahoo_symbol(code: str) -> str:
    return f"{code}.JK"
