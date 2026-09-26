from pathlib import Path

import pytest
from fastapi.testclient import TestClient

from app.api import app
from app.schemas import StructuredEvidence

FIXTURES_DIR = Path(__file__).parent.parent / "fixtures"


def load_evidence(name: str) -> StructuredEvidence:
    path = FIXTURES_DIR / f"{name}.json"
    return StructuredEvidence.model_validate_json(path.read_text())


@pytest.fixture
def client() -> TestClient:
    return TestClient(app)
