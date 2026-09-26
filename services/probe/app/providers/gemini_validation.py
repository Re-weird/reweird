from app.grounding import index_by_ref_id
from app.providers.base import AIProviderResult
from app.providers.gemini_schema import GeminiResponseOut
from app.schemas import GroundedItem, Hypothesis, StructuredEvidence


def _invalid() -> AIProviderResult:
    return AIProviderResult(
        outcome="UNKNOWN",
        hypotheses=[],
        unknown_reason="INVALID_PROVIDER_OUTPUT",
        reasoning_notes=[
            "Gemini output failed deterministic validation; discarding the entire response."
        ],
    )


def validate_and_rank(
    raw: GeminiResponseOut,
    evidence: StructuredEvidence,
    allowed_grounded: list[GroundedItem],
) -> AIProviderResult:
    if raw.outcome == "UNKNOWN":
        return AIProviderResult(
            outcome="UNKNOWN",
            hypotheses=[],
            unknown_reason="PROVIDER_UNCERTAIN",
            reasoning_notes=raw.reasoning_notes,
        )

    allowed_rule_ids = {rule.id for rule in evidence.rule_results}
    index = index_by_ref_id(allowed_grounded)

    validated: list[Hypothesis] = []
    for h in raw.hypotheses:
        if any(ref not in index for ref in h.grounded_in):
            return _invalid()  # unknown/out-of-scope ref
        if any(rid not in allowed_rule_ids for rid in h.supporting_rule_ids):
            return _invalid()  # rule id not present in this evidence at all
        cited_rule_names = {
            index[ref].name for ref in h.grounded_in if index[ref].category == "rule_result"
        }
        if not set(h.supporting_rule_ids).issubset(cited_rule_names):
            return _invalid()  # a claimed rule has no matching cited rule_result ref
        validated.append(
            Hypothesis(
                rank=0,
                label=h.label,
                explanation=h.explanation,
                confidence=h.confidence,
                grounded_in=h.grounded_in,
                supporting_rule_ids=h.supporting_rule_ids,
            )
        )

    validated.sort(key=lambda hy: -hy.confidence)
    for i, hy in enumerate(validated, start=1):
        hy.rank = i

    return AIProviderResult(outcome="DIAGNOSED", hypotheses=validated, reasoning_notes=raw.reasoning_notes)
