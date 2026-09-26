from app.catalog import CatalogEntry
from app.schemas import ComponentProposal, VisionAnalysisResult
from app.vision.gemini_schema import VisionInterpretationOut


def _invalid(reason: str) -> VisionAnalysisResult:
    return VisionAnalysisResult(
        outcome="UNKNOWN", unknown_reason="INVALID_PROVIDER_OUTPUT", reasoning_notes=[reason]
    )


def validate_vision_output(
    raw: VisionInterpretationOut, catalog: dict[str, CatalogEntry]
) -> VisionAnalysisResult:
    if raw.outcome == "UNKNOWN":
        return VisionAnalysisResult(
            outcome="UNKNOWN",
            unknown_reason="PROVIDER_UNCERTAIN",
            reasoning_notes=raw.reasoning_notes,
        )

    components: list[ComponentProposal] = []
    for candidate in raw.component_candidates:
        if candidate.catalog_id not in catalog:
            return _invalid(f"Vision candidate cited unknown catalog_id '{candidate.catalog_id}'.")
        entry = catalog[candidate.catalog_id]
        components.append(
            ComponentProposal(
                catalog_id=entry.id,
                name=entry.name,
                source=f"component-catalog/{entry.id}",
                confidence=candidate.confidence,
                sources=["vision"],
                grounded_in=[],
            )
        )

    return VisionAnalysisResult(
        outcome="INTERPRETED",
        component_candidates=components,
        reasoning_notes=raw.reasoning_notes,
    )
