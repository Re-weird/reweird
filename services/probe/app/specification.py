from app.catalog import CatalogSpecEntry, RangeSpec, SpecificationEntry
from app.schemas import EvidenceFact, RuleResult, StructuredEvidence

# Fixed, minimal fact-name vocabulary this evaluator looks for. These names
# mirror the ones apps/api's own Go engine already emits as EvidenceFacts
# (apps/api/internal/diagnostics/engine.go:307-325: "average_voltage",
# "pulse_count", "digital_transitions"), plus one new name ("pulse_width")
# for the one new checkable dimension this milestone adds.
_VOLTAGE_FACT_NAME = "average_voltage"
_PULSE_WIDTH_FACT_NAME = "pulse_width"
_ACTIVITY_FACT_NAMES = {
    "analog": (_VOLTAGE_FACT_NAME,),
    "digital": ("digital_transitions", "pulse_count"),
    "pulse": ("pulse_count", "digital_transitions"),
}


def _not_evaluable(probe: str, message: str) -> RuleResult:
    return RuleResult(
        id="specification-not-evaluable",
        probe=probe,
        status="warn",
        message=message,
        provenance="SPECIFICATION",
    )


def _find_fact(facts: list[EvidenceFact], name: str) -> EvidenceFact | None:
    for fact in facts:
        if fact.name == name:
            return fact
    return None


def _evaluate_range(
    *,
    spec: RangeSpec,
    fact_name: str,
    facts: list[EvidenceFact],
    probe: str,
    role: str,
    catalog_id: str,
    rule_id: str,
    dimension_label: str,
    source: str,
) -> tuple[list[EvidenceFact], list[RuleResult]]:
    fact = _find_fact(facts, fact_name)
    if fact is None:
        return [], [
            _not_evaluable(
                probe,
                f"No '{fact_name}' measurement was present in the submitted evidence; "
                f"{catalog_id}'s {dimension_label} specification for {role} cannot be "
                "evaluated without it.",
            )
        ]

    # Exact-unit matching only. No implicit conversion, no guessing: a
    # missing or mismatched unit fails closed rather than risking a
    # wrong-by-a-factor comparison.
    if fact.unit != spec.unit:
        return [], [
            _not_evaluable(
                probe,
                f"Unit mismatch evaluating {role}'s {dimension_label}: observed "
                f"'{fact.unit or 'no unit'}' does not match {catalog_id}'s specified "
                f"'{spec.unit}'; skipping the comparison rather than guessing.",
            )
        ]

    if not isinstance(fact.value, (int, float)) or isinstance(fact.value, bool):
        return [], [
            _not_evaluable(
                probe,
                f"'{fact_name}' value is not numeric; {dimension_label} specification "
                f"for {role} cannot be evaluated.",
            )
        ]

    value = float(fact.value)
    outside = value < spec.min or value > spec.max
    spec_fact = EvidenceFact(
        probe=probe,
        name=f"{fact_name}_specification_range",
        value=f"{spec.min}-{spec.max}",
        unit=spec.unit,
        provenance="SPECIFICATION",
        detail=source,
    )
    status = "fail" if outside else "pass"
    message = (
        f"{role} {dimension_label} {value}{fact.unit} is "
        f"{'outside' if outside else 'within'} {catalog_id}'s specified "
        f"{spec.min}-{spec.max}{spec.unit} range."
    )
    return [spec_fact], [
        RuleResult(id=rule_id, probe=probe, status=status, message=message, provenance="SPECIFICATION")
    ]


def _evaluate_required_signal(
    spec: SpecificationEntry, facts: list[EvidenceFact], probe: str, role: str, catalog_id: str
) -> list[RuleResult]:
    if not spec.required or spec.mode is None:
        # No activity concept applies (e.g. a continuous power rail, checked
        # by its voltage range instead) - nothing to evaluate here at all,
        # not even a "not evaluable" warning.
        return []

    candidate_names = _ACTIVITY_FACT_NAMES.get(spec.mode, ())
    fact = None
    for name in candidate_names:
        fact = _find_fact(facts, name)
        if fact is not None:
            break

    if fact is None:
        return [
            _not_evaluable(
                probe,
                f"{role} is required by {catalog_id}'s specification, but no "
                f"activity measurement ({', '.join(candidate_names) or 'n/a'}) was "
                "present in the submitted evidence - absence was not established, "
                "only unobserved.",
            )
        ]

    value = fact.value
    is_zero = (
        (isinstance(value, bool) and value is False)
        or (isinstance(value, (int, float)) and not isinstance(value, bool) and abs(value) <= 0.01)
    )
    if is_zero:
        return [
            RuleResult(
                id="missing-signal",
                probe=probe,
                status="fail",
                message=(
                    f"{role} is required by {catalog_id}'s specification, and '{fact.name}' "
                    f"deterministically shows zero activity ({value})."
                ),
                provenance="SPECIFICATION",
            )
        ]
    return []


def evaluate_component_specification(
    entry: CatalogSpecEntry, probe: str, role: str, evidence: StructuredEvidence
) -> tuple[list[EvidenceFact], list[RuleResult]]:
    """Deterministically compare a catalog-derived specification against
    already-observed evidence. Pure function, no I/O, no Gemini call - the
    result is plain data merged into StructuredEvidence before grounding.
    """

    spec = entry.specifications.get(role)
    if spec is None:
        return [], [
            _not_evaluable(
                probe,
                f"{entry.id} has no catalog specification defined for role '{role}'.",
            )
        ]

    observed_facts = [*(evidence.measurements or []), *(evidence.derived_facts or [])]

    facts: list[EvidenceFact] = []
    rules: list[RuleResult] = []

    if spec.voltage is not None:
        f, r = _evaluate_range(
            spec=spec.voltage,
            fact_name=_VOLTAGE_FACT_NAME,
            facts=observed_facts,
            probe=probe,
            role=role,
            catalog_id=entry.id,
            rule_id="voltage-outside-specification",
            dimension_label="voltage",
            source=spec.source,
        )
        facts += f
        rules += r

    if spec.pulse_width is not None:
        f, r = _evaluate_range(
            spec=spec.pulse_width,
            fact_name=_PULSE_WIDTH_FACT_NAME,
            facts=observed_facts,
            probe=probe,
            role=role,
            catalog_id=entry.id,
            rule_id="pulse-width-outside-specification",
            dimension_label="pulse width",
            source=spec.source,
        )
        facts += f
        rules += r

    rules += _evaluate_required_signal(spec, observed_facts, probe, role, entry.id)

    return facts, rules
