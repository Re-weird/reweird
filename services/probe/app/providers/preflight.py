from collections import defaultdict
from dataclasses import dataclass
from typing import Literal

from app.schemas import GroundedItem, StructuredEvidence


def is_baseline_trusted(evidence: StructuredEvidence) -> bool:
    status = evidence.baseline.get("status")
    if status is None or status == "UNKNOWN":
        return False
    return evidence.baseline.get("trusted") is not False


@dataclass
class PreflightResult:
    ok: bool
    reason: Literal["NO_EVIDENCE", "CONFLICTING_RULES"] | None
    filtered_evidence: StructuredEvidence | None
    filtered_grounded: list[GroundedItem]


def _fail(reason: Literal["NO_EVIDENCE", "CONFLICTING_RULES"]) -> PreflightResult:
    return PreflightResult(ok=False, reason=reason, filtered_evidence=None, filtered_grounded=[])


def run_preflight(evidence: StructuredEvidence, grounded: list[GroundedItem]) -> PreflightResult:
    if not evidence.rule_results:
        return _fail("NO_EVIDENCE")

    trusted = is_baseline_trusted(evidence)
    usable_rules = [
        rule
        for rule in evidence.rule_results
        if not (rule.id == "baseline-deviation" and not trusted)
    ]
    if not usable_rules:
        return _fail("NO_EVIDENCE")

    groups: dict[tuple[str, str | None], set[str]] = defaultdict(set)
    for rule in usable_rules:
        groups[(rule.id, rule.probe)].add(rule.status)
    if any(len(statuses) > 1 for statuses in groups.values()):
        return _fail("CONFLICTING_RULES")

    # Untrusted baseline data is REMOVED, not merely down-weighted: it never
    # appears in the evidence JSON or the allowed-reference list sent to the
    # model. This includes the evidence.baseline mapping itself, since its
    # numeric content (e.g. a stored average_voltage/frequency_hz) is exactly
    # the kind of untrusted value the model must never see, even once the
    # rule/fact that referenced it has been excluded.
    filtered_grounded = [
        item
        for item in grounded
        if not (item.category == "baseline_comparison" and not trusted)
        and not (item.category == "rule_result" and item.name == "baseline-deviation" and not trusted)
    ]
    filtered_evidence = evidence.model_copy(
        update={
            "rule_results": usable_rules,
            "baseline_comparison": evidence.baseline_comparison if trusted else None,
            "baseline": (
                evidence.baseline
                if trusted
                else {"status": evidence.baseline.get("status", "UNKNOWN")}
            ),
        }
    )
    return PreflightResult(
        ok=True, reason=None, filtered_evidence=filtered_evidence, filtered_grounded=filtered_grounded
    )
