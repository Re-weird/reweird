import logging
import os
import re
import secrets
from contextlib import asynccontextmanager
from functools import lru_cache

from fastapi import Depends, FastAPI, HTTPException, Request

from app.analytics_sink import (
    AnalyticsSink,
    NullAnalyticsSink,
    SnowflakeAnalyticsSink,
    build_session_summary,
)
from app.catalog import CatalogSpecEntry, get_default_catalog
from app.config import get_settings
from app.events import new_event
from app.export import build_diagnostic_export
from app.main_compat import diagnose_for_main
from app.patch_provider import FakePatchProposalProvider, PatchProposalProvider
from app.patch_proposals import (
    InvalidProposalTransitionError,
    PatchProposalRejected,
    ProposalAlreadyVerifiedError,
    ProposalNotExecutedError,
    create_patch_proposal,
    record_external_result,
    verify_proposal,
)
from app.planner import TestPlanner
from app.probe import run_probe
from app.providers.base import AIProvider
from app.providers.fake import FakeAIProvider
from app.providers.gemini import GeminiAIProvider
from app.repository import DiagnosticRepository, RepositoryUnavailableError, get_repository
from app.schemas import (
    CreateSessionRequest,
    DiagnosticSession,
    MainDiagnosis,
    MainProbeRequest,
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
from app.sessions import SessionStoppedError, create_session, stop_session, submit_evidence
from app.telemetry_sink import NullTelemetrySink, TelemetrySink, TigerTelemetrySink, extract_telemetry_records

logger = logging.getLogger("app.api")


@lru_cache
def get_provider() -> AIProvider:
    settings = get_settings()
    if settings.ai_provider == "gemini":
        return GeminiAIProvider(settings)
    return FakeAIProvider()


@lru_cache
def get_patch_proposal_provider() -> PatchProposalProvider:
    # Milestone 6 is intentionally narrow: only a deterministic, network-free
    # provider is wired in by default. See app/patch_provider.py's docstring
    # for why a full Gemini-backed variant is not built here.
    return FakePatchProposalProvider()


@lru_cache
def get_telemetry_sink() -> TelemetrySink:
    settings = get_settings()
    if settings.telemetry_sink == "tiger":
        if not settings.tiger_database_url:
            raise RuntimeError("TIGER_DATABASE_URL is required when TELEMETRY_SINK=tiger")
        return TigerTelemetrySink(settings.tiger_database_url)
    return NullTelemetrySink()


@lru_cache
def get_analytics_sink() -> AnalyticsSink:
    settings = get_settings()
    if settings.analytics_sink == "snowflake":
        missing = [
            name
            for name, value in (
                ("SNOWFLAKE_ACCOUNT", settings.snowflake_account),
                ("SNOWFLAKE_USER", settings.snowflake_user),
                ("SNOWFLAKE_PASSWORD", settings.snowflake_password),
                ("SNOWFLAKE_DATABASE", settings.snowflake_database),
                ("SNOWFLAKE_WAREHOUSE", settings.snowflake_warehouse),
            )
            if not value
        ]
        if missing:
            raise RuntimeError(f"ANALYTICS_SINK=snowflake requires {', '.join(missing)}")
        return SnowflakeAnalyticsSink(
            account=settings.snowflake_account,
            user=settings.snowflake_user,
            password=settings.snowflake_password,
            database=settings.snowflake_database,
            warehouse=settings.snowflake_warehouse,
        )
    return NullAnalyticsSink()


@asynccontextmanager
async def lifespan(app: FastAPI):
    get_provider()  # forces construction + config validation at startup, not on first request
    get_default_catalog()
    get_patch_proposal_provider()
    get_repository()  # DIAGNOSTIC_REPOSITORY=mongodb with no MONGODB_URI fails fast here
    get_telemetry_sink()  # TELEMETRY_SINK=tiger with no TIGER_DATABASE_URL fails fast here
    get_analytics_sink()  # ANALYTICS_SINK=snowflake with missing config/package fails fast here
    yield


app = FastAPI(title="reweird-probe", version="0.1.0", lifespan=lifespan)


def get_planner() -> TestPlanner:
    return TestPlanner()


def get_catalog() -> dict[str, CatalogSpecEntry]:
    return get_default_catalog()


@app.get("/health")
def get_health() -> dict:
    settings = get_settings()
    return {
        "status": "ok",
        "ai_provider": settings.ai_provider,
        "diagnostic_repository": settings.diagnostic_repository,
        "telemetry_sink": settings.telemetry_sink,
        "analytics_sink": settings.analytics_sink,
    }


@app.post("/probe", response_model=ProbeResponse)
def post_probe(
    request: ProbeRequest,
    provider: AIProvider = Depends(get_provider),
    planner: TestPlanner = Depends(get_planner),
    catalog: dict[str, CatalogSpecEntry] = Depends(get_catalog),
) -> ProbeResponse:
    return run_probe(request.evidence, provider, planner, catalog, request.component_id)


@app.post("/probe/main-diagnosis", response_model=MainDiagnosis)
def post_main_diagnosis(
    request: MainProbeRequest,
    provider: AIProvider = Depends(get_provider),
    planner: TestPlanner = Depends(get_planner),
    catalog: dict[str, CatalogSpecEntry] = Depends(get_catalog),
) -> MainDiagnosis:
    """Consume main's StructuredEvidence and return main's Diagnosis shape.

    The detailed grounded references remain available from POST /probe. This
    compatibility endpoint only performs the final, loss-aware mapping needed
    by the current Go API contract.
    """

    return diagnose_for_main(
        request.evidence,
        request.deterministic_diagnosis,
        provider,
        planner,
        catalog,
    )


def _get_session_or_404(repository: DiagnosticRepository, session_id: str) -> DiagnosticSession:
    try:
        session = repository.get_session(session_id)
    except RepositoryUnavailableError as exc:
        raise HTTPException(status_code=503, detail=str(exc))
    if session is None:
        raise HTTPException(status_code=404, detail=f"Session '{session_id}' does not exist.")
    return session


def _save_session(repository: DiagnosticRepository, session: DiagnosticSession) -> None:
    try:
        repository.save_session(session)
    except RepositoryUnavailableError as exc:
        # PRIMARY repository failure is surfaced clearly - the caller asked
        # for persistence and it did not happen, unlike an optional
        # telemetry/analytics sink failure below.
        raise HTTPException(status_code=503, detail=str(exc))


def _record_telemetry_best_effort(
    sink: TelemetrySink, session_id: str, step
) -> None:
    """Best-effort only: telemetry is optional history. A sink failure is
    logged and otherwise ignored - it must never alter or block the actual
    diagnostic response."""
    try:
        for record in extract_telemetry_records(session_id, step.evaluated_evidence, step.created_at_ms):
            sink.record_measurement(record)
    except Exception:  # noqa: BLE001
        logger.warning("telemetry_sink_write_failed", exc_info=True)


def _record_analytics_best_effort(
    sink: AnalyticsSink, repository: DiagnosticRepository, session: DiagnosticSession
) -> None:
    try:
        proposals = repository.list_patch_proposals(session.session_id)
        summary = build_session_summary(session, proposals)
        sink.record_session_summary(summary)
    except Exception:  # noqa: BLE001
        logger.warning("analytics_sink_write_failed", exc_info=True)


@app.post("/sessions", response_model=DiagnosticSession)
def post_create_session(
    request: CreateSessionRequest,
    provider: AIProvider = Depends(get_provider),
    planner: TestPlanner = Depends(get_planner),
    catalog: dict[str, CatalogSpecEntry] = Depends(get_catalog),
    repository: DiagnosticRepository = Depends(get_repository),
    telemetry: TelemetrySink = Depends(get_telemetry_sink),
    analytics: AnalyticsSink = Depends(get_analytics_sink),
) -> DiagnosticSession:
    session = create_session(
        request.evidence, request.component_id, catalog, provider, planner, request.max_steps
    )
    _save_session(repository, session)
    repository.append_event(
        new_event(
            session.session_id,
            "SESSION_CREATED",
            {
                "step_number": 1,
                "outcome": session.steps[0].result.outcome,
                "recommended_test": session.steps[0].result.recommended_test,
            },
        )
    )
    if session.status in ("DIAGNOSED", "STOPPED"):
        repository.append_event(new_event(session.session_id, "SESSION_COMPLETED", {"status": session.status}))
    _record_telemetry_best_effort(telemetry, session.session_id, session.steps[0])
    _record_analytics_best_effort(analytics, repository, session)
    return session


@app.get("/sessions", response_model=list[DiagnosticSession])
def list_sessions(repository: DiagnosticRepository = Depends(get_repository)) -> list[DiagnosticSession]:
    try:
        return repository.list_sessions()
    except RepositoryUnavailableError as exc:
        raise HTTPException(status_code=503, detail=str(exc))


@app.get("/sessions/{session_id}", response_model=DiagnosticSession)
def get_session(
    session_id: str, repository: DiagnosticRepository = Depends(get_repository)
) -> DiagnosticSession:
    return _get_session_or_404(repository, session_id)


@app.get("/sessions/{session_id}/export")
def get_session_export(
    session_id: str, repository: DiagnosticRepository = Depends(get_repository)
) -> dict:
    session = _get_session_or_404(repository, session_id)
    proposals = repository.list_patch_proposals(session_id)
    return build_diagnostic_export(session, proposals)


@app.post("/sessions/{session_id}/evidence", response_model=SubmitEvidenceResponse)
def post_submit_evidence(
    session_id: str,
    request: SubmitEvidenceRequest,
    provider: AIProvider = Depends(get_provider),
    planner: TestPlanner = Depends(get_planner),
    catalog: dict[str, CatalogSpecEntry] = Depends(get_catalog),
    repository: DiagnosticRepository = Depends(get_repository),
    telemetry: TelemetrySink = Depends(get_telemetry_sink),
    analytics: AnalyticsSink = Depends(get_analytics_sink),
) -> SubmitEvidenceResponse:
    session = _get_session_or_404(repository, session_id)
    try:
        updated_session, duplicate_step = submit_evidence(
            session, request.evidence, catalog, provider, planner
        )
    except SessionStoppedError:
        raise HTTPException(
            status_code=409,
            detail=f"Session '{session_id}' is STOPPED and cannot accept new evidence.",
        )

    _save_session(repository, updated_session)
    if duplicate_step is None:
        new_step = updated_session.steps[-1]
        repository.append_event(
            new_event(
                session_id,
                "EVIDENCE_RECEIVED",
                {
                    "step_number": new_step.step_number,
                    "outcome": new_step.result.outcome,
                    "recommended_test": new_step.result.recommended_test,
                },
            )
        )
        if updated_session.status in ("DIAGNOSED", "STOPPED"):
            repository.append_event(
                new_event(session_id, "SESSION_COMPLETED", {"status": updated_session.status})
            )
        _record_telemetry_best_effort(telemetry, session_id, new_step)
        _record_analytics_best_effort(analytics, repository, updated_session)

    return SubmitEvidenceResponse(
        session=updated_session,
        duplicate=duplicate_step is not None,
        duplicate_of_step=duplicate_step.step_number if duplicate_step else None,
    )


@app.post("/sessions/{session_id}/stop", response_model=DiagnosticSession)
def post_stop_session(
    session_id: str, repository: DiagnosticRepository = Depends(get_repository)
) -> DiagnosticSession:
    session = _get_session_or_404(repository, session_id)
    stopped = stop_session(session)
    _save_session(repository, stopped)
    repository.append_event(new_event(session_id, "SESSION_STOPPED", {"stop_reason": stopped.stop_reason}))
    return stopped


# ---------------------------------------------------------------------------
# Milestone 6: PATCH proposal + VERIFY intelligence.
#
# Deliberately named /patch-proposals, never /patch: nothing under this
# prefix executes anything. Go's own PATCH_LOCKED gate
# (apps/api/internal/httpapi/server.go) is the sole, unmodified authority
# for actual hardware execution.
# ---------------------------------------------------------------------------


def _get_proposal_or_404(repository: DiagnosticRepository, proposal_id: str) -> PatchProposal:
    proposal = repository.get_patch_proposal(proposal_id)
    if proposal is None:
        raise HTTPException(status_code=404, detail=f"Patch proposal '{proposal_id}' does not exist.")
    return proposal


@app.post("/sessions/{session_id}/patch-proposals", response_model=PatchProposalResponse)
def post_create_patch_proposal(
    session_id: str,
    request: PatchProposalRequest,
    provider: PatchProposalProvider = Depends(get_patch_proposal_provider),
    repository: DiagnosticRepository = Depends(get_repository),
) -> PatchProposalResponse:
    session = _get_session_or_404(repository, session_id)
    try:
        proposal = create_patch_proposal(session, request, provider)
    except PatchProposalRejected as exc:
        raise HTTPException(
            status_code=422,
            detail={"unknown_reason": exc.unknown_reason, "reasoning_notes": exc.reasoning_notes},
        )
    repository.save_patch_proposal(proposal)
    repository.append_event(
        new_event(
            session_id,
            "PATCH_PROPOSED",
            {"proposal_id": proposal.proposal_id, "target_probe": proposal.target_probe, "target_role": proposal.target_role},
        )
    )
    return PatchProposalResponse(proposal=proposal)


@app.get("/sessions/{session_id}/patch-proposals", response_model=list[PatchProposal])
def list_patch_proposals(
    session_id: str, repository: DiagnosticRepository = Depends(get_repository)
) -> list[PatchProposal]:
    _get_session_or_404(repository, session_id)
    return repository.list_patch_proposals(session_id)


def _get_proposal_for_session_or_404(
    repository: DiagnosticRepository, session_id: str, proposal_id: str
) -> tuple[DiagnosticSession, PatchProposal]:
    session = _get_session_or_404(repository, session_id)
    proposal = _get_proposal_or_404(repository, proposal_id)
    if proposal.session_id != session_id:
        raise HTTPException(
            status_code=404,
            detail=f"Patch proposal '{proposal_id}' does not belong to session '{session_id}'.",
        )
    return session, proposal


@app.get("/sessions/{session_id}/patch-proposals/{proposal_id}", response_model=PatchProposal)
def get_patch_proposal(
    session_id: str, proposal_id: str, repository: DiagnosticRepository = Depends(get_repository)
) -> PatchProposal:
    _, proposal = _get_proposal_for_session_or_404(repository, session_id, proposal_id)
    return proposal


@app.post(
    "/sessions/{session_id}/patch-proposals/{proposal_id}/result", response_model=PatchProposal
)
def post_record_external_result(
    session_id: str,
    proposal_id: str,
    request: RecordExternalResultRequest,
    http_request: Request,
    repository: DiagnosticRepository = Depends(get_repository),
) -> PatchProposal:
    token = os.environ.get("PROBE_REPORTER_TOKEN", "")
    actor_id = os.environ.get("PROBE_REPORTER_ID", "").strip()
    if len(token) < 32 or re.fullmatch(r"[A-Za-z0-9._-]{1,100}", actor_id) is None:
        raise HTTPException(status_code=503, detail="Authenticated action reporting is not configured.")
    supplied = http_request.headers.get("Authorization", "")
    if not supplied.startswith("Bearer ") or not secrets.compare_digest(supplied[7:].encode(), token.encode()):
        raise HTTPException(status_code=401, detail="A valid reporter credential is required.", headers={"WWW-Authenticate": "Bearer"})
    _, proposal = _get_proposal_for_session_or_404(repository, session_id, proposal_id)
    try:
        updated = record_external_result(proposal, request, actor_id=actor_id, record_source="HUMAN_REPORTED")
    except InvalidProposalTransitionError as exc:
        raise HTTPException(status_code=409, detail=str(exc))
    repository.save_patch_proposal(updated)
    repository.append_event(
        new_event(session_id, "PATCH_EXTERNAL_RESULT", {
            "proposal_id": proposal_id, "reported_status": updated.status,
            "actor_id": actor_id, "record_source": "HUMAN_REPORTED",
            "recorded_at_ms": updated.action_history[-1].recorded_at_ms,
            "execution_verified": False,
        })
    )
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
    repository: DiagnosticRepository = Depends(get_repository),
) -> VerifyResponse:
    session, proposal = _get_proposal_for_session_or_404(repository, session_id, proposal_id)
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

    _save_session(repository, updated_session)
    repository.save_patch_proposal(updated_proposal)
    if verification is not None:
        repository.append_event(
            new_event(session_id, "VERIFY_COMPLETED", {"proposal_id": proposal_id, "outcome": verification.outcome})
        )
    return VerifyResponse(
        session=updated_session,
        proposal=updated_proposal,
        verification=verification,
        duplicate=duplicate_step is not None,
        duplicate_of_step=duplicate_step.step_number if duplicate_step else None,
    )
