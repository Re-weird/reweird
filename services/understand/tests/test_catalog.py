import json
from pathlib import Path

from app.catalog import get_default_catalog, load_catalog


def test_loads_real_component_catalog() -> None:
    """packages/component-catalog is authoritative and owned by the Go API
    (packages/component-catalog/catalog.go); its current schema names pins
    via `pin_roles` and interfaces via `known_signal_types`/`interface_type`,
    not this service's original `pins`/`interfaces` field names. CatalogEntry's
    _map_current_catalog_schema validator maps them - this test asserts the
    real, current mapped values, not the pre-integration ones."""
    catalog = get_default_catalog()
    assert "hc-sr04" in catalog
    entry = catalog["hc-sr04"]
    assert entry.name == "HC-SR04 Ultrasonic Distance Sensor"
    assert entry.pins == ["VCC", "TRIG", "ECHO", "GND"]
    assert entry.interfaces == ["voltage rail", "digital pulse", "pulse width"]


def test_empty_directory_returns_empty_catalog(tmp_path: Path) -> None:
    assert load_catalog(tmp_path) == {}


def test_nonexistent_directory_returns_empty_catalog(tmp_path: Path) -> None:
    assert load_catalog(tmp_path / "does-not-exist") == {}


def test_custom_catalog_directory_loads_entries(tmp_path: Path) -> None:
    (tmp_path / "widget.json").write_text(
        json.dumps({"id": "widget", "name": "Test Widget", "pins": ["A", "B"]})
    )
    catalog = load_catalog(tmp_path)
    assert catalog["widget"].name == "Test Widget"
    assert catalog["widget"].pins == ["A", "B"]
    assert catalog["widget"].interfaces == []  # optional fields default cleanly


def test_current_catalog_schema_fields_are_mapped_onto_legacy_names(tmp_path: Path) -> None:
    (tmp_path / "sensor.json").write_text(
        json.dumps(
            {
                "id": "sensor",
                "name": "Test Sensor",
                "pin_roles": [{"name": "VCC"}, {"name": "OUT"}],
                "operating_voltage": {"typical": 3.3, "unit": "V"},
                "known_signal_types": ["digital pulse"],
                "safe_measurement_notes": ["Do not exceed 3.3V."],
            }
        )
    )
    entry = load_catalog(tmp_path)["sensor"]
    assert entry.pins == ["VCC", "OUT"]
    assert entry.supply_voltage == {"typical": 3.3, "unit": "V"}
    assert entry.interfaces == ["digital pulse"]
    assert entry.notes == ["Do not exceed 3.3V."]


def test_legacy_catalog_field_names_still_take_priority_when_present(tmp_path: Path) -> None:
    """If a catalog entry ever carries BOTH old and new field names, the
    old, already-explicit name wins - the mapping only fills in what is
    otherwise absent, it never overrides an explicit value."""
    (tmp_path / "both.json").write_text(
        json.dumps(
            {
                "id": "both",
                "name": "Both Schemas",
                "pins": ["EXPLICIT"],
                "pin_roles": [{"name": "FROM_NEW_SCHEMA"}],
            }
        )
    )
    entry = load_catalog(tmp_path)["both"]
    assert entry.pins == ["EXPLICIT"]


def test_interface_type_singleton_used_when_no_known_signal_types(tmp_path: Path) -> None:
    (tmp_path / "single.json").write_text(
        json.dumps({"id": "single", "name": "Single Interface", "interface_type": "trigger/echo pulse"})
    )
    entry = load_catalog(tmp_path)["single"]
    assert entry.interfaces == ["trigger/echo pulse"]
