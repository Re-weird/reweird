import time
import uuid
from typing import Any, Literal

from pydantic import BaseModel, Field

# History, not new evidence: an event RECORDS that something happened
# elsewhere (a step was appended, a proposal was created); it never carries
# enough information to be replayed as a diagnostic input, and its own
# "reference" payload is intentionally small (ids/outcomes), never a
# duplicate copy of a full StructuredEvidence blob that already lives in the
# session's own step history.
DiagnosticEventType = Literal[
    "SESSION_CREATED",
    "EVIDENCE_RECEIVED",
    "PROBE_COMPLETED",
    "PATCH_PROPOSED",
    "PATCH_EXTERNAL_RESULT",
    "VERIFY_COMPLETED",
    "SESSION_STOPPED",
    "SESSION_COMPLETED",
]


class DiagnosticEvent(BaseModel):
    event_id: str
    session_id: str
    event_type: DiagnosticEventType
    created_at_ms: int
    reference: dict[str, Any] = Field(default_factory=dict)
    # Provenance of the EVENT RECORD itself, not of any evidence it points
    # at - system-generated history is always SOFTWARE, never
    # AI_INTERPRETATION (an event is never itself an AI conclusion).
    provenance: Literal["SOFTWARE"] = "SOFTWARE"


def new_event(session_id: str, event_type: DiagnosticEventType, reference: dict[str, Any]) -> DiagnosticEvent:
    return DiagnosticEvent(
        event_id=f"evt_{uuid.uuid4().hex}",
        session_id=session_id,
        event_type=event_type,
        created_at_ms=int(time.time() * 1000),
        reference=reference,
    )
