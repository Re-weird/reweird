from collections import defaultdict

from app.providers.base import AIProviderResult
from app.schemas import GroundedItem, Hypothesis, RuleResult, StructuredEvidence

_CONFIDENCE: dict[tuple[str, str], float] = {
    ("movement-correlation", "fail"): 0.92,
    ("power-rail-instability", "fail"): 0.90,
    ("simultaneous-dropout", "fail"): 0.90,
    ("voltage-outside-specification", "fail"): 0.88,
    ("pulse-width-outside-specification", "fail"): 0.85,
    ("missing-signal", "fail"): 0.85,
    ("unexpected-dropout", "fail"): 0.70,
    ("baseline-deviation", "fail"): 0.65,
    ("baseline-deviation", "warn"): 0.55,
    ("movement-correlation", "warn"): 0.40,
    # Milestone 4: "we don't have enough data to check this" is never a
    # fault hypothesis in its own right - kept deliberately low confidence,
    # lowest priority, so it never outranks an actual deterministic finding.
    ("specification-not-evaluable", "warn"): 0.15,
}

_PRIORITY = [
    "movement-correlation",
    "power-rail-instability",
    "simultaneous-dropout",
    "voltage-outside-specification",
    "pulse-width-outside-specification",
    "missing-signal",
    "unexpected-dropout",
    "baseline-deviation",
    "specification-not-evaluable",
]

_LABELS: dict[str, str] = {
    "movement-correlation": "Intermittent connection confirmed by movement correlation",
    "power-rail-instability": "Power rail instability",
    "simultaneous-dropout": "Shared electrical fault across probes",
    "voltage-outside-specification": "Voltage outside specification",
    "pulse-width-outside-specification": "Pulse width outside specification",
    "missing-signal": "Missing expected signal activity",
    "unexpected-dropout": "Unexpected signal dropout",
    "baseline-deviation": "Deviation from trusted baseline",
    "specification-not-evaluable": "Specification could not be evaluated",
}


def _label(rule: RuleResult) -> str:
    if rule.id == "movement-correlation" and rule.status == "warn":
        return "Movement correlation not yet tested"
    return _LABELS.get(rule.id, rule.id)


def _baseline_trusted(evidence: StructuredEvidence) -> bool:
    status = evidence.baseline.get("status")
    if status is None or status == "UNKNOWN":
        return False
    if evidence.baseline.get("trusted") is False:
        return False
    return True


def _probe_tokens(rule: RuleResult) -> set[str]:
    if not rule.probe:
        return set()
    return {token.strip() for token in rule.probe.split(",") if token.strip()}


def _explanation(rule_id: str, evidence: StructuredEvidence, messages: list[str]) -> str:
    joined = "; ".join(messages)
    return f"{evidence.role} ({evidence.probe}): {joined}"


class FakeAIProvider:
    """Deterministic rule-based interpreter. No network calls, no randomness."""

    def interpret(
        self, evidence: StructuredEvidence, grounded: list[GroundedItem]
    ) -> AIProviderResult:
        if not evidence.rule_results:
            return AIProviderResult(
                outcome="UNKNOWN",
                hypotheses=[],
                unknown_reason="NO_EVIDENCE",
                reasoning_notes=[
                    "No rule_results were present in the submitted evidence; PROBE cannot "
                    "form a grounded hypothesis without at least one deterministic rule outcome."
                ],
            )

        baseline_ok = _baseline_trusted(evidence)
        usable_rules = [
            rule
            for rule in evidence.rule_results
            if not (rule.id == "baseline-deviation" and not baseline_ok)
        ]
        if not usable_rules:
            return AIProviderResult(
                outcome="UNKNOWN",
                hypotheses=[],
                unknown_reason="NO_EVIDENCE",
                reasoning_notes=[
                    "All rule_results were baseline-deviation entries backed by an "
                    "UNKNOWN/untrusted baseline and were excluded; no other deterministic "
                    "evidence remains."
                ],
            )

        groups: dict[tuple[str, str | None], set[str]] = defaultdict(set)
        for rule in usable_rules:
            groups[(rule.id, rule.probe)].add(rule.status)

        conflicts = {key: statuses for key, statuses in groups.items() if len(statuses) > 1}
        if conflicts:
            (rule_id, probe), statuses = sorted(conflicts.items())[0]
            return AIProviderResult(
                outcome="UNKNOWN",
                hypotheses=[],
                unknown_reason="CONFLICTING_RULES",
                reasoning_notes=[
                    f"Conflicting rule_results for rule '{rule_id}' on {probe}: "
                    f"statuses {sorted(statuses)} cannot be reconciled deterministically."
                ],
            )

        failing = [rule for rule in usable_rules if rule.status in ("fail", "warn")]
        if not failing:
            ref_ids = [item.ref_id for item in grounded if item.category == "rule_result"]
            return AIProviderResult(
                outcome="DIAGNOSED",
                reasoning_notes=[],
                hypotheses=[
                    Hypothesis(
                        rank=1,
                        label="No fault detected",
                        confidence=0.95,
                        explanation=(
                            f"All deterministic rules passed for {evidence.probe} "
                            f"({evidence.role}); the signal is within its configured "
                            "specification and matches its trusted baseline."
                        ),
                        grounded_in=ref_ids,
                        supporting_rule_ids=[rule.id for rule in usable_rules],
                    )
                ],
            )

        by_rule_id: dict[str, list[RuleResult]] = defaultdict(list)
        for rule in failing:
            by_rule_id[rule.id].append(rule)

        candidates: list[tuple[float, int, str, Hypothesis]] = []
        for rule_id, rules in by_rule_id.items():
            best = max(rules, key=lambda r: _CONFIDENCE.get((r.id, r.status), 0.0))
            confidence = _CONFIDENCE.get((best.id, best.status), 0.0)

            probes = {token for rule in rules for token in _probe_tokens(rule)} or {
                evidence.probe
            }
            ref_ids: list[str] = []
            for item in grounded:
                if item.category == "rule_result" and item.name == rule_id:
                    ref_ids.append(item.ref_id)
                elif item.category != "rule_result" and item.probe in probes:
                    ref_ids.append(item.ref_id)

            priority_index = (
                _PRIORITY.index(rule_id) if rule_id in _PRIORITY else len(_PRIORITY)
            )
            hypothesis = Hypothesis(
                rank=0,
                label=_label(best),
                confidence=confidence,
                explanation=_explanation(rule_id, evidence, [rule.message for rule in rules]),
                grounded_in=ref_ids,
                supporting_rule_ids=[rule_id],
            )
            candidates.append((confidence, priority_index, evidence.probe, hypothesis))

        candidates.sort(key=lambda c: (-c[0], c[1], c[2]))
        hypotheses = [c[3] for c in candidates]
        for rank, hypothesis in enumerate(hypotheses, start=1):
            hypothesis.rank = rank

        return AIProviderResult(outcome="DIAGNOSED", hypotheses=hypotheses, reasoning_notes=[])
