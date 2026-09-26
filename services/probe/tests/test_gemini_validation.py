from app.grounding import ground_evidence
from app.providers.gemini_schema import GeminiResponseOut
from app.providers.gemini_validation import validate_and_rank
from app.providers.preflight import run_preflight
from app.schemas import RuleResult, StructuredEvidence
from tests.conftest import load_evidence


def _evidence(**overrides) -> StructuredEvidence:
    fields = dict(
        probe="P3",
        role="ECHO",
        expected={},
        observed={},
        baseline={"status": "USER_CONFIRMED_HEALTHY", "trusted": True},
        rule_results=[
            RuleResult(id="unexpected-dropout", probe="P3", status="fail", message="dropouts")
        ],
    )
    fields.update(overrides)
    return StructuredEvidence(**fields)


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


def test_valid_multi_hypothesis_response_ranks_by_confidence() -> None:
    evidence = _evidence(
        rule_results=[
            RuleResult(id="unexpected-dropout", probe="P3", status="fail", message="a"),
            RuleResult(id="missing-signal", probe="P3", status="fail", message="b"),
        ]
    )
    grounded = ground_evidence(evidence)
    raw = GeminiResponseOut(
        outcome="DIAGNOSED",
        hypotheses=[
            _hypothesis(
                confidence=0.5,
                grounded_in=["rule:p3:unexpected-dropout"],
                supporting_rule_ids=["unexpected-dropout"],
            ),
            _hypothesis(
                confidence=0.9,
                grounded_in=["rule:p3:missing-signal"],
                supporting_rule_ids=["missing-signal"],
            ),
        ],
    )
    result = validate_and_rank(raw, evidence, grounded)
    assert result.outcome == "DIAGNOSED"
    assert [h.supporting_rule_ids[0] for h in result.hypotheses] == [
        "missing-signal",
        "unexpected-dropout",
    ]
    assert [h.rank for h in result.hypotheses] == [1, 2]


def test_hallucinated_ref_id_invalidates_entire_response() -> None:
    evidence = _evidence()
    grounded = ground_evidence(evidence)
    raw = GeminiResponseOut(
        outcome="DIAGNOSED",
        hypotheses=[
            _hypothesis(grounded_in=["rule:p3:unexpected-dropout", "measurement:p3:nonexistent"])
        ],
    )
    result = validate_and_rank(raw, evidence, grounded)
    assert result.outcome == "UNKNOWN"
    assert result.unknown_reason == "INVALID_PROVIDER_OUTPUT"
    assert result.hypotheses == []


def test_hallucinated_rule_id_invalidates_entire_response() -> None:
    evidence = _evidence()
    grounded = ground_evidence(evidence)
    raw = GeminiResponseOut(
        outcome="DIAGNOSED",
        hypotheses=[
            _hypothesis(
                grounded_in=["rule:p3:unexpected-dropout"],
                supporting_rule_ids=["nonexistent-rule"],
            )
        ],
    )
    result = validate_and_rank(raw, evidence, grounded)
    assert result.outcome == "UNKNOWN"
    assert result.unknown_reason == "INVALID_PROVIDER_OUTPUT"


def test_mismatched_rule_and_ref_invalidates_entire_response() -> None:
    # supporting_rule_ids names a rule that IS present in evidence, but
    # grounded_in never cites that rule's own rule_result reference.
    evidence = _evidence(
        rule_results=[
            RuleResult(id="unexpected-dropout", probe="P3", status="fail", message="a"),
            RuleResult(id="missing-signal", probe="P3", status="fail", message="b"),
        ]
    )
    grounded = ground_evidence(evidence)
    raw = GeminiResponseOut(
        outcome="DIAGNOSED",
        hypotheses=[
            _hypothesis(
                grounded_in=["rule:p3:unexpected-dropout"],
                supporting_rule_ids=["missing-signal"],
            )
        ],
    )
    result = validate_and_rank(raw, evidence, grounded)
    assert result.outcome == "UNKNOWN"
    assert result.unknown_reason == "INVALID_PROVIDER_OUTPUT"


def test_one_bad_hypothesis_invalidates_an_otherwise_good_response() -> None:
    evidence = _evidence(
        rule_results=[
            RuleResult(id="unexpected-dropout", probe="P3", status="fail", message="a"),
            RuleResult(id="missing-signal", probe="P3", status="fail", message="b"),
        ]
    )
    grounded = ground_evidence(evidence)
    raw = GeminiResponseOut(
        outcome="DIAGNOSED",
        hypotheses=[
            _hypothesis(
                grounded_in=["rule:p3:unexpected-dropout"],
                supporting_rule_ids=["unexpected-dropout"],
            ),
            _hypothesis(
                grounded_in=["measurement:p3:nonexistent"],
                supporting_rule_ids=["missing-signal"],
            ),
        ],
    )
    result = validate_and_rank(raw, evidence, grounded)
    assert result.outcome == "UNKNOWN"
    assert result.unknown_reason == "INVALID_PROVIDER_OUTPUT"
    assert result.hypotheses == []


def test_citing_a_preflight_excluded_baseline_ref_is_rejected_as_unknown() -> None:
    evidence = _evidence(
        baseline={"status": "UNKNOWN"},
        baseline_comparison=[
            {
                "probe": "P3",
                "name": "baseline_deviation_percent",
                "value": 15.1,
                "unit": "%",
                "provenance": "BASELINE",
            }
        ],
        rule_results=[
            RuleResult(id="baseline-deviation", probe="P3", status="warn", message="deviates"),
            RuleResult(id="unexpected-dropout", probe="P3", status="fail", message="dropouts"),
        ],
    )
    full_grounded = ground_evidence(evidence)
    pre = run_preflight(evidence, full_grounded)
    assert pre.ok is True
    assert pre.filtered_evidence is not None

    # The model hallucinates a citation to the baseline_comparison ref that
    # preflight excluded from allowed_grounded (it exists in the ORIGINAL
    # full grounded list, but not in the filtered one passed to validation).
    excluded_ref = next(
        item.ref_id for item in full_grounded if item.category == "baseline_comparison"
    )
    raw = GeminiResponseOut(
        outcome="DIAGNOSED",
        hypotheses=[
            _hypothesis(
                grounded_in=["rule:p3:unexpected-dropout", excluded_ref],
                supporting_rule_ids=["unexpected-dropout"],
            )
        ],
    )
    result = validate_and_rank(raw, pre.filtered_evidence, pre.filtered_grounded)
    assert result.outcome == "UNKNOWN"
    assert result.unknown_reason == "INVALID_PROVIDER_OUTPUT"


def test_model_declared_unknown_passes_through_as_provider_uncertain() -> None:
    evidence = _evidence()
    grounded = ground_evidence(evidence)
    raw = GeminiResponseOut(
        outcome="UNKNOWN",
        unknown_reason="PROVIDER_UNCERTAIN",
        reasoning_notes=["Not confident enough given the evidence."],
    )
    result = validate_and_rank(raw, evidence, grounded)
    assert result.outcome == "UNKNOWN"
    assert result.unknown_reason == "PROVIDER_UNCERTAIN"
    assert result.hypotheses == []
    assert result.reasoning_notes == ["Not confident enough given the evidence."]


def test_intermittent_echo_fixture_valid_response_matches_expected_shape() -> None:
    evidence = load_evidence("intermittent_echo")
    grounded = ground_evidence(evidence)
    rule_ref = next(item.ref_id for item in grounded if item.category == "rule_result")
    raw = GeminiResponseOut(
        outcome="DIAGNOSED",
        hypotheses=[
            _hypothesis(
                grounded_in=[rule_ref],
                supporting_rule_ids=["unexpected-dropout"],
            )
        ],
    )
    result = validate_and_rank(raw, evidence, grounded)
    assert result.outcome == "DIAGNOSED"
    assert len(result.hypotheses) == 1
    assert result.hypotheses[0].rank == 1
