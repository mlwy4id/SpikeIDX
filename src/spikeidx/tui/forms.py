"""Validasi input TUI, terpisah dari widget supaya bisa di-test langsung."""

from spikeidx.signals import Rule

MAX_HORIZON = 60
RULE_FIELDS = ("multiple_min", "z_min", "pct_min", "cmf_min")


def parse_horizons(text: str) -> tuple[int, ...]:
    try:
        values = {int(part) for part in text.split(",") if part.strip()}
    except ValueError:
        raise ValueError("horizon harus bilangan bulat dipisah koma, mis. 1,5,20") from None
    if not values or min(values) < 1 or max(values) > MAX_HORIZON:
        raise ValueError(f"horizon harus antara 1 dan {MAX_HORIZON} hari")
    return tuple(sorted(values))


def rule_from_inputs(values: dict[str, str], price_filter: bool) -> Rule:
    parsed = {}
    for name in RULE_FIELDS:
        try:
            parsed[name] = float(values[name])
        except ValueError:
            raise ValueError(f"{name} harus angka") from None
        if parsed[name] < 0:
            raise ValueError(f"{name} tidak boleh negatif")
    return Rule(**parsed, price_filter=price_filter)
