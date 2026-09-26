import json
from functools import lru_cache
from pathlib import Path

from app.schemas import CatalogEntry

# packages/component-catalog is a sibling top-level directory to services/;
# this is the FIRST real runtime reader of that directory (confirmed via
# research: no Go/TS code loads it today, only a hand-copied citation
# string). Read-only - never written to.
_DEFAULT_CATALOG_DIR = Path(__file__).resolve().parents[3] / "packages" / "component-catalog"


def load_catalog(catalog_dir: Path | None = None) -> dict[str, CatalogEntry]:
    directory = catalog_dir or _DEFAULT_CATALOG_DIR
    entries: dict[str, CatalogEntry] = {}
    if not directory.is_dir():
        return entries
    for path in sorted(directory.glob("*.json")):
        data = json.loads(path.read_text())
        entry = CatalogEntry.model_validate(data)
        entries[entry.id] = entry
    return entries


@lru_cache
def get_default_catalog() -> dict[str, CatalogEntry]:
    return load_catalog()
