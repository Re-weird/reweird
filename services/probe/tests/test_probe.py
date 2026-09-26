from app.grounding import ground_evidence
from app.planner import TestPlanner
from app.providers.fake import FakeAIProvider
from app.schemas import RuleResult, StructuredEvidence
from tests.conftest import load_evidence

provider = FakeAIProvider()
planner = TestPlanner()


def _diagnose(name: str):
    evidence = load_evidence(name)
    grounded = ground_evidence(evidence)
    result = provider.interpret(evidence, grounded)
    recommended_test = planner.recommend(evidence, result)
    return evidence, result, recommended_test


def test_healthy_is_diagnosed_with_no_fault_hypothesis() -> None:
    _, result, recommended_test = _diagnose("healthy")
    assert result.outcome == "DIAGNOSED"
    assert len(result.hypotheses) == 1
    assert result.hypotheses[0].label == "No fault detected"
    assert "no further test" in recommended_test.lower()


def test_missing_power_ranks_voltage_above_missing_signal() -> None:
    _, result, recommended_test = _diagnose("missing_power")
    assert result.outcome == "DIAGNOSED"
    assert [h.supporting_rule_ids[0] for h in result.hypotheses] == [
        "voltage-outside-specification",
        "missing-signal",
    ]
    assert result.hypotheses[0].rank == 1
    assert result.hypotheses[1].rank == 2
    assert "divider" in recommended_test.lower()


def test_missing_trigger_diagnoses_missing_signal() -> None:
    _, result, recommended_test = _diagnose("missing_trigger")
    assert result.outcome == "DIAGNOSED"
    assert len(result.hypotheses) == 1
    assert result.hypotheses[0].supporting_rule_ids == ["missing-signal"]
    assert "probe assignment" in recommended_test.lower()


def test_missing_echo_diagnoses_missing_signal() -> None:
    _, result, _ = _diagnose("missing_echo")
    assert result.outcome == "DIAGNOSED"
    assert len(result.hypotheses) == 1
    assert result.hypotheses[0].supporting_rule_ids == ["missing-signal"]


def test_intermittent_echo_recommends_movement_correlation_test() -> None:
    _, result, recommended_test = _diagnose("intermittent_echo")
    assert result.outcome == "DIAGNOSED"
    assert len(result.hypotheses) == 1
    assert result.hypotheses[0].supporting_rule_ids == ["unexpected-dropout"]
    assert "movement correlation" in recommended_test.lower()
    assert result.hypotheses[0].grounded_in  # cites at least one ref id


def test_movement_warning_never_claims_correlation_is_confirmed() -> None:
    evidence = _base_evidence(
        rule_results=[
            RuleResult(
                id="movement-correlation",
                probe="P3",
                status="warn",
                message="Movement correlation has not been tested",
            )
        ]
    )
    result = provider.interpret(evidence, ground_evidence(evidence))
    assert result.hypotheses[0].label == "Movement correlation not yet tested"
    assert "confirmed" not in result.hypotheses[0].label.lower()


def test_conflicting_evidence_is_unknown_with_conflicting_rules_reason() -> None:
    _, result, recommended_test = _diagnose("conflicting_evidence")
    assert result.outcome == "UNKNOWN"
    assert result.unknown_reason == "CONFLICTING_RULES"
    assert result.hypotheses == []
    assert "conflicting rule results" in recommended_test.lower()


def test_insufficient_evidence_is_unknown_with_no_evidence_reason() -> None:
    _, result, recommended_test = _diagnose("insufficient_evidence")
    assert result.outcome == "UNKNOWN"
    assert result.unknown_reason == "NO_EVIDENCE"
    assert result.hypotheses == []
    assert "capture an initial measurement window" in recommended_test.lower()


def _base_evidence(**overrides) -> StructuredEvidence:
    fields = dict(
        probe="P3",
        role="ECHO",
        expected={},
        observed={},
        baseline={"status": "USER_CONFIRMED_HEALTHY", "trusted": True},
        rule_results=[],
    )
    fields.update(overrides)
    return StructuredEvidence(**fields)


def test_untrusted_baseline_never_grounds_or_ranks_a_hypothesis() -> None:
    evidence = _base_evidence(
        baseline={"status": "UNKNOWN"},
        rule_results=[
            RuleResult(id="baseline-deviation", probe="P3", status="warn", message="deviates")
        ],
    )
    grounded = ground_evidence(evidence)
    result = provider.interpret(evidence, grounded)
    assert result.outcome == "UNKNOWN"
    assert result.unknown_reason == "NO_EVIDENCE"
    assert all(
        "baseline-deviation" not in h.supporting_rule_ids for h in result.hypotheses
    )


def test_power_rail_instability_produces_high_confidence_hypothesis() -> None:
    evidence = _base_evidence(
        probe="P1",
        role="POWER",
        rule_results=[
            RuleResult(id="power-rail-instability", probe="P1", status="fail", message="unstable")
        ],
    )
    grounded = ground_evidence(evidence)
    result = provider.interpret(evidence, grounded)
    assert result.outcome == "DIAGNOSED"
    assert result.hypotheses[0].confidence == 0.90
    assert result.hypotheses[0].supporting_rule_ids == ["power-rail-instability"]


def test_simultaneous_dropout_recommends_shared_electrical_test() -> None:
    evidence = _base_evidence(
        rule_results=[
            RuleResult(
                id="simultaneous-dropout",
                probe="P2,P3",
                status="fail",
                message="shared cause",
            )
        ],
    )
    grounded = ground_evidence(evidence)
    result = provider.interpret(evidence, grounded)
    recommended_test = planner.recommend(evidence, result)
    assert result.outcome == "DIAGNOSED"
    assert "shared electrical cause" in recommended_test.lower()
