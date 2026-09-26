import re

from app.schemas import CodeFact

_SLUG_RE = re.compile(r"[^a-z0-9_]+")


def _slug(text: str) -> str:
    return _SLUG_RE.sub("-", text.lower()).strip("-") or "na"


def make_ref_id(file: str, line: int, kind: str, symbol: str | None, seen: dict[str, int]) -> str:
    """Deterministic, content-derived ref_id - never random/UUID.

    Format: fact:{file-slug}:{line}:{kind-slug}[:{symbol-slug}], with a
    trailing disambiguation counter appended only on the rare collision
    (two facts sharing every other component).
    """
    base = f"fact:{_slug(file)}:{line}:{_slug(kind)}"
    if symbol:
        base = f"{base}:{_slug(symbol)}"
    seen[base] = seen.get(base, 0) + 1
    return base if seen[base] == 1 else f"{base}:{seen[base]}"


def index_by_ref_id(facts: list[CodeFact]) -> dict[str, CodeFact]:
    return {fact.ref_id: fact for fact in facts}
