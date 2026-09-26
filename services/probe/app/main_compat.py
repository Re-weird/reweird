from app.catalog import CatalogSpecEntry
from app.planner import TestPlanner
from app.probe import run_probe
from app.providers.base import AIProvider
from app.schemas import MainDiagnosis, StructuredEvidence


def diagnose_for_main(
    evidence: StructuredEvidence,
    deterministic: MainDiagnosis,
    provider: AIProvider,
    planner: TestPlanner,
    catalog: dict[str, CatalogSpecEntry] | None = None,
) -> MainDiagnosis:
    """Map grounded PROBE output onto main's existing Diagnosis contract.

    Main's deterministic diagnosis is the fail-closed result. UNKNOWN, empty
    hypotheses, provider failure, or invalid provider output therefore cannot
    erase a known deterministic finding or fabricate a replacement. A
    diagnosed result may only replace narrative fields, and its confidence is
    capped at main's deterministic confidence.
    """

    result = run_probe(evidence, provider, planner, catalog)
    if result.outcome == "UNKNOWN" or not result.hypotheses:
        return deterministic.model_copy(deep=True)

    primary = result.hypotheses[0]
    failing_rule_ids = {rule.id for rule in evidence.rule_results if rule.status == "fail"}
    causes = [
        hypothesis.label
        for hypothesis in result.hypotheses
        if failing_rule_ids.intersection(hypothesis.supporting_rule_ids)
    ]
    return MainDiagnosis(
        headline=primary.label,
        summary=primary.explanation,
        possible_causes=causes,
        confidence=min(primary.confidence, deterministic.confidence),
        next_test=result.recommended_test,
    )
