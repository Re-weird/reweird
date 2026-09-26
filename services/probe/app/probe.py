from app.grounding import ground_evidence
from app.planner import TestPlanner
from app.providers.base import AIProvider
from app.schemas import ProbeResponse, StructuredEvidence


def run_probe(
    evidence: StructuredEvidence, provider: AIProvider, planner: TestPlanner
) -> ProbeResponse:
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
