from typing import Any, Literal

from pydantic import BaseModel, Field, model_validator

# Reused verbatim from the shared EvidenceProvenance vocabulary
# (packages/shared-types, apps/api/internal/domain/models.go, and already
# established in services/probe). Only the two values this service actually
# produces are declared here.
Provenance = Literal["SOFTWARE", "AI_INTERPRETATION"]

CodeFactKind = Literal[
    "pin_constant",
    "pin_mode_call",
    "hardware_api_call",
    "include_directive",
    "symbol_reference",
]

Language = Literal["cpp", "unsupported"]

UnknownReason = Literal[
    "INSUFFICIENT_INFORMATION",
    "PROVIDER_ERROR",
    "INVALID_PROVIDER_OUTPUT",
    "PROVIDER_UNCERTAIN",
]

CandidateSource = Literal["code_analysis", "vision"]


class CodeFact(BaseModel):
    """A single literal, deterministic fact extracted from source code.

    Never a diagnostic conclusion, never a component/role guess - only what
    is literally present in the parsed syntax tree.
    """

    ref_id: str
    file: str
    line: int
    kind: CodeFactKind
    symbol: str | None = None
    pin: str | None = None
    mode: Literal["INPUT", "OUTPUT", "INPUT_PULLUP", "INPUT_PULLDOWN"] | None = None
    api_call: str | None = None
    raw_snippet: str = Field(max_length=240)
    provenance: Literal["SOFTWARE"] = "SOFTWARE"


class CodeAnalysisResult(BaseModel):
    files_analyzed: list[str]
    language: Language
    facts: list[CodeFact]
    parse_errors: list[str] = Field(default_factory=list)


class CatalogEntry(BaseModel):
    id: str
    name: str
    pins: list[str] = Field(default_factory=list)
    supply_voltage: dict[str, Any] = Field(default_factory=dict)
    interfaces: list[str] = Field(default_factory=list)
    notes: list[str] = Field(default_factory=list)

    @model_validator(mode="before")
    @classmethod
    def _map_current_catalog_schema(cls, data: Any) -> Any:
        """packages/component-catalog's authoritative schema (owned by the
        Go API, see packages/component-catalog/catalog.go) uses
        pin_roles/operating_voltage/known_signal_types/safe_measurement_notes
        rather than this service's original pins/supply_voltage/interfaces/
        notes names. Map the current field names onto the old ones (only
        when the old name is absent) so this service's Gemini-facing
        catalog index stays meaningful without requiring every catalog
        entry to carry two parallel schemas."""
        if not isinstance(data, dict):
            return data
        mapped = dict(data)
        if "pins" not in mapped and isinstance(mapped.get("pin_roles"), list):
            mapped["pins"] = [
                role["name"] for role in mapped["pin_roles"] if isinstance(role, dict) and role.get("name")
            ]
        if "supply_voltage" not in mapped and isinstance(mapped.get("operating_voltage"), dict):
            mapped["supply_voltage"] = mapped["operating_voltage"]
        if "interfaces" not in mapped:
            if isinstance(mapped.get("known_signal_types"), list):
                mapped["interfaces"] = mapped["known_signal_types"]
            elif isinstance(mapped.get("interface_type"), str) and mapped["interface_type"]:
                mapped["interfaces"] = [mapped["interface_type"]]
        if "notes" not in mapped and isinstance(mapped.get("safe_measurement_notes"), list):
            mapped["notes"] = mapped["safe_measurement_notes"]
        return mapped


class ConflictNote(BaseModel):
    area: str
    code_catalog_id: str | None
    vision_catalog_id: str | None
    detail: str


class ProposedRole(BaseModel):
    """A likely pin role in the TARGET project's own pin namespace.

    Deliberately has NO `probe` field: which of ReWeird's own P1-P6 harness
    channels gets physically clipped onto this pin can never be determined
    from source code or a photo - that mapping is a later, human, physical
    step, entirely out of scope for this service.
    """

    target_pin: str
    role: str = Field(max_length=64)
    mode: Literal["analog", "digital", "pulse"]
    is_power_rail: bool
    expected: dict[str, Any] = Field(default_factory=dict)
    confidence: float = Field(ge=0.0, le=1.0)
    sources: list[CandidateSource]
    grounded_in: list[str]


class ComponentProposal(BaseModel):
    catalog_id: str
    name: str
    manufacturer: str | None = None
    source: str
    confidence: float = Field(ge=0.0, le=1.0)
    sources: list[CandidateSource]
    grounded_in: list[str]


class ProposedProject(BaseModel):
    project_name: str | None = None
    controller: str | None = None
    logic_voltage: float | None = None
    expected_behavior: str | None = None
    components: list[ComponentProposal] = Field(default_factory=list)
    roles: list[ProposedRole] = Field(default_factory=list)


class ProjectProfileProposal(BaseModel):
    status: Literal["PROPOSED", "PROPOSED_WITH_CONFLICTS", "INSUFFICIENT_INFORMATION"]
    project: ProposedProject | None = None
    conflicts: list[ConflictNote] = Field(default_factory=list)
    unresolved_questions: list[str] = Field(default_factory=list)
    code_analysis: CodeAnalysisResult
    vision_analysis: "VisionAnalysisResult | None" = None
    reasoning_notes: list[str] = Field(default_factory=list)


class VisionAnalysisResult(BaseModel):
    outcome: Literal["INTERPRETED", "UNKNOWN"]
    component_candidates: list[ComponentProposal] = Field(default_factory=list)
    unknown_reason: UnknownReason | None = None
    reasoning_notes: list[str] = Field(default_factory=list)


ProjectProfileProposal.model_rebuild()
