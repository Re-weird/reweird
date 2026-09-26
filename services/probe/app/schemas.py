from typing import Any, Literal

from pydantic import BaseModel, Field

EvidenceProvenance = Literal[
    "MEASURED", "DERIVED", "SPECIFICATION", "BASELINE", "SOFTWARE", "AI_INTERPRETATION"
]

RuleStatus = Literal["pass", "warn", "fail"]

GroundedCategory = Literal[
    "measurement", "derived_fact", "specification_result", "baseline_comparison", "rule_result"
]


class EvidenceFact(BaseModel):
    probe: str | None = None
    name: str
    value: Any
    unit: str | None = None
    provenance: EvidenceProvenance
    detail: str | None = None


class RuleResult(BaseModel):
    id: str
    probe: str | None = None
    status: RuleStatus
    severity: int | None = None
    message: str
    provenance: EvidenceProvenance | None = None


class StructuredEvidence(BaseModel):
    probe: str
    role: str
    expected: dict[str, str | int | float | bool]
    observed: dict[str, str | int | float | bool]
    baseline: dict[str, str | int | float | bool]
    measurements: list[EvidenceFact] | None = None
    derived_facts: list[EvidenceFact] | None = None
    specification_results: list[EvidenceFact] | None = None
    baseline_comparison: list[EvidenceFact] | None = None
    rule_results: list[RuleResult]
    unresolved_questions: list[str] | None = None


class GroundedItem(BaseModel):
    ref_id: str
    category: GroundedCategory
    probe: str | None
    name: str
    status: RuleStatus | None = None
    provenance: EvidenceProvenance | None = None
    summary: str


class Hypothesis(BaseModel):
    rank: int
    label: str
    explanation: str
    confidence: float
    grounded_in: list[str]
    supporting_rule_ids: list[str]
    provenance: Literal["AI_INTERPRETATION"] = "AI_INTERPRETATION"


class ProbeRequest(BaseModel):
    evidence: StructuredEvidence
    # Optional catalog id (e.g. "hc-sr04") for THIS probe's role. Omitted by
    # every Milestone 1/2 caller - behavior is then unchanged from before
    # Milestone 4 existed. See app/specification.py.
    component_id: str | None = None


UnknownReason = Literal[
    "NO_EVIDENCE",
    "CONFLICTING_RULES",
    "PROVIDER_ERROR",
    "INVALID_PROVIDER_OUTPUT",
    "PROVIDER_UNCERTAIN",
    "UNKNOWN_COMPONENT",
]


class ProbeResponse(BaseModel):
    probe: str
    role: str
    outcome: Literal["DIAGNOSED", "UNKNOWN"]
    hypotheses: list[Hypothesis]
    recommended_test: str
    unresolved_questions: list[str]
    grounded_evidence_count: int
    unknown_reason: UnknownReason | None = None
    reasoning_notes: list[str] = Field(default_factory=list)


# ---------------------------------------------------------------------------
# Milestone 5: closed-loop diagnostic sessions
# ---------------------------------------------------------------------------

SessionStatus = Literal["ACTIVE", "DIAGNOSED", "UNKNOWN", "STOPPED"]


class DiagnosticStep(BaseModel):
    """One append-only round of the diagnostic loop.

    `submitted_evidence` is exactly what the caller sent - never mutated.
    `evaluated_evidence` is that same evidence AFTER Milestone 4's
    deterministic catalog/specification merge (a no-op copy when no
    component_id is set) - this is the "deterministic specification/rule
    state" a reviewer can audit without re-running anything.
    """

    step_number: int
    submitted_evidence: StructuredEvidence
    evaluated_evidence: StructuredEvidence
    evidence_fingerprint: str
    result: ProbeResponse
    created_at_ms: int


class DiagnosticSession(BaseModel):
    session_id: str
    component_id: str | None = None
    probe: str
    role: str
    status: SessionStatus
    stop_reason: str | None = None
    max_steps: int
    steps: list[DiagnosticStep] = Field(default_factory=list)
    created_at_ms: int
    updated_at_ms: int


class CreateSessionRequest(BaseModel):
    evidence: StructuredEvidence
    component_id: str | None = None
    max_steps: int | None = None


class SubmitEvidenceRequest(BaseModel):
    evidence: StructuredEvidence


class SubmitEvidenceResponse(BaseModel):
    session: DiagnosticSession
    duplicate: bool
    duplicate_of_step: int | None = None
