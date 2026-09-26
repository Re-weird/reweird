import json
import logging
from functools import lru_cache
from pathlib import Path
from typing import Literal

from pydantic import BaseModel, Field, ValidationError, model_validator

logger = logging.getLogger("app.catalog")

# packages/component-catalog is a sibling top-level directory to services/.
# Read-only; never written to. A deliberate duplicate of
# services/understand/app/catalog.py's loader pattern (own Pydantic model,
# own failure handling) rather than a shared package - the two services are
# independently deployable, consistent with this repo's existing precedent
# of small duplication over cross-service coupling.
_DEFAULT_CATALOG_DIR = Path(__file__).resolve().parents[3] / "packages" / "component-catalog"


class RangeSpec(BaseModel):
    model_config = {"extra": "forbid"}

    min: float
    max: float
    nominal: float | None = None
    unit: str

    @model_validator(mode="after")
    def _min_le_max(self) -> "RangeSpec":
        if self.min > self.max:
            raise ValueError("min must be <= max")
        return self


class SpecificationEntry(BaseModel):
    """A deterministic, datasheet-sourced expected signal for one pin role.

    Every numeric field here must be traceable to `source`. Nothing here is
    inferred or diagnosed - it is reference data, evaluated against real
    evidence by app/specification.py.
    """

    model_config = {"extra": "forbid"}

    pin: str
    signal_type: str
    mode: Literal["analog", "digital", "pulse"] | None = None
    is_power_rail: bool = False
    required: bool = False
    voltage: RangeSpec | None = None
    pulse_width: RangeSpec | None = None
    source: str = Field(min_length=1)


class CatalogSpecEntry(BaseModel):
    """services/probe's own view of a catalog entry.

    Tolerates (ignores) the other fields services/understand's simpler
    CatalogEntry model reads (pins, supply_voltage, interfaces, notes) -
    this service only needs id/name/specifications.
    """

    model_config = {"extra": "ignore"}

    id: str
    name: str
    specifications: dict[str, SpecificationEntry] = Field(default_factory=dict)


def load_catalog(catalog_dir: Path | None = None) -> dict[str, CatalogSpecEntry]:
    directory = catalog_dir or _DEFAULT_CATALOG_DIR
    entries: dict[str, CatalogSpecEntry] = {}
    if not directory.is_dir():
        return entries
    for path in sorted(directory.glob("*.json")):
        try:
            data = json.loads(path.read_text())
            entry = CatalogSpecEntry.model_validate(data)
        except (json.JSONDecodeError, ValidationError) as exc:
            # Fail closed for THIS file only - one malformed entry must never
            # corrupt or block every other valid catalog entry.
            logger.warning(
                "Skipping malformed component catalog entry %s: %s",
                path.name,
                type(exc).__name__,
            )
            continue
        entries[entry.id] = entry
    return entries


@lru_cache
def get_default_catalog() -> dict[str, CatalogSpecEntry]:
    return load_catalog()
