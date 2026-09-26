from app.grounding import ground_evidence
from app.schemas import RuleResult, StructuredEvidence
from tests.conftest import load_evidence


def test_ref_ids_are_deterministic_across_calls() -> None:
    evidence = load_evidence("intermittent_echo")
    first = ground_evidence(evidence)
    second = ground_evidence(evidence)
    assert first == second


def test_ref_ids_are_stable_string_values() -> None:
    evidence = load_evidence("intermittent_echo")
    grounded = ground_evidence(evidence)
    ref_ids = {item.ref_id for item in grounded}
    assert "rule:p3:unexpected-dropout" in ref_ids
    assert "measurement:p3:pulse_count" in ref_ids


def test_ref_id_unaffected_by_unrelated_reordering() -> None:
    evidence = load_evidence("missing_power")
    original = {item.name: item.ref_id for item in ground_evidence(evidence)}

    reordered = evidence.model_copy(
        update={"measurements": list(reversed(evidence.measurements or []))}
    )
    after = {item.name: item.ref_id for item in ground_evidence(reordered)}

    assert original == after


def test_collision_disambiguation() -> None:
    evidence = load_evidence("conflicting_evidence")
    grounded = ground_evidence(evidence)
    rule_ref_ids = [item.ref_id for item in grounded if item.category == "rule_result"]
    assert rule_ref_ids == ["rule:p3:unexpected-dropout", "rule:p3:unexpected-dropout:2"]


def test_grounding_covers_all_categories() -> None:
    evidence = StructuredEvidence(
        probe="P3",
        role="ECHO",
        expected={},
        observed={},
        baseline={"status": "USER_CONFIRMED_HEALTHY"},
        measurements=[{"probe": "P3", "name": "m1", "value": 1, "provenance": "MEASURED"}],
        derived_facts=[{"probe": "P3", "name": "d1", "value": 1, "provenance": "DERIVED"}],
        specification_results=[
            {"probe": "P3", "name": "s1", "value": 1, "provenance": "SPECIFICATION"}
        ],
        baseline_comparison=[{"probe": "P3", "name": "b1", "value": 1, "provenance": "BASELINE"}],
        rule_results=[RuleResult(id="r1", probe="P3", status="pass", message="ok")],
    )
    grounded = ground_evidence(evidence)
    categories = {item.category for item in grounded}
    assert categories == {
        "measurement",
        "derived_fact",
        "specification_result",
        "baseline_comparison",
        "rule_result",
    }
    assert len(grounded) == 5
