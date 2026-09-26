from app.catalog import CatalogEntry
from app.code_analysis.gemini_schema import CodeInterpretationOut
from app.grounding import index_by_ref_id
from app.schemas import CodeFact, ComponentProposal, ProposedRole


class CodeInterpretationResult:
    def __init__(
        self,
        outcome: str,
        components: list[ComponentProposal],
        roles: list[ProposedRole],
        controller: str | None,
        expected_behavior: str | None,
        unknown_reason: str | None,
        reasoning_notes: list[str],
    ):
        self.outcome = outcome
        self.components = components
        self.roles = roles
        self.controller = controller
        self.expected_behavior = expected_behavior
        self.unknown_reason = unknown_reason
        self.reasoning_notes = reasoning_notes


def _invalid(reason: str) -> CodeInterpretationResult:
    return CodeInterpretationResult(
        outcome="UNKNOWN",
        components=[],
        roles=[],
        controller=None,
        expected_behavior=None,
        unknown_reason="INVALID_PROVIDER_OUTPUT",
        reasoning_notes=[reason],
    )


def validate_and_rank(
    raw: CodeInterpretationOut,
    facts: list[CodeFact],
    catalog: dict[str, CatalogEntry],
) -> CodeInterpretationResult:
    if raw.outcome == "UNKNOWN":
        return CodeInterpretationResult(
            outcome="UNKNOWN",
            components=[],
            roles=[],
            controller=None,
            expected_behavior=None,
            unknown_reason="PROVIDER_UNCERTAIN",
            reasoning_notes=raw.reasoning_notes,
        )

    fact_index = index_by_ref_id(facts)

    components: list[ComponentProposal] = []
    for candidate in raw.component_candidates:
        if candidate.catalog_id not in catalog:
            return _invalid(
                f"Component candidate cited unknown catalog_id '{candidate.catalog_id}'."
            )
        if any(ref not in fact_index for ref in candidate.grounded_in):
            return _invalid("Component candidate cited a ref_id not present in extracted facts.")
        entry = catalog[candidate.catalog_id]
        components.append(
            ComponentProposal(
                catalog_id=entry.id,
                name=entry.name,
                source=f"component-catalog/{entry.id}",
                confidence=candidate.confidence,
                sources=["code_analysis"],
                grounded_in=candidate.grounded_in,
            )
        )

    roles: list[ProposedRole] = []
    for role in raw.role_candidates:
        if any(ref not in fact_index for ref in role.grounded_in):
            return _invalid("Role candidate cited a ref_id not present in extracted facts.")
        roles.append(
            ProposedRole(
                target_pin=role.target_pin,
                role=role.likely_role,
                mode=role.mode,
                is_power_rail=role.is_power_rail,
                expected={},
                confidence=role.confidence,
                sources=["code_analysis"],
                grounded_in=role.grounded_in,
            )
        )

    return CodeInterpretationResult(
        outcome="INTERPRETED",
        components=components,
        roles=roles,
        controller=raw.controller,
        expected_behavior=raw.expected_behavior_summary,
        unknown_reason=None,
        reasoning_notes=raw.reasoning_notes,
    )
