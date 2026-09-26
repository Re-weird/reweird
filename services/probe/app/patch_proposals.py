import time
import uuid
from functools import lru_cache
from typing import Protocol

from app.catalog import CatalogSpecEntry
from app.grounding import ground_evidence, index_by_ref_id
from app.patch_provider import PatchProposalProvider
from app.patch_safety import scan_text_fields
from app.planner import TestPlanner
from app.providers.base import AIProvider
from app.schemas import (
    DiagnosticSession,
    DiagnosticStep,
    PatchProposal,
    PatchProposalRequest,
    RecordExternalResultRequest,
    StructuredEvidence,
    UnknownReason,
    VerificationResult,
)
from app.sessions import submit_evidence
from app.verification import build_verification_result


class PatchProposalRejected(Exception):
    """Deterministic proposal creation failure - fails closed. Carries the
    same UnknownReason vocabulary used everywhere else in this service."""

    def __init__(self, unknown_reason: UnknownReason, reasoning_notes: list[str]) -> None:
        self.unknown_reason = unknown_reason
        self.reasoning_notes = reasoning_notes
        super().__init__(reasoning_notes[0] if reasoning_notes else unknown_reason)


class InvalidProposalTransitionError(Exception):
    def __init__(self, current_status: str, requested_status: str) -> None:
        self.current_status = current_status
        self.requested_status = requested_status
        super().__init__(f"Cannot move a '{current_status}' proposal to '{requested_status}'.")


class ProposalAlreadyVerifiedError(Exception):
    def __init__(self, status: str) -> None:
        self.status = status
        super().__init__(f"Proposal is already terminal ('{status}') and cannot be re-verified.")


class ProposalNotExecutedError(Exception):
    def __init__(self, status: str) -> None:
        self.status = status
        super().__init__(f"Proposal must be EXECUTED_EXTERNALLY before verify; current status is '{status}'.")


def _now_ms() -> int:
    return int(time.time() * 1000)


def new_proposal_id() -> str:
    return f"patch_{uuid.uuid4().hex}"


def create_patch_proposal(
    session: DiagnosticSession,
    request: PatchProposalRequest,
    provider: PatchProposalProvider,
) -> PatchProposal:
    """Generate-then-validate, exactly like every AI-touched path in this
    service: `provider.propose()` may draft free text and select evidence,
    but nothing it returns is trusted until re-checked here. Any single
    problem invalidates the entire draft - fails closed, never partially."""
    source_step_number = request.source_step_number or len(session.steps)
    matching_steps = [s for s in session.steps if s.step_number == source_step_number]
    if not matching_steps:
        raise PatchProposalRejected(
            "NO_EVIDENCE",
            [f"Session '{session.session_id}' has no step numbered {source_step_number}."],
        )
    source_step = matching_steps[0]

    try:
        result = provider.propose(source_step, request.target_probe, request.target_role, request.patch_type)
    except Exception as exc:  # noqa: BLE001 - never leak provider internals, mirrors GeminiAIProvider
        raise PatchProposalRejected("PROVIDER_ERROR", ["Patch proposal provider failed."]) from exc

    if result.outcome != "PROPOSED" or result.draft is None:
        raise PatchProposalRejected(
            result.unknown_reason or "PROVIDER_UNCERTAIN",
            result.reasoning_notes or ["Provider did not produce a proposal."],
        )

    draft = result.draft
    grounded = ground_evidence(source_step.evaluated_evidence)
    grounded_index = index_by_ref_id(grounded)

    if any(ref not in grounded_index for ref in draft.evidence_refs):
        raise PatchProposalRejected(
            "INVALID_PROVIDER_OUTPUT",
            ["Proposal cited an evidence ref_id that does not exist in the source step - hallucinated reference."],
        )
    if not any(grounded_index[ref].probe == request.target_probe for ref in draft.evidence_refs):
        raise PatchProposalRejected(
            "INVALID_PROVIDER_OUTPUT",
            [f"None of the cited evidence refs pertain to target_probe '{request.target_probe}'."],
        )

    unsafe_reason = scan_text_fields(draft.purpose, draft.expected_effect)
    if unsafe_reason is not None:
        raise PatchProposalRejected("INVALID_PROVIDER_OUTPUT", [unsafe_reason])

    now = _now_ms()
    return PatchProposal(
        proposal_id=new_proposal_id(),
        session_id=session.session_id,
        source_step_number=source_step.step_number,
        target_probe=request.target_probe,
        target_role=request.target_role,
        patch_type=request.patch_type,
        purpose=draft.purpose,
        expected_effect=draft.expected_effect,
        evidence_refs=draft.evidence_refs,
        safety_requirements=[
            "Temporary emulation only - no persistent firmware or configuration change.",
            "Subject to external Go safety/approval authorization before any execution.",
        ],
        status="PROPOSED",
        created_at_ms=now,
        updated_at_ms=now,
    )


_ALLOWED_TRANSITIONS: dict[str, set[str]] = {
    "PROPOSED": {"APPROVED_EXTERNALLY", "REJECTED"},
    "APPROVED_EXTERNALLY": {"EXECUTED_EXTERNALLY", "REJECTED"},
}


def record_external_result(proposal: PatchProposal, request: RecordExternalResultRequest) -> PatchProposal:
    allowed = _ALLOWED_TRANSITIONS.get(proposal.status, set())
    if request.external_status not in allowed:
        raise InvalidProposalTransitionError(proposal.status, request.external_status)
    return proposal.model_copy(
        update={
            "status": request.external_status,
            "external_reason": request.reason,
            "updated_at_ms": _now_ms(),
        }
    )


class PatchProposalStore(Protocol):
    def get(self, proposal_id: str) -> PatchProposal | None: ...

    def save(self, proposal: PatchProposal) -> None: ...

    def list_for_session(self, session_id: str) -> list[PatchProposal]: ...


class InMemoryPatchProposalStore:
    def __init__(self) -> None:
        self._proposals: dict[str, PatchProposal] = {}

    def get(self, proposal_id: str) -> PatchProposal | None:
        return self._proposals.get(proposal_id)

    def save(self, proposal: PatchProposal) -> None:
        self._proposals[proposal.proposal_id] = proposal

    def list_for_session(self, session_id: str) -> list[PatchProposal]:
        return sorted(
            (p for p in self._proposals.values() if p.session_id == session_id),
            key=lambda p: p.created_at_ms,
        )


@lru_cache
def get_default_patch_proposal_store() -> InMemoryPatchProposalStore:
    return InMemoryPatchProposalStore()


_TERMINAL_VERIFICATION_STATUSES = {"VERIFIED", "FAILED_VERIFICATION"}

# Verification outcome -> new proposal status. INCONCLUSIVE is deliberately
# NOT terminal: it means "try again with the right measurement," not "this
# proposal is resolved" - the proposal stays EXECUTED_EXTERNALLY so a real
# retry with better evidence is still possible.
_OUTCOME_TO_STATUS = {"SUPPORTED": "VERIFIED", "NOT_SUPPORTED": "FAILED_VERIFICATION"}


def verify_proposal(
    session: DiagnosticSession,
    proposal: PatchProposal,
    after_evidence: StructuredEvidence,
    catalog: dict[str, CatalogSpecEntry],
    provider: AIProvider,
    planner: TestPlanner,
) -> tuple[DiagnosticSession, PatchProposal, VerificationResult | None, DiagnosticStep | None]:
    """Returns (updated_session, updated_proposal, verification_or_None,
    duplicate_step_or_None). A non-None duplicate_step means the after
    evidence was an exact duplicate of an earlier step - no new step was
    appended, no verification was computed, and the proposal is returned
    unchanged (still EXECUTED_EXTERNALLY, so a genuine retry remains
    possible)."""
    if proposal.session_id != session.session_id:
        raise PatchProposalRejected(
            "INVALID_PROVIDER_OUTPUT", ["Proposal does not belong to this session."]
        )
    if proposal.status in _TERMINAL_VERIFICATION_STATUSES:
        raise ProposalAlreadyVerifiedError(proposal.status)
    if proposal.status != "EXECUTED_EXTERNALLY":
        raise ProposalNotExecutedError(proposal.status)

    # SessionStoppedError propagates to the caller unchanged - reusing
    # Milestone 5's own stopped-session rejection rather than reimplementing
    # it here.
    updated_session, duplicate_step = submit_evidence(session, after_evidence, catalog, provider, planner)
    if duplicate_step is not None:
        return updated_session, proposal, None, duplicate_step

    before_matches = [s for s in updated_session.steps if s.step_number == proposal.source_step_number]
    before_step = before_matches[0]
    after_step = updated_session.steps[-1]

    verification = build_verification_result(proposal, before_step, after_step, session.probe)

    new_status = _OUTCOME_TO_STATUS.get(verification.outcome, proposal.status)
    updated_proposal = proposal.model_copy(
        update={"status": new_status, "verification": verification, "updated_at_ms": _now_ms()}
    )
    return updated_session, updated_proposal, verification, None
