from typing import Protocol

from pydantic import BaseModel, Field

from app.grounding import ground_evidence
from app.schemas import DiagnosticStep, PatchType, UnknownReason


class PatchProposalDraft(BaseModel):
    """What a provider proposes BEFORE deterministic validation. Never
    stored or returned as-is - app.patch_proposals.create_patch_proposal
    re-validates every field before it becomes a real PatchProposal."""

    purpose: str = Field(min_length=1, max_length=500)
    expected_effect: str = Field(min_length=1, max_length=500)
    evidence_refs: list[str] = Field(min_length=1)
    reasoning_notes: list[str] = Field(default_factory=list)


class PatchProposalProviderResult(BaseModel):
    outcome: str  # "PROPOSED" | "UNKNOWN"
    draft: PatchProposalDraft | None = None
    unknown_reason: UnknownReason | None = None
    reasoning_notes: list[str] = Field(default_factory=list)


class PatchProposalProvider(Protocol):
    def propose(
        self,
        source_step: DiagnosticStep,
        target_probe: str,
        target_role: str,
        patch_type: PatchType,
    ) -> PatchProposalProviderResult: ...


class FakePatchProposalProvider:
    """Deterministic, network-free default. "Explains/selects" a proposal
    the same way FakeAIProvider stands in for Gemini elsewhere in this
    service: no randomness, no network, fully reproducible from the given
    diagnostic step's own grounded evidence.

    Only drafts free text (purpose/expected_effect) and picks which already-
    grounded evidence justifies the patch - it never invents a target_probe/
    target_role/patch_type; those are supplied by the caller and validated
    deterministically regardless of which provider drafted the rest.
    """

    def propose(
        self,
        source_step: DiagnosticStep,
        target_probe: str,
        target_role: str,
        patch_type: PatchType,
    ) -> PatchProposalProviderResult:
        grounded = ground_evidence(source_step.evaluated_evidence)
        failing_refs = [
            item.ref_id
            for item in grounded
            if item.category == "rule_result" and item.probe == target_probe and item.status == "fail"
        ]
        if not failing_refs:
            return PatchProposalProviderResult(
                outcome="UNKNOWN",
                unknown_reason="NO_EVIDENCE",
                reasoning_notes=[
                    f"No failing rule evidence for probe '{target_probe}' exists in step "
                    f"{source_step.step_number}; a patch proposal must be grounded in a real "
                    "deterministic finding, not drafted speculatively."
                ],
            )

        draft = PatchProposalDraft(
            purpose=(
                f"{target_role} ({target_probe}) shows failing deterministic evidence; "
                f"temporarily emulate a valid signal to test whether it is the upstream cause."
            ),
            expected_effect=(
                f"If {target_role} is the upstream cause, the session's own focus signal "
                "should show restored expected activity after the patch; if not, the fault "
                "lies elsewhere."
            ),
            evidence_refs=failing_refs,
        )
        return PatchProposalProviderResult(outcome="PROPOSED", draft=draft)
