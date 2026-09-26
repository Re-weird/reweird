from app.catalog import CatalogEntry
from app.code_analysis.gemini_validation import CodeInterpretationResult
from app.schemas import (
    CodeAnalysisResult,
    ComponentProposal,
    ConflictNote,
    ProjectProfileProposal,
    ProposedProject,
    VisionAnalysisResult,
)


def _catalog_ids(components: list[ComponentProposal]) -> set[str]:
    return {c.catalog_id for c in components}


def assemble_proposal(
    code_analysis: CodeAnalysisResult,
    code_interpretation: CodeInterpretationResult,
    vision_analysis: VisionAnalysisResult | None,
    catalog: dict[str, CatalogEntry],
) -> ProjectProfileProposal:
    reasoning_notes: list[str] = list(code_interpretation.reasoning_notes)
    unresolved_questions: list[str] = []

    code_ok = code_interpretation.outcome == "INTERPRETED"
    vision_ok = vision_analysis is not None and vision_analysis.outcome == "INTERPRETED"

    if not code_ok and not vision_ok:
        # Nothing usable from either modality: insufficient information,
        # never a confident (or even tentative) proposal.
        if vision_analysis is not None:
            reasoning_notes.extend(vision_analysis.reasoning_notes)
        return ProjectProfileProposal(
            status="INSUFFICIENT_INFORMATION",
            project=None,
            unresolved_questions=unresolved_questions,
            code_analysis=code_analysis,
            vision_analysis=vision_analysis,
            reasoning_notes=reasoning_notes,
        )

    code_components = code_interpretation.components if code_ok else []
    vision_components = (
        vision_analysis.component_candidates if vision_ok and vision_analysis else []
    )

    conflicts: list[ConflictNote] = []
    status = "PROPOSED"

    if code_ok and vision_ok:
        code_ids = _catalog_ids(code_components)
        vision_ids = _catalog_ids(vision_components)
        if code_ids and vision_ids and code_ids.isdisjoint(vision_ids):
            status = "PROPOSED_WITH_CONFLICTS"
            conflicts.append(
                ConflictNote(
                    area="component identification",
                    code_catalog_id=next(iter(code_ids)),
                    vision_catalog_id=next(iter(vision_ids)),
                    detail=(
                        "Code analysis and vision analysis identified different, "
                        "non-overlapping catalog components for this project. A "
                        "human must resolve which (if either) is correct."
                    ),
                )
            )
            unresolved_questions.append(
                "Code analysis and image analysis disagree on the likely component "
                "- please confirm which is correct."
            )

    if vision_analysis is not None:
        reasoning_notes.extend(vision_analysis.reasoning_notes)

    # Merge components: keep every distinct catalog_id, unioning `sources`
    # when both modalities independently agreed (never averaging away the
    # disagreement when they didn't - that's exactly the conflict case above).
    merged: dict[str, ComponentProposal] = {}
    for component in code_components + vision_components:
        existing = merged.get(component.catalog_id)
        if existing is None:
            merged[component.catalog_id] = component
        else:
            combined_sources = list(dict.fromkeys(existing.sources + component.sources))
            merged[component.catalog_id] = existing.model_copy(
                update={
                    "sources": combined_sources,
                    "confidence": max(existing.confidence, component.confidence),
                    "grounded_in": list(dict.fromkeys(existing.grounded_in + component.grounded_in)),
                }
            )

    project = ProposedProject(
        controller=code_interpretation.controller if code_ok else None,
        expected_behavior=code_interpretation.expected_behavior if code_ok else None,
        components=list(merged.values()),
        roles=code_interpretation.roles if code_ok else [],
    )

    return ProjectProfileProposal(
        status=status,  # type: ignore[arg-type]
        project=project,
        conflicts=conflicts,
        unresolved_questions=unresolved_questions,
        code_analysis=code_analysis,
        vision_analysis=vision_analysis,
        reasoning_notes=reasoning_notes,
    )
