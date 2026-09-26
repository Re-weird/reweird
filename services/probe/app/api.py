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
from app.schemas import (
    CreateSessionRequest,
    DiagnosticSession,
    ProbeRequest,
    ProbeResponse,
    SubmitEvidenceRequest,
    SubmitEvidenceResponse,
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
    yield


app = FastAPI(title="reweird-probe", version="0.1.0", lifespan=lifespan)


def get_planner() -> TestPlanner:
    return TestPlanner()


def get_catalog() -> dict[str, CatalogSpecEntry]:
    return get_default_catalog()


def get_session_store() -> InMemorySessionStore:
    return get_default_session_store()


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
