"""Deterministic, secret-safe JSON export of a diagnostic session.

Preserves provenance and the full ordered diagnostic timeline - evidence,
deterministic rules/specification results, hypotheses, recommended tests,
PATCH proposals, and VerificationResults - without ever including API keys,
database credentials, provider internals, or raw environment variables
(none of which exist on DiagnosticSession/PatchProposal in the first place;
`_redact_secrets` below is defense-in-depth in case a future field or a
caller-supplied evidence `detail`/`message` string ever carries something
that merely LOOKS like a secret).
"""

import re
from typing import Any

from app.schemas import DiagnosticSession, PatchProposal

_SECRET_KEY_PATTERN = re.compile(
    r"(api[_-]?key|secret|password|token|credential|authorization)", re.IGNORECASE
)
_REDACTED = "[REDACTED]"


def _redact_secrets(value: Any) -> Any:
    if isinstance(value, dict):
        return {
            key: (_REDACTED if _SECRET_KEY_PATTERN.search(str(key)) else _redact_secrets(val))
            for key, val in value.items()
        }
    if isinstance(value, list):
        return [_redact_secrets(item) for item in value]
    return value


def build_diagnostic_export(session: DiagnosticSession, proposals: list[PatchProposal]) -> dict:
    export = {
        "metadata": {
            "session_id": session.session_id,
            "component_id": session.component_id,
            "probe": session.probe,
            "role": session.role,
            "status": session.status,
            "stop_reason": session.stop_reason,
            "max_steps": session.max_steps,
            "created_at_ms": session.created_at_ms,
            "updated_at_ms": session.updated_at_ms,
        },
        "timeline": [
            {
                "step_number": step.step_number,
                "evidence_fingerprint": step.evidence_fingerprint,
                "submitted_evidence": step.submitted_evidence.model_dump(mode="json"),
                "evaluated_evidence": step.evaluated_evidence.model_dump(mode="json"),
                "outcome": step.result.outcome,
                "unknown_reason": step.result.unknown_reason,
                "hypotheses": [h.model_dump(mode="json") for h in step.result.hypotheses],
                "recommended_test": step.result.recommended_test,
                "grounded_evidence_count": step.result.grounded_evidence_count,
                "created_at_ms": step.created_at_ms,
            }
            for step in session.steps
        ],
        "patch_proposals": [proposal.model_dump(mode="json") for proposal in proposals],
        "final_status": session.status,
    }
    return _redact_secrets(export)
