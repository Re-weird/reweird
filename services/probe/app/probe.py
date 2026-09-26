from app.catalog import CatalogSpecEntry
from app.grounding import ground_evidence
from app.planner import TestPlanner
from app.providers.base import AIProvider
from app.schemas import ProbeResponse, StructuredEvidence
from app.specification import evaluate_component_specification


class UnknownComponentError(Exception):
    """Raised when a caller-supplied component_id does not exist in the
    loaded catalog. Callers (run_probe, and Milestone 5's session pipeline)
    both translate this into the same deterministic UNKNOWN_COMPONENT
    response - never a 500, never a silent no-op."""

    def __init__(self, component_id: str) -> None:
        self.component_id = component_id
        super().__init__(f"Unknown component id: {component_id}")


def merge_component_specification(
    evidence: StructuredEvidence,
    catalog: dict[str, CatalogSpecEntry] | None,
    component_id: str | None,
) -> StructuredEvidence:
    """Deterministically merge catalog-derived SPECIFICATION facts/rules
    into evidence for component_id (Milestone 4's evaluator, unchanged).

    A no-op when component_id is None - this is the exact Milestone 1-3
    behavior, preserved byte-for-byte. Raises UnknownComponentError rather
    than returning a sentinel, so every caller is forced to handle the
    unknown-component case explicitly instead of accidentally treating a
    typo'd id as "no component".
    """
    if component_id is None:
        return evidence
    entry = (catalog or {}).get(component_id)
    if entry is None:
        raise UnknownComponentError(component_id)
    spec_facts, spec_rules = evaluate_component_specification(
        entry, evidence.probe, evidence.role, evidence
    )
    return evidence.model_copy(
        update={
            "specification_results": [*(evidence.specification_results or []), *spec_facts],
            "rule_results": [*evidence.rule_results, *spec_rules],
        }
    )


def unknown_component_response(evidence: StructuredEvidence, component_id: str) -> ProbeResponse:
    return ProbeResponse(
        probe=evidence.probe,
        role=evidence.role,
        outcome="UNKNOWN",
        hypotheses=[],
        recommended_test=(
            f"Confirm the catalog component id '{component_id}' is correct; it was not "
            "found in the component catalog."
        ),
        unresolved_questions=list(evidence.unresolved_questions or []),
        grounded_evidence_count=0,
        unknown_reason="UNKNOWN_COMPONENT",
        reasoning_notes=[f"Component id '{component_id}' does not exist in the loaded catalog."],
    )


def run_probe(
    evidence: StructuredEvidence,
    provider: AIProvider,
    planner: TestPlanner,
    catalog: dict[str, CatalogSpecEntry] | None = None,
    component_id: str | None = None,
) -> ProbeResponse:
    # Backward compatible by construction: every Milestone 1/2 call site
    # passes only (evidence, provider, planner); catalog/component_id default
    # to None, and merge_component_specification() is then a no-op.
    if component_id is not None:
        try:
            evidence = merge_component_specification(evidence, catalog, component_id)
        except UnknownComponentError:
            return unknown_component_response(evidence, component_id)

    grounded = ground_evidence(evidence)
    result = provider.interpret(evidence, grounded)
    recommended_test = planner.recommend(evidence, result)
    return ProbeResponse(
        probe=evidence.probe,
        role=evidence.role,
        outcome=result.outcome,
        hypotheses=result.hypotheses,
        recommended_test=recommended_test,
        unresolved_questions=list(evidence.unresolved_questions or []),
        grounded_evidence_count=len(grounded),
        unknown_reason=result.unknown_reason,
        reasoning_notes=result.reasoning_notes,
    )
