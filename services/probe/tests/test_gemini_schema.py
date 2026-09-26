import pytest
from pydantic import ValidationError

from app.providers.gemini_schema import GeminiHypothesisOut, GeminiResponseOut


def _hypothesis(**overrides) -> dict:
    fields = dict(
        label="Unexpected signal dropout",
        explanation="ECHO shows dropouts",
        confidence=0.7,
        grounded_in=["rule:p3:unexpected-dropout"],
        supporting_rule_ids=["unexpected-dropout"],
    )
    fields.update(overrides)
    return fields


def test_valid_diagnosed_round_trips() -> None:
    response = GeminiResponseOut(outcome="DIAGNOSED", hypotheses=[_hypothesis()])
    dumped = response.model_dump_json()
    reloaded = GeminiResponseOut.model_validate_json(dumped)
    assert reloaded == response


def test_valid_unknown_round_trips() -> None:
    response = GeminiResponseOut(outcome="UNKNOWN", unknown_reason="PROVIDER_UNCERTAIN")
    dumped = response.model_dump_json()
    reloaded = GeminiResponseOut.model_validate_json(dumped)
    assert reloaded == response


def test_extra_field_rejected() -> None:
    with pytest.raises(ValidationError):
        GeminiHypothesisOut(**_hypothesis(), extra_field="not allowed")


def test_extra_field_rejected_on_response() -> None:
    with pytest.raises(ValidationError):
        GeminiResponseOut(outcome="UNKNOWN", unknown_reason="PROVIDER_UNCERTAIN", bogus="x")


@pytest.mark.parametrize("confidence", [1.5, -0.1])
def test_confidence_out_of_range_rejected(confidence: float) -> None:
    with pytest.raises(ValidationError):
        GeminiHypothesisOut(**_hypothesis(confidence=confidence))


def test_empty_grounded_in_list_rejected() -> None:
    with pytest.raises(ValidationError):
        GeminiHypothesisOut(**_hypothesis(grounded_in=[]))


def test_empty_supporting_rule_ids_list_rejected() -> None:
    with pytest.raises(ValidationError):
        GeminiHypothesisOut(**_hypothesis(supporting_rule_ids=[]))


def test_empty_string_item_in_grounded_in_rejected() -> None:
    with pytest.raises(ValidationError):
        GeminiHypothesisOut(**_hypothesis(grounded_in=["rule:p3:unexpected-dropout", ""]))


def test_overlong_ref_id_item_rejected() -> None:
    with pytest.raises(ValidationError):
        GeminiHypothesisOut(**_hypothesis(grounded_in=["x" * 200]))


def test_overlong_rule_id_item_rejected() -> None:
    with pytest.raises(ValidationError):
        GeminiHypothesisOut(**_hypothesis(supporting_rule_ids=["x" * 100]))


def test_overlong_reasoning_note_rejected() -> None:
    with pytest.raises(ValidationError):
        GeminiResponseOut(
            outcome="UNKNOWN",
            unknown_reason="PROVIDER_UNCERTAIN",
            reasoning_notes=["x" * 600],
        )


def test_duplicate_grounded_in_rejected() -> None:
    with pytest.raises(ValidationError):
        GeminiHypothesisOut(**_hypothesis(grounded_in=["ref1", "ref1"]))


def test_duplicate_supporting_rule_ids_rejected() -> None:
    with pytest.raises(ValidationError):
        GeminiHypothesisOut(**_hypothesis(supporting_rule_ids=["rule1", "rule1"]))


def test_too_many_hypotheses_rejected() -> None:
    with pytest.raises(ValidationError):
        GeminiResponseOut(outcome="DIAGNOSED", hypotheses=[_hypothesis() for _ in range(11)])


def test_unknown_outcome_with_hypotheses_rejected() -> None:
    with pytest.raises(ValidationError):
        GeminiResponseOut(
            outcome="UNKNOWN", unknown_reason="PROVIDER_UNCERTAIN", hypotheses=[_hypothesis()]
        )


def test_diagnosed_outcome_with_no_hypotheses_rejected() -> None:
    with pytest.raises(ValidationError):
        GeminiResponseOut(outcome="DIAGNOSED", hypotheses=[])


def test_diagnosed_outcome_with_unknown_reason_rejected() -> None:
    with pytest.raises(ValidationError):
        GeminiResponseOut(
            outcome="DIAGNOSED", hypotheses=[_hypothesis()], unknown_reason="PROVIDER_UNCERTAIN"
        )
