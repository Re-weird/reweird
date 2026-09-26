import pytest
from pydantic import ValidationError

from app.vision.gemini_schema import VisionComponentCandidateOut, VisionInterpretationOut


def _candidate(**overrides) -> dict:
    fields = dict(catalog_id="hc-sr04", confidence=0.7, rationale="Looks like an HC-SR04 module.")
    fields.update(overrides)
    return fields


def test_valid_interpreted_round_trips() -> None:
    out = VisionInterpretationOut(outcome="INTERPRETED", component_candidates=[_candidate()])
    reloaded = VisionInterpretationOut.model_validate_json(out.model_dump_json())
    assert reloaded == out


def test_valid_unknown_round_trips() -> None:
    out = VisionInterpretationOut(outcome="UNKNOWN", unknown_reason="PROVIDER_UNCERTAIN")
    reloaded = VisionInterpretationOut.model_validate_json(out.model_dump_json())
    assert reloaded == out


def test_extra_field_rejected() -> None:
    with pytest.raises(ValidationError):
        VisionComponentCandidateOut(**_candidate(), extra="nope")


def test_confidence_out_of_range_rejected() -> None:
    with pytest.raises(ValidationError):
        VisionComponentCandidateOut(**_candidate(confidence=1.2))


def test_unknown_outcome_with_candidates_rejected() -> None:
    with pytest.raises(ValidationError):
        VisionInterpretationOut(
            outcome="UNKNOWN",
            unknown_reason="PROVIDER_UNCERTAIN",
            component_candidates=[_candidate()],
        )


def test_schema_has_no_numeric_measurement_field() -> None:
    """Structural guarantee: vision output can never claim an exact
    electrical measurement, because no such field exists to populate."""
    field_names = set(VisionComponentCandidateOut.model_fields.keys())
    measurement_shaped_names = {
        "voltage",
        "frequency",
        "resistance",
        "current",
        "distance",
        "value",
        "measurement",
    }
    assert field_names.isdisjoint(measurement_shaped_names)
    # confidence is the only float field, and it is a 0..1 score, not a
    # physical unit.
    float_fields = {
        name
        for name, info in VisionComponentCandidateOut.model_fields.items()
        if info.annotation is float
    }
    assert float_fields == {"confidence"}
