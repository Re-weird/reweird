import json

from fastapi.testclient import TestClient

from app.catalog import get_default_catalog
from app.planner import TestPlanner
from app.probe import run_probe
from app.providers.base import AIProviderResult
from app.providers.fake import FakeAIProvider
from app.schemas import Hypothesis, StructuredEvidence
from tests.conftest import FIXTURES_DIR, load_evidence

planner = TestPlanner()
catalog = get_default_catalog()


class _CountingProvider:
    """Counts .interpret() calls; never touches the network. Used to prove
    an unknown component_id short-circuits before any provider call."""

    def __init__(self) -> None:
        self.calls = 0

    def interpret(self, evidence, grounded) -> AIProviderResult:
        self.calls += 1
        return AIProviderResult(outcome="DIAGNOSED", hypotheses=[], reasoning_notes=[])


# ---------------------------------------------------------------------------
# Backward compatibility (instruction #2): every Milestone 1/2 call must
# continue behaving exactly as before, whether component_id is omitted from
# the call entirely or explicitly passed as None.
# ---------------------------------------------------------------------------


def test_run_probe_without_component_id_is_unchanged_from_milestone_1_2() -> None:
    evidence = load_evidence("healthy")
    provider = FakeAIProvider()

    result_three_args = run_probe(evidence, provider, planner)
    result_explicit_none = run_probe(evidence, provider, planner, catalog=catalog, component_id=None)

    assert result_three_args == result_explicit_none
    assert result_three_args.outcome == "DIAGNOSED"
    assert result_three_args.hypotheses[0].label == "No fault detected"


def test_post_probe_without_component_id_field_matches_pre_milestone_4_response(client: TestClient) -> None:
    payload = {"evidence": json.loads((FIXTURES_DIR / "intermittent_echo.json").read_text())}
    response = client.post("/probe", json=payload)
    assert response.status_code == 200
    body = response.json()
    assert body["outcome"] == "DIAGNOSED"
    assert body["hypotheses"][0]["supporting_rule_ids"] == ["unexpected-dropout"]


# ---------------------------------------------------------------------------
# Component-driven specification evaluation
# ---------------------------------------------------------------------------


def _run(fixture_name: str, component_id: str | None, provider=None):
    evidence = load_evidence(fixture_name)
    return run_probe(evidence, provider or FakeAIProvider(), planner, catalog, component_id)


def test_healthy_hc_sr04_evidence_with_component_id_is_diagnosed_no_fault() -> None:
    result = _run("hc_sr04_healthy", "hc-sr04")
    assert result.outcome == "DIAGNOSED"
    assert result.hypotheses[0].label == "No fault detected"


def test_voltage_outside_specification_is_deterministically_detected() -> None:
    result = _run("hc_sr04_voltage_outside_spec", "hc-sr04")
    assert result.outcome == "DIAGNOSED"
    assert result.hypotheses[0].supporting_rule_ids == ["voltage-outside-specification"]


def test_pulse_width_outside_specification_is_deterministically_detected() -> None:
    result = _run("hc_sr04_pulse_outside_spec", "hc-sr04")
    assert result.outcome == "DIAGNOSED"
    assert "pulse-width-outside-specification" in result.hypotheses[0].supporting_rule_ids


def test_established_signal_absence_is_missing_signal_not_not_evaluable() -> None:
    result = _run("hc_sr04_missing_echo_activity", "hc-sr04")
    assert result.outcome == "DIAGNOSED"
    assert result.hypotheses[0].supporting_rule_ids == ["missing-signal"]


def test_lacking_observation_is_not_evaluable_not_missing_signal() -> None:
    result = _run("hc_sr04_not_evaluable", "hc-sr04")
    assert result.outcome == "DIAGNOSED"
    assert result.hypotheses[0].supporting_rule_ids == ["specification-not-evaluable"]


def test_unknown_component_id_is_unknown_component_with_zero_provider_calls() -> None:
    provider = _CountingProvider()
    result = _run("hc_sr04_healthy", "does-not-exist", provider=provider)
    assert result.outcome == "UNKNOWN"
    assert result.unknown_reason == "UNKNOWN_COMPONENT"
    assert result.hypotheses == []
    assert provider.calls == 0


def test_post_probe_unknown_component_id_end_to_end(client: TestClient) -> None:
    payload = {
        "evidence": json.loads((FIXTURES_DIR / "hc_sr04_healthy.json").read_text()),
        "component_id": "does-not-exist",
    }
    response = client.post("/probe", json=payload)
    assert response.status_code == 200
    body = response.json()
    assert body["outcome"] == "UNKNOWN"
    assert body["unknown_reason"] == "UNKNOWN_COMPONENT"


# ---------------------------------------------------------------------------
# Gemini cannot override a deterministic rule status (instruction #7),
# enforced structurally: Hypothesis has no field capable of mutating a
# RuleResult at all - it can only cite/explain rule ids that already exist.
# ---------------------------------------------------------------------------


def test_hypothesis_schema_has_no_field_capable_of_mutating_rule_status() -> None:
    mutating_field_names = {"status", "rule_status", "override_status", "pass", "fail"}
    assert mutating_field_names.isdisjoint(Hypothesis.model_fields)


def test_confident_contradicting_provider_output_does_not_alter_deterministic_rule_results() -> None:
    """Even a provider that is fully confident the opposite is true cannot
    change what the deterministic evaluator already decided - it can only
    add an explanation referencing the rule ids/facts it was given."""
    evidence = load_evidence("hc_sr04_voltage_outside_spec")

    class _ContradictingProvider:
        def interpret(self, evidence: StructuredEvidence, grounded) -> AIProviderResult:
            # A provider claiming (with high confidence) that everything is
            # fine has no way to touch evidence.rule_results at all - the
            # type system gives it no such field.
            return AIProviderResult(
                outcome="DIAGNOSED",
                hypotheses=[
                    Hypothesis(
                        rank=1,
                        label="No fault detected (contradicting claim)",
                        explanation="Claiming everything is fine regardless of the rule.",
                        confidence=0.99,
                        grounded_in=[],
                        supporting_rule_ids=[],
                    )
                ],
                reasoning_notes=[],
            )

    result = run_probe(evidence, _ContradictingProvider(), planner, catalog, "hc-sr04")
    # The provider's opinion is reflected in its own hypothesis...
    assert result.hypotheses[0].label == "No fault detected (contradicting claim)"
    # ...but the deterministic fact remains independently recoverable and
    # untouched: re-running the same evaluator on the same evidence still
    # finds the voltage failure, proving the provider never had any way to
    # mutate it.
    from app.specification import evaluate_component_specification

    entry = catalog["hc-sr04"]
    _, rules = evaluate_component_specification(entry, evidence.probe, evidence.role, evidence)
    assert rules[0].id == "voltage-outside-specification"
    assert rules[0].status == "fail"
