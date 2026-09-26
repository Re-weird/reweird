from contextlib import asynccontextmanager
from functools import lru_cache

from fastapi import Depends, FastAPI, HTTPException

from app.catalog import CatalogSpecEntry, get_default_catalog
from app.config import get_settings
from app.planner import TestPlanner
from app.probe import run_probe
from app.providers.base import AIProvider
from app.providers.fake import FakeAIProvider
from app.providers.gemini import GeminiAIProvider
from app.patch_provider import FakePatchProposalProvider, PatchProposalProvider
from app.patch_proposals import (
    InMemoryPatchProposalStore,
    InvalidProposalTransitionError,
    PatchProposalRejected,
    ProposalAlreadyVerifiedError,
    ProposalNotExecutedError,
    create_patch_proposal,
    get_default_patch_proposal_store,
    record_external_result,
    verify_proposal,
)
from app.schemas import (
    CreateSessionRequest,
    DiagnosticSession,
    PatchProposal,
    PatchProposalRequest,
    PatchProposalResponse,
    ProbeRequest,
    ProbeResponse,
    RecordExternalResultRequest,
    SubmitEvidenceRequest,
    SubmitEvidenceResponse,
    VerifyRequest,
    VerifyResponse,
)
from app.sessions import (
    InMemorySessionStore,
    SessionStoppedError,
    create_session,
    get_default_session_store,
    stop_session,
    submit_evidence,
)


@lru_cache
def get_provider() -> AIProvider:
    settings = get_settings()
    if settings.ai_provider == "gemini":
        return GeminiAIProvider(settings)
    return FakeAIProvider()


@asynccontextmanager
async def lifespan(app: FastAPI):
    get_provider()  # forces construction + config validation at startup, not on first request
    get_default_catalog()
    get_patch_proposal_provider()
    yield


app = FastAPI(title="reweird-probe", version="0.1.0", lifespan=lifespan)


def get_planner() -> TestPlanner:
    return TestPlanner()


def get_catalog() -> dict[str, CatalogSpecEntry]:
    return get_default_catalog()


def get_session_store() -> InMemorySessionStore:
    return get_default_session_store()


def get_patch_proposal_store() -> InMemoryPatchProposalStore:
    return get_default_patch_proposal_store()


@lru_cache
def get_patch_proposal_provider() -> PatchProposalProvider:
    # Milestone 6 is intentionally narrow: only a deterministic, network-free
    # provider is wired in by default. See app/patch_provider.py's docstring
    # for why a full Gemini-backed variant is not built here.
    return FakePatchProposalProvider()


@app.post("/probe", response_model=ProbeResponse)
def post_probe(
    request: ProbeRequest,
    provider: AIProvider = Depends(get_provider),
    planner: TestPlanner = Depends(get_planner),
    catalog: dict[str, CatalogSpecEntry] = Depends(get_catalog),
) -> ProbeResponse:
    return run_probe(request.evidence, provider, planner, catalog, request.component_id)


def _get_session_or_404(store: InMemorySessionStore, session_id: str) -> DiagnosticSession:
    session = store.get(session_id)
    if session is None:
        raise HTTPException(status_code=404, detail=f"Session '{session_id}' does not exist.")
    return session


@app.post("/sessions", response_model=DiagnosticSession)
def post_create_session(
    request: CreateSessionRequest,
    provider: AIProvider = Depends(get_provider),
    planner: TestPlanner = Depends(get_planner),
    catalog: dict[str, CatalogSpecEntry] = Depends(get_catalog),
    store: InMemorySessionStore = Depends(get_session_store),
) -> DiagnosticSession:
    session = create_session(
        request.evidence, request.component_id, catalog, provider, planner, request.max_steps
    )
    store.save(session)
    return session


@app.get("/sessions/{session_id}", response_model=DiagnosticSession)
def get_session(
    session_id: str, store: InMemorySessionStore = Depends(get_session_store)
) -> DiagnosticSession:
    return _get_session_or_404(store, session_id)


@app.post("/sessions/{session_id}/evidence", response_model=SubmitEvidenceResponse)
def post_submit_evidence(
    session_id: str,
    request: SubmitEvidenceRequest,
    provider: AIProvider = Depends(get_provider),
    planner: TestPlanner = Depends(get_planner),
    catalog: dict[str, CatalogSpecEntry] = Depends(get_catalog),
    store: InMemorySessionStore = Depends(get_session_store),
) -> SubmitEvidenceResponse:
    session = _get_session_or_404(store, session_id)
    try:
        updated_session, duplicate_step = submit_evidence(
            session, request.evidence, catalog, provider, planner
        )
    except SessionStoppedError:
        raise HTTPException(
            status_code=409,
            detail=f"Session '{session_id}' is STOPPED and cannot accept new evidence.",
        )

    store.save(updated_session)
    return SubmitEvidenceResponse(
        session=updated_session,
        duplicate=duplicate_step is not None,
        duplicate_of_step=duplicate_step.step_number if duplicate_step else None,
    )


@app.post("/sessions/{session_id}/stop", response_model=DiagnosticSession)
def post_stop_session(
    session_id: str, store: InMemorySessionStore = Depends(get_session_store)
) -> DiagnosticSession:
    session = _get_session_or_404(store, session_id)
    stopped = stop_session(session)
    store.save(stopped)
    return stopped


# ---------------------------------------------------------------------------
# Milestone 6: PATCH proposal + VERIFY intelligence.
#
# Deliberately named /patch-proposals, never /patch: nothing under this
# prefix executes anything. Go's own PATCH_LOCKED gate
# (apps/api/internal/httpapi/server.go) is the sole, unmodified authority
# for actual hardware execution.
# ---------------------------------------------------------------------------


def _get_proposal_or_404(store: InMemoryPatchProposalStore, proposal_id: str) -> PatchProposal:
    proposal = store.get(proposal_id)
    if proposal is None:
        raise HTTPException(status_code=404, detail=f"Patch proposal '{proposal_id}' does not exist.")
    return proposal


@app.post("/sessions/{session_id}/patch-proposals", response_model=PatchProposalResponse)
def post_create_patch_proposal(
    session_id: str,
    request: PatchProposalRequest,
    provider: PatchProposalProvider = Depends(get_patch_proposal_provider),
    session_store: InMemorySessionStore = Depends(get_session_store),
    proposal_store: InMemoryPatchProposalStore = Depends(get_patch_proposal_store),
) -> PatchProposalResponse:
    session = _get_session_or_404(session_store, session_id)
    try:
        proposal = create_patch_proposal(session, request, provider)
    except PatchProposalRejected as exc:
        raise HTTPException(
            status_code=422,
            detail={"unknown_reason": exc.unknown_reason, "reasoning_notes": exc.reasoning_notes},
        )
    proposal_store.save(proposal)
    return PatchProposalResponse(proposal=proposal)


@app.get("/sessions/{session_id}/patch-proposals", response_model=list[PatchProposal])
def list_patch_proposals(
    session_id: str,
    session_store: InMemorySessionStore = Depends(get_session_store),
    proposal_store: InMemoryPatchProposalStore = Depends(get_patch_proposal_store),
) -> list[PatchProposal]:
    _get_session_or_404(session_store, session_id)
    return proposal_store.list_for_session(session_id)


def _get_proposal_for_session_or_404(
    session_store: InMemorySessionStore,
    proposal_store: InMemoryPatchProposalStore,
    session_id: str,
    proposal_id: str,
) -> tuple[DiagnosticSession, PatchProposal]:
    session = _get_session_or_404(session_store, session_id)
    proposal = _get_proposal_or_404(proposal_store, proposal_id)
    if proposal.session_id != session_id:
        raise HTTPException(
            status_code=404,
            detail=f"Patch proposal '{proposal_id}' does not belong to session '{session_id}'.",
        )
    return session, proposal


@app.get("/sessions/{session_id}/patch-proposals/{proposal_id}", response_model=PatchProposal)
def get_patch_proposal(
    session_id: str,
    proposal_id: str,
    session_store: InMemorySessionStore = Depends(get_session_store),
    proposal_store: InMemoryPatchProposalStore = Depends(get_patch_proposal_store),
) -> PatchProposal:
    _, proposal = _get_proposal_for_session_or_404(session_store, proposal_store, session_id, proposal_id)
    return proposal


@app.post(
    "/sessions/{session_id}/patch-proposals/{proposal_id}/result", response_model=PatchProposal
)
def post_record_external_result(
    session_id: str,
    proposal_id: str,
    request: RecordExternalResultRequest,
    session_store: InMemorySessionStore = Depends(get_session_store),
    proposal_store: InMemoryPatchProposalStore = Depends(get_patch_proposal_store),
) -> PatchProposal:
    _, proposal = _get_proposal_for_session_or_404(session_store, proposal_store, session_id, proposal_id)
    try:
        updated = record_external_result(proposal, request)
    except InvalidProposalTransitionError as exc:
        raise HTTPException(status_code=409, detail=str(exc))
    proposal_store.save(updated)
    return updated


@app.post(
    "/sessions/{session_id}/patch-proposals/{proposal_id}/verify", response_model=VerifyResponse
)
def post_verify_patch_proposal(
    session_id: str,
    proposal_id: str,
    request: VerifyRequest,
    provider: AIProvider = Depends(get_provider),
    planner: TestPlanner = Depends(get_planner),
    catalog: dict[str, CatalogSpecEntry] = Depends(get_catalog),
    session_store: InMemorySessionStore = Depends(get_session_store),
    proposal_store: InMemoryPatchProposalStore = Depends(get_patch_proposal_store),
) -> VerifyResponse:
    session, proposal = _get_proposal_for_session_or_404(
        session_store, proposal_store, session_id, proposal_id
    )
    try:
        updated_session, updated_proposal, verification, duplicate_step = verify_proposal(
            session, proposal, request.evidence, catalog, provider, planner
        )
    except SessionStoppedError:
        raise HTTPException(
            status_code=409,
            detail=f"Session '{session_id}' is STOPPED and cannot accept new evidence.",
        )
    except ProposalNotExecutedError as exc:
        raise HTTPException(status_code=409, detail=str(exc))
    except ProposalAlreadyVerifiedError as exc:
        raise HTTPException(status_code=409, detail=str(exc))

    session_store.save(updated_session)
    proposal_store.save(updated_proposal)
    return VerifyResponse(
        session=updated_session,
        proposal=updated_proposal,
        verification=verification,
        duplicate=duplicate_step is not None,
        duplicate_of_step=duplicate_step.step_number if duplicate_step else None,
    )
