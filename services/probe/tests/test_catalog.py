import json
from pathlib import Path

from app.catalog import get_default_catalog, load_catalog

REAL_CATALOG_DIR = Path(__file__).parent.parent.parent.parent / "packages" / "component-catalog"


def test_real_catalog_loads_hc_sr04_with_specifications() -> None:
    catalog = load_catalog(REAL_CATALOG_DIR)
    assert "hc-sr04" in catalog
    entry = catalog["hc-sr04"]
    assert entry.name == "HC-SR04 Ultrasonic Distance Sensor"
    assert set(entry.specifications) == {"VCC", "TRIG", "ECHO"}
    assert entry.specifications["VCC"].voltage is not None
    assert entry.specifications["VCC"].voltage.min == 4.5
    assert entry.specifications["VCC"].voltage.max == 5.5
    assert entry.specifications["ECHO"].pulse_width is not None
    assert entry.specifications["ECHO"].pulse_width.min == 116
    assert entry.specifications["ECHO"].pulse_width.max == 23200
    # Every numeric specification must cite where it came from.
    for spec in entry.specifications.values():
        assert spec.source


def test_get_default_catalog_is_cached_and_matches_load_catalog() -> None:
    cached = get_default_catalog()
    direct = load_catalog(REAL_CATALOG_DIR)
    assert set(cached) == set(direct)
    assert get_default_catalog() is cached  # lru_cache: same object every call


def test_empty_directory_yields_empty_catalog(tmp_path: Path) -> None:
    assert load_catalog(tmp_path) == {}


def test_malformed_entry_is_skipped_without_corrupting_valid_entries(tmp_path: Path) -> None:
    (tmp_path / "good.json").write_text(
        json.dumps({"id": "good-widget", "name": "Good Widget", "specifications": {}})
    )
    (tmp_path / "bad_json.json").write_text("{not valid json")
    (tmp_path / "bad_schema.json").write_text(
        json.dumps(
            {
                "id": "bad-widget",
                "name": "Bad Widget",
                "specifications": {
                    "TRIG": {
                        "pin": "TRIG",
                        "signal_type": "digital_pulse",
                        "source": "test",
                        "unexpected_extra_field": "forbidden by extra=forbid",
                    }
                },
            }
        )
    )
    (tmp_path / "bad_range.json").write_text(
        json.dumps(
            {
                "id": "bad-range-widget",
                "name": "Bad Range Widget",
                "specifications": {
                    "VCC": {
                        "pin": "VCC",
                        "signal_type": "power_rail",
                        "source": "test",
                        "voltage": {"min": 10, "max": 1, "unit": "V"},
                    }
                },
            }
        )
    )

    catalog = load_catalog(tmp_path)

    assert set(catalog) == {"good-widget"}
