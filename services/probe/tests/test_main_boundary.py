from copy import deepcopy

from fastapi.testclient import TestClient

from app.api import app, get_provider
from app.grounding import ground_evidence
from app.providers.fake import FakeAIProvider
from app.schemas import MainDiagnosis, StructuredEvidence


def _main_evidence() -> dict:
    return {
        "probe": "P3",
        "role": "ECHO",
        "expected": {
            "signal": "pulse",
            "required": True,
            "max_dropouts": 0,
            "safe_max_pin_voltage": 3.3,
            "nominal_frequency_hz": 28.4,
        },
        "observed": {
            "pulse_detected": True,
            "dropouts_per_window": 12,
            "missing_expected_activity": False,
            "stable": False,
            "rail_voltage_stable": True,
            "other_signals_active": True,
            "movement_correlation": False,
        },
        "baseline": {
            "status": "USER_CONFIRMED_HEALTHY",
            "trusted": True,
            "frequency_hz": 28.4,
        },
        "measurements": [
            {
                "probe": "P3",
                "name": "pulse_count",
                "value": 1450,
                "provenance": "MEASURED",
            }
        ],
        "derived_facts": [
            {
                "probe": "P3",
                "name": "dropout_events",
                "value": 12,
                "provenance": "DERIVED",
            }
        ],
        "specification_results": [],
        "baseline_comparison": [
            {
                "probe": "P3",
                "name": "baseline_deviation",
                "value": 42.2,
                "unit": "%",
                "provenance": "BASELINE",
            }
        ],
        "rule_results": [
            {
                "id": "unexpected-dropout",
                "probe": "P3",
                "status": "fail",
                "severity": 3,
                "message": "12 unexpected ECHO dropouts detected",
                "provenance": "DERIVED",
            },
            {
                "id": "movement-correlation",
                "probe": "P3",
                "status": "warn",
                "severity": 1,
                "message": "Movement correlation has not been tested for ECHO",
                "provenance": "DERIVED",
            },
        ],
        "unresolved_questions": [
            "Does the dropout rate increase during controlled movement?"
        ],
    }


def _deterministic_diagnosis() -> dict:
    return {
        "headline": "Intermittent ECHO activity",
        "summary": "The failure is isolated to the ECHO path.",
        "possible_causes": ["Intermittent connection"],
        "confidence": 0.68,
        "next_test": "Gently wiggle the ECHO connection.",
    }


def test_main_structured_evidence_to_grounded_diagnosis_boundary() -> None:
    app.dependency_overrides[get_provider] = FakeAIProvider
    original = _main_evidence()
    try:
        response = TestClient(app).post(
            "/probe/main-diagnosis",
            json={
                "evidence": original,
                "deterministic_diagnosis": _deterministic_diagnosis(),
            },
        )
    finally:
        app.dependency_overrides.clear()

    assert response.status_code == 200
    diagnosis = MainDiagnosis.model_validate(response.json())
    parsed = StructuredEvidence.model_validate(original)
    grounded = ground_evidence(parsed)
    valid_refs = {item.ref_id for item in grounded}
    detailed = FakeAIProvider().interpret(parsed, grounded)

    assert valid_refs
    assert detailed.hypotheses
    assert set(detailed.hypotheses[0].grounded_in) <= valid_refs
    assert diagnosis.confidence <= _deterministic_diagnosis()["confidence"]
    assert "movement" in diagnosis.next_test.lower()
    assert diagnosis.possible_causes == ["Unexpected signal dropout"]
    assert detailed.hypotheses[1].label == "Movement correlation not yet tested"
    assert original == _main_evidence()
    assert all(
        fact["provenance"] != "AI_INTERPRETATION"
        for category in (
            "measurements",
            "derived_facts",
            "specification_results",
            "baseline_comparison",
        )
        for fact in original[category]
    )


def test_main_boundary_unknown_fails_closed_to_deterministic_diagnosis() -> None:
    app.dependency_overrides[get_provider] = FakeAIProvider
    evidence = deepcopy(_main_evidence())
    evidence["measurements"] = []
    evidence["derived_facts"] = []
    evidence["baseline_comparison"] = []
    evidence["rule_results"] = []
    deterministic = _deterministic_diagnosis()
    try:
        response = TestClient(app).post(
            "/probe/main-diagnosis",
            json={"evidence": evidence, "deterministic_diagnosis": deterministic},
        )
    finally:
        app.dependency_overrides.clear()

    assert response.status_code == 200
    assert response.json() == deterministic
