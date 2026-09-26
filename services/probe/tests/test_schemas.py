import pytest
from pydantic import ValidationError

from app.schemas import EvidenceFact, RuleResult
from tests.conftest import FIXTURES_DIR, load_evidence

FIXTURE_NAMES = [
    "healthy",
    "missing_power",
    "missing_trigger",
    "missing_echo",
    "intermittent_echo",
    "conflicting_evidence",
    "insufficient_evidence",
]


@pytest.mark.parametrize("name", FIXTURE_NAMES)
def test_fixture_round_trips_without_data_loss(name: str) -> None:
    evidence = load_evidence(name)
    dumped = evidence.model_dump(mode="json")
    reloaded = evidence.model_validate(dumped)
    assert reloaded == evidence


def test_all_fixture_files_are_present() -> None:
    for name in FIXTURE_NAMES:
        assert (FIXTURES_DIR / f"{name}.json").exists()


@pytest.mark.parametrize(
    "provenance", ["MEASURED", "DERIVED", "SPECIFICATION", "BASELINE", "SOFTWARE", "AI_INTERPRETATION"]
)
def test_evidence_fact_accepts_each_provenance_literal(provenance: str) -> None:
    fact = EvidenceFact(name="x", value=1, provenance=provenance)
    assert fact.provenance == provenance


def test_evidence_fact_rejects_invalid_provenance() -> None:
    with pytest.raises(ValidationError):
        EvidenceFact(name="x", value=1, provenance="NOT_A_REAL_PROVENANCE")


@pytest.mark.parametrize("status", ["pass", "warn", "fail"])
def test_rule_result_accepts_each_status_literal(status: str) -> None:
    rule = RuleResult(id="r1", status=status, message="m")
    assert rule.status == status


def test_rule_result_rejects_invalid_status() -> None:
    with pytest.raises(ValidationError):
        RuleResult(id="r1", status="unknown-status", message="m")
