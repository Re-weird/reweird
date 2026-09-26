"""Snowflake analytics/export sink.

Optional, out of PROBE's critical path: an AnalyticsSink only ever receives
a small, already-computed SessionAnalyticsSummary. It never influences a
diagnosis, and its failure must never alter the diagnostic result - see
app/api.py's best-effort, exception-swallowed call sites. The real
`snowflake-connector-python` SDK is deliberately NOT vendored as a
dependency (it pulls in a large transitive dependency tree); selecting
ANALYTICS_SINK=snowflake without it installed fails clearly at
construction, never silently, and never by pretending the connection
succeeded.
"""

import logging
from typing import Protocol

from pydantic import BaseModel

logger = logging.getLogger("app.analytics_sink")


class AnalyticsSinkUnavailableError(Exception):
    """Raised by a real sink on connection/write failure or a missing
    optional dependency. Callers (app.api) catch this, log it, and
    continue - analytics is optional history, never a gate on the actual
    diagnostic response."""


class SessionAnalyticsSummary(BaseModel):
    session_id: str
    component_id: str | None
    probe: str
    role: str
    step_count: int
    final_status: str
    unknown_reason: str | None
    rule_failure_count: int
    patch_proposed: bool
    verification_outcome: str | None
    duration_ms: int
    created_at_ms: int


class AnalyticsSink(Protocol):
    def record_session_summary(self, summary: SessionAnalyticsSummary) -> None: ...


class NullAnalyticsSink:
    """Default: does nothing. ANALYTICS_SINK=none."""

    def record_session_summary(self, summary: SessionAnalyticsSummary) -> None:
        pass


class InMemoryAnalyticsSink:
    """Offline test fake - never touches a network."""

    def __init__(self) -> None:
        self.summaries: list[SessionAnalyticsSummary] = []

    def record_session_summary(self, summary: SessionAnalyticsSummary) -> None:
        self.summaries.append(summary)


class SnowflakeAnalyticsSink:
    """Real adapter. Accepts an injected, duck-typed `connection` (anything
    with `.cursor()` -> object with `.execute()`) for testability - the same
    constructor-injection pattern every other real-client adapter in this
    service already uses. Without an injected connection, the real
    `snowflake-connector-python` package is required and is imported eagerly
    at construction time (fail fast at startup, matching the
    GEMINI_API_KEY-at-boot precedent), not deferred to the first write.
    """

    _INSERT_SQL = (
        "INSERT INTO probe_session_summary "
        "(session_id, component_id, probe, role, step_count, final_status, unknown_reason, "
        "rule_failure_count, patch_proposed, verification_outcome, duration_ms, created_at_ms) "
        "VALUES (%(session_id)s, %(component_id)s, %(probe)s, %(role)s, %(step_count)s, "
        "%(final_status)s, %(unknown_reason)s, %(rule_failure_count)s, %(patch_proposed)s, "
        "%(verification_outcome)s, %(duration_ms)s, %(created_at_ms)s)"
    )

    def __init__(
        self,
        account: str,
        user: str,
        password: str,
        database: str,
        warehouse: str,
        connection: object | None = None,
    ):
        if connection is not None:
            self._connection = connection
        else:
            try:
                import snowflake.connector
            except Exception as exc:  # noqa: BLE001
                raise AnalyticsSinkUnavailableError(
                    "snowflake-connector-python is not installed; ANALYTICS_SINK=snowflake "
                    "requires it (deliberately not vendored by default - see README)"
                ) from exc
            try:
                self._connection = snowflake.connector.connect(
                    account=account, user=user, password=password, database=database, warehouse=warehouse
                )
            except Exception as exc:  # noqa: BLE001
                raise AnalyticsSinkUnavailableError(f"Snowflake connection failed: {type(exc).__name__}") from exc

    def record_session_summary(self, summary: SessionAnalyticsSummary) -> None:
        try:
            with self._connection.cursor() as cur:
                cur.execute(self._INSERT_SQL, summary.model_dump(mode="json"))
        except Exception as exc:  # noqa: BLE001
            logger.error("snowflake_write_failed: %s", type(exc).__name__)
            raise AnalyticsSinkUnavailableError(f"Snowflake write failed: {type(exc).__name__}") from exc


def build_session_summary(session, proposals: list) -> SessionAnalyticsSummary:
    """Pure helper deriving only safe, aggregate fields from a session and
    its patch proposals - never raw evidence, never source code/images."""
    last_step = session.steps[-1] if session.steps else None
    rule_failure_count = (
        sum(1 for r in last_step.evaluated_evidence.rule_results if r.status == "fail") if last_step else 0
    )
    verified = next((p for p in proposals if p.verification is not None), None)
    duration_ms = session.updated_at_ms - session.created_at_ms
    return SessionAnalyticsSummary(
        session_id=session.session_id,
        component_id=session.component_id,
        probe=session.probe,
        role=session.role,
        step_count=len(session.steps),
        final_status=session.status,
        unknown_reason=last_step.result.unknown_reason if last_step else None,
        rule_failure_count=rule_failure_count,
        patch_proposed=len(proposals) > 0,
        verification_outcome=verified.verification.outcome if verified else None,
        duration_ms=duration_ms,
        created_at_ms=session.created_at_ms,
    )
