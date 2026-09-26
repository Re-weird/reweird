import time

from app.schemas import DiagnosticStep, EvidenceChange, PatchProposal, VerificationOutcome, VerificationResult

# Friendly labels for a fail->pass transition on a known rule id. A rule id
# not in this table still gets a generic, still-meaningful label - this is
# presentation only, never part of the pass/fail determination itself.
_RESTORATION_LABELS: dict[str, str] = {
    "missing-signal": "signal_restored",
    "voltage-outside-specification": "voltage_restored",
    "pulse-width-outside-specification": "timing_restored",
}


def _now_ms() -> int:
    return int(time.time() * 1000)


def compare_rule_status(
    before: DiagnosticStep, after: DiagnosticStep, focus_probe: str
) -> list[EvidenceChange]:
    """Pure, deterministic before/after comparison of rule_results scoped to
    ONE probe. Only ever compares an (id, probe) pair that exists on BOTH
    sides - a key present on only one side is reported as not_evaluable
    rather than guessed at, and no unit conversion or numeric guessing ever
    happens here (rule status is already unit-safe, computed once by
    Milestone 4's deterministic evaluator)."""
    before_rules = {
        (r.id, r.probe): r.status for r in before.evaluated_evidence.rule_results if r.probe == focus_probe
    }
    after_rules = {
        (r.id, r.probe): r.status for r in after.evaluated_evidence.rule_results if r.probe == focus_probe
    }

    changes: list[EvidenceChange] = []
    for key in sorted(set(before_rules) | set(after_rules)):
        rule_id, probe = key
        b = before_rules.get(key)
        a = after_rules.get(key)
        if b is None or a is None:
            change = "not_evaluable"
        elif b == "fail" and a == "pass":
            change = _RESTORATION_LABELS.get(rule_id, f"{rule_id}_resolved")
        elif b == "pass" and a == "fail":
            change = "regressed"
        elif b == a:
            change = "unchanged"
        else:
            change = "changed"
        changes.append(EvidenceChange(name=rule_id, kind="rule_status", probe=probe, before=b, after=a, change=change))
    return changes


def compare_fact_values(before: DiagnosticStep, after: DiagnosticStep, focus_probe: str) -> list[EvidenceChange]:
    """Secondary, transparency-only comparison of named facts (e.g.
    pulse_count). Exact-unit matching only, mirroring Milestone 4's own
    discipline: a unit mismatch or a fact missing on either side is reported
    as incompatible/not_evaluable, never silently compared or converted."""
    def _facts(step: DiagnosticStep) -> dict[str, tuple[object, str | None]]:
        all_facts = [*(step.evaluated_evidence.measurements or []), *(step.evaluated_evidence.derived_facts or [])]
        return {f.name: (f.value, f.unit) for f in all_facts if f.probe == focus_probe}

    before_facts = _facts(before)
    after_facts = _facts(after)

    changes: list[EvidenceChange] = []
    for name in sorted(set(before_facts) | set(after_facts)):
        b_pair = before_facts.get(name)
        a_pair = after_facts.get(name)
        if b_pair is None or a_pair is None:
            changes.append(
                EvidenceChange(name=name, kind="fact_value", probe=focus_probe, before=None, after=None, change="not_evaluable")
            )
            continue
        b_value, b_unit = b_pair
        a_value, a_unit = a_pair
        if b_unit != a_unit:
            changes.append(
                EvidenceChange(
                    name=name, kind="fact_value", probe=focus_probe,
                    before=str(b_value), after=str(a_value), change="incompatible_units",
                )
            )
            continue
        change = "unchanged" if b_value == a_value else "changed"
        changes.append(
            EvidenceChange(name=name, kind="fact_value", probe=focus_probe, before=str(b_value), after=str(a_value), change=change)
        )
    return changes


_RESOLVED_CHANGE_LABELS = set(_RESTORATION_LABELS.values())


def determine_outcome(
    changes: list[EvidenceChange], predicted_rule_ids: set[str]
) -> tuple[VerificationOutcome, str, list[str]]:
    """Never claims SUPPORTED on missing evidence. `predicted_rule_ids` is
    the set of rule ids that were FAILING in the before step for the
    session's own focus probe - i.e. the problem this patch aims to
    resolve. A rule id outside this table still counts as "resolved" via
    the generic f"{rule_id}_resolved" label produced by compare_rule_status."""
    if not predicted_rule_ids:
        return (
            "INCONCLUSIVE",
            "No failing rule existed for the session's focus signal before the patch, so there is "
            "nothing deterministic to verify a restoration against.",
            ["No pre-patch failure was recorded for the session's focus probe."],
        )

    relevant = [c for c in changes if c.kind == "rule_status" and c.name in predicted_rule_ids]
    missing = [c for c in relevant if c.change == "not_evaluable"]
    if len(relevant) < len(predicted_rule_ids) or missing:
        return (
            "INCONCLUSIVE",
            "The after-evidence did not include a comparable measurement for every rule that failed "
            "before the patch, so restoration cannot be determined either way.",
            [
                f"No after-evidence comparable to before rule '{name}' on the focus probe."
                for name in predicted_rule_ids
                if name not in {c.name for c in relevant if c.change != "not_evaluable"}
            ],
        )

    resolved = [c for c in relevant if c.change in _RESOLVED_CHANGE_LABELS or c.change.endswith("_resolved")]
    if len(resolved) == len(relevant):
        return (
            "SUPPORTED",
            "Every rule that was failing before the patch is passing in the after-evidence, matching "
            "the proposal's predicted effect.",
            [],
        )

    still_failing = [c for c in relevant if c.after == "fail"]
    if still_failing:
        return (
            "NOT_SUPPORTED",
            "The after-evidence still shows a failing result for the rule(s) the patch was expected to "
            "resolve; the predicted effect did not occur.",
            [],
        )

    return (
        "INCONCLUSIVE",
        "The after-evidence changed but not cleanly into a resolved or still-failing state for every "
        "predicted rule; the result cannot be classified as SUPPORTED or NOT_SUPPORTED.",
        [f"Ambiguous change for rule '{c.name}': {c.before} -> {c.after}" for c in relevant if c.change not in _RESOLVED_CHANGE_LABELS and c.after != "fail"],
    )


def build_verification_result(
    proposal: PatchProposal,
    before_step: DiagnosticStep,
    after_step: DiagnosticStep,
    focus_probe: str,
) -> VerificationResult:
    predicted_rule_ids = {
        r.id for r in before_step.evaluated_evidence.rule_results if r.probe == focus_probe and r.status == "fail"
    }
    rule_changes = compare_rule_status(before_step, after_step, focus_probe)
    fact_changes = compare_fact_values(before_step, after_step, focus_probe)
    outcome, explanation, remaining_uncertainty = determine_outcome(rule_changes, predicted_rule_ids)

    return VerificationResult(
        proposal_id=proposal.proposal_id,
        session_id=proposal.session_id,
        before_step_number=before_step.step_number,
        after_step_number=after_step.step_number,
        changes=[*rule_changes, *fact_changes],
        outcome=outcome,
        evidence_refs=proposal.evidence_refs,
        explanation=explanation,
        remaining_uncertainty=remaining_uncertainty,
        created_at_ms=_now_ms(),
    )
