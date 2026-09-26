import pytest
from pydantic import ValidationError

from app.code_analysis.gemini_schema import (
    CodeInterpretationOut,
    ComponentCandidateOut,
    RoleCandidateOut,
)


def _component(**overrides) -> dict:
    fields = dict(
        catalog_id="hc-sr04",
        confidence=0.8,
        grounded_in=["fact:main-cpp:3:pin_constant:trig_pin"],
        rationale="TRIG/ECHO pin pattern matches an ultrasonic sensor.",
    )
    fields.update(overrides)
    return fields


def _role(**overrides) -> dict:
    fields = dict(
        target_pin="GPIO25",
        likely_role="TRIG",
        mode="pulse",
        is_power_rail=False,
        confidence=0.75,
        grounded_in=["fact:main-cpp:3:pin_constant:trig_pin"],
    )
    fields.update(overrides)
    return fields


def test_valid_interpreted_round_trips() -> None:
    out = CodeInterpretationOut(
        outcome="INTERPRETED",
        component_candidates=[_component()],
        role_candidates=[_role()],
    )
    reloaded = CodeInterpretationOut.model_validate_json(out.model_dump_json())
    assert reloaded == out


def test_valid_unknown_round_trips() -> None:
    out = CodeInterpretationOut(outcome="UNKNOWN", unknown_reason="PROVIDER_UNCERTAIN")
    reloaded = CodeInterpretationOut.model_validate_json(out.model_dump_json())
    assert reloaded == out


def test_extra_field_rejected() -> None:
    with pytest.raises(ValidationError):
        ComponentCandidateOut(**_component(), extra="nope")


def test_confidence_out_of_range_rejected() -> None:
    with pytest.raises(ValidationError):
        ComponentCandidateOut(**_component(confidence=1.5))


def test_empty_grounded_in_rejected() -> None:
    with pytest.raises(ValidationError):
        ComponentCandidateOut(**_component(grounded_in=[]))


def test_duplicate_grounded_in_rejected() -> None:
    with pytest.raises(ValidationError):
        ComponentCandidateOut(**_component(grounded_in=["ref1", "ref1"]))


def test_role_mode_must_be_known_literal() -> None:
    with pytest.raises(ValidationError):
        RoleCandidateOut(**_role(mode="quantum"))


def test_too_many_component_candidates_rejected() -> None:
    with pytest.raises(ValidationError):
        CodeInterpretationOut(
            outcome="INTERPRETED", component_candidates=[_component() for _ in range(11)]
        )


def test_unknown_outcome_with_candidates_rejected() -> None:
    with pytest.raises(ValidationError):
        CodeInterpretationOut(
            outcome="UNKNOWN",
            unknown_reason="PROVIDER_UNCERTAIN",
            component_candidates=[_component()],
        )


def test_interpreted_outcome_with_unknown_reason_rejected() -> None:
    with pytest.raises(ValidationError):
        CodeInterpretationOut(
            outcome="INTERPRETED",
            component_candidates=[_component()],
            unknown_reason="PROVIDER_UNCERTAIN",
        )
