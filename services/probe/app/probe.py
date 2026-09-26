from app.catalog import CatalogSpecEntry
from app.grounding import ground_evidence
from app.planner import TestPlanner
from app.providers.base import AIProvider
from app.schemas import ProbeResponse, StructuredEvidence
from app.specification import evaluate_component_specification


def _unknown_component_response(evidence: StructuredEvidence, component_id: str) -> ProbeResponse:
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
    # to None, and the block below is a no-op unless a caller opts in.
    if component_id is not None:
        entry = (catalog or {}).get(component_id)
        if entry is None:
            return _unknown_component_response(evidence, component_id)

        spec_facts, spec_rules = evaluate_component_specification(
            entry, evidence.probe, evidence.role, evidence
        )
        evidence = evidence.model_copy(
            update={
                "specification_results": [*(evidence.specification_results or []), *spec_facts],
                "rule_results": [*evidence.rule_results, *spec_rules],
            }
        )

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
