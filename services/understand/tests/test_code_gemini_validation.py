from app.catalog import get_default_catalog
from app.code_analysis.extractor import analyze_files
from app.code_analysis.gemini_schema import CodeInterpretationOut
from app.code_analysis.gemini_validation import validate_and_rank
from tests.conftest import load_code_fixture, two_entry_test_catalog


def _facts_and_catalog():
    files = {"healthy_ultrasonic.cpp": load_code_fixture("healthy_ultrasonic.cpp")}
    result = analyze_files(files)
    return result.facts, get_default_catalog()


def _rule_ref(facts, symbol: str) -> str:
    return next(f.ref_id for f in facts if f.kind == "pin_constant" and f.symbol == symbol)


def test_valid_response_produces_components_and_roles() -> None:
    facts, catalog = _facts_and_catalog()
    trig_ref = _rule_ref(facts, "TRIG_PIN")
    raw = CodeInterpretationOut(
        outcome="INTERPRETED",
        controller="ESP32",
        expected_behavior_summary="Measures distance via an ultrasonic trigger/echo pair.",
        component_candidates=[
            {
                "catalog_id": "hc-sr04",
                "confidence": 0.9,
                "grounded_in": [trig_ref],
                "rationale": "TRIG/ECHO pin pattern.",
            }
        ],
        role_candidates=[
            {
                "target_pin": "25",
                "likely_role": "TRIG",
                "mode": "pulse",
                "is_power_rail": False,
                "confidence": 0.8,
                "grounded_in": [trig_ref],
            }
        ],
    )
    result = validate_and_rank(raw, facts, catalog)
    assert result.outcome == "INTERPRETED"
    assert result.components[0].catalog_id == "hc-sr04"
    assert result.components[0].name == "HC-SR04 Ultrasonic Distance Sensor"
    assert result.components[0].source == "component-catalog/hc-sr04"
    assert result.roles[0].role == "TRIG"


def test_hallucinated_catalog_id_invalidates_entire_response() -> None:
    facts, catalog = _facts_and_catalog()
    trig_ref = _rule_ref(facts, "TRIG_PIN")
    raw = CodeInterpretationOut(
        outcome="INTERPRETED",
        component_candidates=[
            {
                "catalog_id": "totally-made-up-sensor",
                "confidence": 0.9,
                "grounded_in": [trig_ref],
                "rationale": "x",
            }
        ],
    )
    result = validate_and_rank(raw, facts, catalog)
    assert result.outcome == "UNKNOWN"
    assert result.unknown_reason == "INVALID_PROVIDER_OUTPUT"
    assert result.components == []


def test_hallucinated_ref_id_invalidates_entire_response() -> None:
    facts, catalog = _facts_and_catalog()
    raw = CodeInterpretationOut(
        outcome="INTERPRETED",
        component_candidates=[
            {
                "catalog_id": "hc-sr04",
                "confidence": 0.9,
                "grounded_in": ["fact:nonexistent:1:pin_constant:x"],
                "rationale": "x",
            }
        ],
    )
    result = validate_and_rank(raw, facts, catalog)
    assert result.outcome == "UNKNOWN"
    assert result.unknown_reason == "INVALID_PROVIDER_OUTPUT"


def test_hallucinated_role_ref_invalidates_entire_response() -> None:
    facts, catalog = _facts_and_catalog()
    raw = CodeInterpretationOut(
        outcome="INTERPRETED",
        role_candidates=[
            {
                "target_pin": "25",
                "likely_role": "TRIG",
                "mode": "pulse",
                "is_power_rail": False,
                "confidence": 0.8,
                "grounded_in": ["fact:nonexistent:1:pin_constant:x"],
            }
        ],
    )
    result = validate_and_rank(raw, facts, catalog)
    assert result.outcome == "UNKNOWN"
    assert result.unknown_reason == "INVALID_PROVIDER_OUTPUT"


def test_model_declared_unknown_passes_through() -> None:
    facts, catalog = _facts_and_catalog()
    raw = CodeInterpretationOut(
        outcome="UNKNOWN",
        unknown_reason="PROVIDER_UNCERTAIN",
        reasoning_notes=["Not enough context to be confident."],
    )
    result = validate_and_rank(raw, facts, catalog)
    assert result.outcome == "UNKNOWN"
    assert result.unknown_reason == "PROVIDER_UNCERTAIN"
    assert result.components == []
    assert result.roles == []


def test_ambiguous_candidates_both_surface_when_catalog_has_two_similar_entries() -> None:
    files = {"ambiguous_component.cpp": load_code_fixture("ambiguous_component.cpp")}
    facts = analyze_files(files).facts
    catalog = two_entry_test_catalog()
    trig_ref = _rule_ref(facts, "TRIG_PIN")

    raw = CodeInterpretationOut(
        outcome="INTERPRETED",
        component_candidates=[
            {
                "catalog_id": "hc-sr04",
                "confidence": 0.55,
                "grounded_in": [trig_ref],
                "rationale": "Matches trigger/echo pattern.",
            },
            {
                "catalog_id": "test-only-dual-pulse-sensor",
                "confidence": 0.5,
                "grounded_in": [trig_ref],
                "rationale": "Also matches trigger/echo pattern.",
            },
        ],
    )
    result = validate_and_rank(raw, facts, catalog)
    assert result.outcome == "INTERPRETED"
    # Both plausible candidates survive - the assembler never silently
    # commits to a single "winner" when the code facts are genuinely
    # ambiguous between two catalog entries.
    assert {c.catalog_id for c in result.components} == {"hc-sr04", "test-only-dual-pulse-sensor"}
