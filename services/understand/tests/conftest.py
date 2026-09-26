from pathlib import Path

import pytest
from fastapi.testclient import TestClient

from app.api import app
from app.schemas import CatalogEntry

FIXTURES_DIR = Path(__file__).parent.parent / "fixtures"
CODE_FIXTURES_DIR = FIXTURES_DIR / "code"


def load_code_fixture(name: str) -> bytes:
    return (CODE_FIXTURES_DIR / name).read_bytes()


def two_entry_test_catalog() -> dict[str, CatalogEntry]:
    """A test-only, in-memory catalog fixture for ambiguous-match tests.

    Never written to packages/component-catalog/ - this exists purely so
    tests can exercise "code facts plausibly match more than one catalog
    entry" without adding a fictional component to the real, production
    catalog.
    """
    return {
        "hc-sr04": CatalogEntry(
            id="hc-sr04",
            name="HC-SR04 Ultrasonic Distance Sensor",
            pins=["VCC", "TRIG", "ECHO", "GND"],
            supply_voltage={"typical": 5.0, "unit": "V"},
            interfaces=["trigger pulse", "echo pulse"],
            notes=["Catalog values are project context and do not replace measured evidence."],
        ),
        "test-only-dual-pulse-sensor": CatalogEntry(
            id="test-only-dual-pulse-sensor",
            name="TEST-ONLY Dual Pulse Sensor (fixture, not a real catalog entry)",
            pins=["VCC", "TRIG", "ECHO", "GND"],
            supply_voltage={"typical": 5.0, "unit": "V"},
            interfaces=["trigger pulse", "echo pulse"],
            notes=["Fixture-only entry used solely to test ambiguous-match handling."],
        ),
    }


@pytest.fixture
def client() -> TestClient:
    return TestClient(app)
