import json
from pathlib import Path

from app.catalog import get_default_catalog, load_catalog


def test_loads_real_component_catalog() -> None:
    catalog = get_default_catalog()
    assert "hc-sr04" in catalog
    entry = catalog["hc-sr04"]
    assert entry.name == "HC-SR04 Ultrasonic Distance Sensor"
    assert entry.pins == ["VCC", "TRIG", "ECHO", "GND"]
    assert entry.interfaces == ["trigger pulse", "echo pulse"]


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
