import hashlib
import json
import time
import uuid
from functools import lru_cache
from typing import Protocol

from app.catalog import CatalogSpecEntry
from app.planner import TestPlanner
from app.probe import (
    UnknownComponentError,
    merge_component_specification,
    run_probe,
    unknown_component_response,
)
from app.providers.base import AIProvider
from app.schemas import (
    DiagnosticSession,
    DiagnosticStep,
    ProbeResponse,
    SessionStatus,
    StructuredEvidence,
)

DEFAULT_MAX_STEPS = 10


class SessionStoppedError(Exception):
    """Raised when new evidence is submitted to a STOPPED session."""


def _now_ms() -> int:
    return int(time.time() * 1000)


def new_session_id() -> str:
    return f"sess_{uuid.uuid4().hex}"


def fingerprint_evidence(evidence: StructuredEvidence) -> str:
    """Deterministic, stable-across-key-order fingerprint of submitted
    evidence. Used ONLY for duplicate-submission detection - never as a
    diagnostic fact and never sent to any provider."""
    canonical = json.dumps(evidence.model_dump(mode="json"), sort_keys=True, separators=(",", ":"))
    return hashlib.sha256(canonical.encode("utf-8")).hexdigest()


def _terminal_diagnosed(evaluated_evidence: StructuredEvidence, result: ProbeResponse) -> bool:
    """A session is only DIAGNOSED (terminal) when PROBE reached a grounded
    conclusion AND every deterministic rule for this step passed - i.e. the
    genuine "no fault detected" case. A DIAGNOSED result that still carries a
    fail/warn rule (a real, unresolved lead) keeps the session ACTIVE, since
    a meaningful next test still exists - this is deliberately independent
    of how confident any provider's hypothesis text sounds."""
    if result.outcome != "DIAGNOSED":
        return False
    return all(rule.status == "pass" for rule in evaluated_evidence.rule_results)


def _status_for_step(
    evaluated_evidence: StructuredEvidence, result: ProbeResponse, step_number: int, max_steps: int
) -> tuple[SessionStatus, str | None]:
    if result.outcome == "UNKNOWN":
        status: SessionStatus = "UNKNOWN"
    elif _terminal_diagnosed(evaluated_evidence, result):
        status = "DIAGNOSED"
    else:
        status = "ACTIVE"

    if step_number >= max_steps:
        return "STOPPED", "max_steps_reached"
    return status, None


def _run_step(
    evidence: StructuredEvidence,
    component_id: str | None,
    catalog: dict[str, CatalogSpecEntry],
    provider: AIProvider,
    planner: TestPlanner,
) -> tuple[StructuredEvidence, ProbeResponse]:
    """Runs the exact same catalog/spec-merge + PROBE pipeline `POST /probe`
    uses (app.probe.run_probe, unmodified) and separately recovers the
    post-merge evaluated evidence for step-history transparency. component_id
    is fixed for the whole session, so this runs identically on every step -
    Milestone 4 behavior is preserved on every applicable diagnostic step."""
    try:
        evaluated = merge_component_specification(evidence, catalog, component_id)
    except UnknownComponentError:
        evaluated = evidence
    result = run_probe(evidence, provider, planner, catalog, component_id)
    return evaluated, result


def _append_step(
    session: DiagnosticSession,
    evidence: StructuredEvidence,
    catalog: dict[str, CatalogSpecEntry],
    provider: AIProvider,
    planner: TestPlanner,
) -> DiagnosticSession:
    step_number = len(session.steps) + 1
    evaluated, result = _run_step(evidence, session.component_id, catalog, provider, planner)
    status, stop_reason = _status_for_step(evaluated, result, step_number, session.max_steps)

    step = DiagnosticStep(
        step_number=step_number,
        submitted_evidence=evidence,
        evaluated_evidence=evaluated,
        evidence_fingerprint=fingerprint_evidence(evidence),
        result=result,
        created_at_ms=_now_ms(),
    )
    # Append-only: steps is rebuilt as [*old, new], never mutating an
    # existing DiagnosticStep in place.
    return session.model_copy(
        update={
            "steps": [*session.steps, step],
            "status": status,
            "stop_reason": stop_reason,
            "updated_at_ms": _now_ms(),
        }
    )


def create_session(
    evidence: StructuredEvidence,
    component_id: str | None,
    catalog: dict[str, CatalogSpecEntry],
    provider: AIProvider,
    planner: TestPlanner,
    max_steps: int | None = None,
) -> DiagnosticSession:
    now = _now_ms()
    session = DiagnosticSession(
        session_id=new_session_id(),
        component_id=component_id,
        probe=evidence.probe,
        role=evidence.role,
        status="ACTIVE",
        max_steps=max_steps or DEFAULT_MAX_STEPS,
        steps=[],
        created_at_ms=now,
        updated_at_ms=now,
    )
    return _append_step(session, evidence, catalog, provider, planner)


def find_duplicate_step(session: DiagnosticSession, evidence: StructuredEvidence) -> DiagnosticStep | None:
    """Exact-duplicate detection: identical (canonically serialized)
    submitted evidence anywhere in this session's history - not merely the
    most recent step - since resubmitting an EARLIER step's evidence is
    equally "no new information"."""
    fp = fingerprint_evidence(evidence)
    for step in session.steps:
        if step.evidence_fingerprint == fp:
            return step
    return None


def submit_evidence(
    session: DiagnosticSession,
    evidence: StructuredEvidence,
    catalog: dict[str, CatalogSpecEntry],
    provider: AIProvider,
    planner: TestPlanner,
) -> tuple[DiagnosticSession, DiagnosticStep | None]:
    """Returns (possibly-updated session, duplicate_step_or_None).

    A duplicate_step being non-None means: no new step was appended, the
    session is returned unchanged, and the caller should report a
    no-new-information result referencing that existing step - submitting
    the same evidence twice must never fabricate progress or a new
    diagnosis merely because PROBE was invoked again.
    """
    if session.status == "STOPPED":
        raise SessionStoppedError(session.session_id)

    duplicate = find_duplicate_step(session, evidence)
    if duplicate is not None:
        return session, duplicate

    return _append_step(session, evidence, catalog, provider, planner), None


def stop_session(session: DiagnosticSession) -> DiagnosticSession:
    if session.status == "STOPPED":
        return session
    return session.model_copy(
        update={"status": "STOPPED", "stop_reason": "stopped_by_user", "updated_at_ms": _now_ms()}
    )


class SessionStore(Protocol):
    def get(self, session_id: str) -> DiagnosticSession | None: ...

    def save(self, session: DiagnosticSession) -> None: ...


class InMemorySessionStore:
    """No database in Milestone 5: a plain in-process dict. Deterministic,
    simple, trivially testable, and swappable later behind the same
    SessionStore protocol without touching the API layer."""

    def __init__(self) -> None:
        self._sessions: dict[str, DiagnosticSession] = {}

    def get(self, session_id: str) -> DiagnosticSession | None:
        return self._sessions.get(session_id)

    def save(self, session: DiagnosticSession) -> None:
        self._sessions[session.session_id] = session


@lru_cache
def get_default_session_store() -> InMemorySessionStore:
    # lru_cache here (unlike get_default_catalog) is deliberately used for
    # its singleton behavior, not for avoiding recomputation: FastAPI's
    # Depends() needs the SAME mutable store instance across requests
    # within one process for in-memory sessions to persist at all.
    return InMemorySessionStore()
