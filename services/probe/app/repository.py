"""DiagnosticRepository: the storage abstraction M5/M6 previously lacked.

Sponsor systems (MongoDB Atlas here) STORE/INDEX/ANALYZE diagnostic history.
They never decide electrical truth - every method on this Protocol only
ever accepts an already-computed DiagnosticSession/PatchProposal/
DiagnosticEvent (all produced by app.sessions/app.patch_proposals'
deterministic + Gemini pipeline) and stores or returns it unchanged. Nothing
here re-derives a diagnosis from stored data.
"""

import logging
from functools import lru_cache
from typing import Protocol

from app.config import ProbeSettings
from app.events import DiagnosticEvent
from app.patch_proposals import InMemoryPatchProposalStore
from app.schemas import DiagnosticSession, PatchProposal
from app.sessions import InMemorySessionStore

logger = logging.getLogger("app.repository")


class RepositoryUnavailableError(Exception):
    """Raised by a PRIMARY repository on connection/read/write failure.

    Unlike an optional telemetry/analytics sink failure (which is logged and
    swallowed - infrastructure trouble is not diagnostic evidence), a
    repository failure is surfaced clearly to the caller: the caller asked
    for persistence and persistence did not happen. Never silently fall back
    to pretending an in-memory write succeeded.
    """


class DiagnosticRepository(Protocol):
    def create_session(self, session: DiagnosticSession) -> None: ...

    def get_session(self, session_id: str) -> DiagnosticSession | None: ...

    def save_session(self, session: DiagnosticSession) -> None: ...

    def list_sessions(self) -> list[DiagnosticSession]: ...

    def save_patch_proposal(self, proposal: PatchProposal) -> None: ...

    def get_patch_proposal(self, proposal_id: str) -> PatchProposal | None: ...

    def list_patch_proposals(self, session_id: str) -> list[PatchProposal]: ...

    def append_event(self, event: DiagnosticEvent) -> None: ...

    def list_events(self, session_id: str) -> list[DiagnosticEvent]: ...


class InMemoryDiagnosticRepository:
    """Default, safe-by-default repository: no database, no network.

    Composes the exact, already-tested Milestone 5/6 in-memory stores rather
    than reimplementing their storage logic - `InMemorySessionStore` and
    `InMemoryPatchProposalStore` remain available and unchanged for any code
    that still constructs them directly.
    """

    def __init__(self) -> None:
        self._sessions = InMemorySessionStore()
        self._proposals = InMemoryPatchProposalStore()
        self._events: list[DiagnosticEvent] = []

    def create_session(self, session: DiagnosticSession) -> None:
        self._sessions.save(session)

    def get_session(self, session_id: str) -> DiagnosticSession | None:
        return self._sessions.get(session_id)

    def save_session(self, session: DiagnosticSession) -> None:
        self._sessions.save(session)

    def list_sessions(self) -> list[DiagnosticSession]:
        return self._sessions.list_all()

    def save_patch_proposal(self, proposal: PatchProposal) -> None:
        self._proposals.save(proposal)

    def get_patch_proposal(self, proposal_id: str) -> PatchProposal | None:
        return self._proposals.get(proposal_id)

    def list_patch_proposals(self, session_id: str) -> list[PatchProposal]:
        return self._proposals.list_for_session(session_id)

    def append_event(self, event: DiagnosticEvent) -> None:
        self._events.append(event)

    def list_events(self, session_id: str) -> list[DiagnosticEvent]:
        return [e for e in self._events if e.session_id == session_id]


@lru_cache
def get_default_in_memory_repository() -> InMemoryDiagnosticRepository:
    return InMemoryDiagnosticRepository()


class MongoDiagnosticRepository:
    """MongoDB Atlas-backed repository.

    Accepts an injected, duck-typed `database` object exposing `.sessions`,
    `.patch_proposals`, and `.events` collections (each supporting
    `insert_one`/`replace_one`/`find_one`/`find`, i.e. the same surface
    pymongo's `Collection` already has) - exactly the same
    constructor-injection pattern `GeminiAIProvider`/`VisionInterpretationProvider`
    already use for testability. Default offline tests inject a small fake;
    they never require the real `pymongo` package or a live cluster.

    Every method wraps the underlying call and re-raises any failure as
    `RepositoryUnavailableError` - a caller-selected PRIMARY repository
    failure must be surfaced, never silently swallowed as if the write or
    read had succeeded.
    """

    def __init__(self, settings: ProbeSettings, database: object | None = None):
        if database is not None:
            self._database = database
        else:
            if not settings.mongodb_uri:
                raise RuntimeError(
                    "MONGODB_URI is required when DIAGNOSTIC_REPOSITORY=mongodb"
                )
            self._database = self._connect(settings.mongodb_uri, settings.mongodb_database)

    @staticmethod
    def _connect(uri: str, database_name: str) -> object:
        # Lazy import, exactly like _RealGeminiClient's `from google import
        # genai` - pymongo is a real dependency (see pyproject.toml) but is
        # only ever imported when DIAGNOSTIC_REPOSITORY=mongodb is actually
        # selected with no injected fake database.
        import pymongo

        client = pymongo.MongoClient(uri, serverSelectionTimeoutMS=5000)
        return client[database_name]

    def _run(self, op_name: str, fn):
        try:
            return fn()
        except Exception as exc:  # noqa: BLE001 - never leak driver internals raw
            logger.error("mongo_repository_%s_failed: %s", op_name, type(exc).__name__)
            raise RepositoryUnavailableError(f"MongoDB {op_name} failed: {type(exc).__name__}") from exc

    def create_session(self, session: DiagnosticSession) -> None:
        self.save_session(session)

    def get_session(self, session_id: str) -> DiagnosticSession | None:
        def _get():
            doc = self._database.sessions.find_one({"_id": session_id})
            return DiagnosticSession.model_validate(doc) if doc else None

        return self._run("get_session", _get)

    def save_session(self, session: DiagnosticSession) -> None:
        def _save():
            doc = {"_id": session.session_id, **session.model_dump(mode="json")}
            self._database.sessions.replace_one({"_id": session.session_id}, doc, upsert=True)

        self._run("save_session", _save)

    def list_sessions(self) -> list[DiagnosticSession]:
        def _list():
            return [
                DiagnosticSession.model_validate(doc)
                for doc in sorted(self._database.sessions.find({}), key=lambda d: d.get("created_at_ms", 0))
            ]

        return self._run("list_sessions", _list)

    def save_patch_proposal(self, proposal: PatchProposal) -> None:
        def _save():
            doc = {"_id": proposal.proposal_id, **proposal.model_dump(mode="json")}
            self._database.patch_proposals.replace_one({"_id": proposal.proposal_id}, doc, upsert=True)

        self._run("save_patch_proposal", _save)

    def get_patch_proposal(self, proposal_id: str) -> PatchProposal | None:
        def _get():
            doc = self._database.patch_proposals.find_one({"_id": proposal_id})
            return PatchProposal.model_validate(doc) if doc else None

        return self._run("get_patch_proposal", _get)

    def list_patch_proposals(self, session_id: str) -> list[PatchProposal]:
        def _list():
            return [
                PatchProposal.model_validate(doc)
                for doc in sorted(
                    self._database.patch_proposals.find({"session_id": session_id}),
                    key=lambda d: d.get("created_at_ms", 0),
                )
            ]

        return self._run("list_patch_proposals", _list)

    def append_event(self, event: DiagnosticEvent) -> None:
        def _append():
            doc = {"_id": event.event_id, **event.model_dump(mode="json")}
            self._database.events.insert_one(doc)

        self._run("append_event", _append)

    def list_events(self, session_id: str) -> list[DiagnosticEvent]:
        def _list():
            return [
                DiagnosticEvent.model_validate(doc)
                for doc in sorted(
                    self._database.events.find({"session_id": session_id}),
                    key=lambda d: d.get("created_at_ms", 0),
                )
            ]

        return self._run("list_events", _list)


@lru_cache
def get_repository() -> DiagnosticRepository:
    from app.config import get_settings

    settings = get_settings()
    if settings.diagnostic_repository == "mongodb":
        return MongoDiagnosticRepository(settings)
    return get_default_in_memory_repository()
