import json

from app.grounding import ground_evidence
from app.providers.gemini import build_prompt
from app.providers.preflight import run_preflight
from app.schemas import RuleResult, StructuredEvidence


def _evidence(**overrides) -> StructuredEvidence:
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


def test_no_rule_results_is_no_evidence() -> None:
    evidence = _evidence(rule_results=[])
    result = run_preflight(evidence, ground_evidence(evidence))
    assert result.ok is False
    assert result.reason == "NO_EVIDENCE"


def test_conflicting_statuses_is_conflicting_rules() -> None:
    evidence = _evidence(
        rule_results=[
            RuleResult(id="unexpected-dropout", probe="P3", status="fail", message="a"),
            RuleResult(id="unexpected-dropout", probe="P3", status="pass", message="b"),
        ]
    )
    result = run_preflight(evidence, ground_evidence(evidence))
    assert result.ok is False
    assert result.reason == "CONFLICTING_RULES"


def test_untrusted_baseline_with_only_baseline_deviation_is_no_evidence() -> None:
    evidence = _evidence(
        baseline={"status": "UNKNOWN"},
        rule_results=[
            RuleResult(id="baseline-deviation", probe="P3", status="warn", message="deviates")
        ],
    )
    result = run_preflight(evidence, ground_evidence(evidence))
    assert result.ok is False
    assert result.reason == "NO_EVIDENCE"


def test_untrusted_baseline_mixed_with_other_rule_filters_but_stays_usable() -> None:
    evidence = _evidence(
        baseline={"status": "UNKNOWN", "average_voltage": 5.01, "frequency_hz": 28.4},
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
    grounded = ground_evidence(evidence)
    result = run_preflight(evidence, grounded)

    assert result.ok is True
    assert result.reason is None
    assert result.filtered_evidence is not None
    assert [r.id for r in result.filtered_evidence.rule_results] == ["unexpected-dropout"]
    assert result.filtered_evidence.baseline_comparison is None
    assert result.filtered_evidence.baseline == {"status": "UNKNOWN"}
    assert "average_voltage" not in result.filtered_evidence.baseline
    assert "frequency_hz" not in result.filtered_evidence.baseline

    categories = {item.category for item in result.filtered_grounded}
    assert "baseline_comparison" not in categories
    rule_names = {item.name for item in result.filtered_grounded if item.category == "rule_result"}
    assert "baseline-deviation" not in rule_names
    assert "unexpected-dropout" in rule_names


def test_untrusted_baseline_values_never_reach_the_serialized_prompt() -> None:
    evidence = _evidence(
        baseline={"status": "UNKNOWN", "average_voltage": 5.01, "frequency_hz": 28.4},
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
    grounded = ground_evidence(evidence)
    result = run_preflight(evidence, grounded)
    assert result.filtered_evidence is not None

    _, user_content = build_prompt(result.filtered_evidence, result.filtered_grounded)
    serialized = json.dumps(user_content)

    assert "5.01" not in serialized
    assert "average_voltage" not in serialized
    assert "baseline_deviation_percent" not in serialized
    assert "15.1" not in serialized


def test_trusted_baseline_is_not_filtered() -> None:
    evidence = _evidence(
        baseline={"status": "USER_CONFIRMED_HEALTHY", "trusted": True, "frequency_hz": 28.4},
        baseline_comparison=[
            {
                "probe": "P3",
                "name": "baseline_deviation_percent",
                "value": 1.0,
                "unit": "%",
                "provenance": "BASELINE",
            }
        ],
        rule_results=[
            RuleResult(id="baseline-deviation", probe="P3", status="pass", message="matches")
        ],
    )
    grounded = ground_evidence(evidence)
    result = run_preflight(evidence, grounded)

    assert result.ok is True
    assert result.filtered_evidence is not None
    assert result.filtered_evidence.baseline == evidence.baseline
    assert result.filtered_evidence.baseline_comparison == evidence.baseline_comparison
    assert len(result.filtered_grounded) == len(grounded)
