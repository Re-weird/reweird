from typing import Annotated, Literal

from pydantic import BaseModel, ConfigDict, Field, field_validator, model_validator

RefId = Annotated[str, Field(min_length=1, max_length=160)]
CatalogId = Annotated[str, Field(min_length=1, max_length=64)]
Note = Annotated[str, Field(min_length=1, max_length=500)]


class ComponentCandidateOut(BaseModel):
    model_config = ConfigDict(extra="forbid")

    catalog_id: CatalogId
    confidence: float = Field(ge=0.0, le=1.0)
    grounded_in: list[RefId] = Field(min_length=1, max_length=20)
    rationale: str = Field(min_length=1, max_length=500)

    @field_validator("grounded_in")
    @classmethod
    def _no_duplicates(cls, v: list[str]) -> list[str]:
        if len(set(v)) != len(v):
            raise ValueError("duplicate entries are not allowed")
        return v


class RoleCandidateOut(BaseModel):
    model_config = ConfigDict(extra="forbid")

    target_pin: str = Field(min_length=1, max_length=64)
    likely_role: str = Field(min_length=1, max_length=64)
    mode: Literal["analog", "digital", "pulse"]
    is_power_rail: bool
    confidence: float = Field(ge=0.0, le=1.0)
    grounded_in: list[RefId] = Field(min_length=1, max_length=20)

    @field_validator("grounded_in")
    @classmethod
    def _no_duplicates(cls, v: list[str]) -> list[str]:
        if len(set(v)) != len(v):
            raise ValueError("duplicate entries are not allowed")
        return v


class CodeInterpretationOut(BaseModel):
    model_config = ConfigDict(extra="forbid")

    outcome: Literal["INTERPRETED", "UNKNOWN"]
    expected_behavior_summary: str | None = Field(default=None, max_length=500)
    controller: str | None = Field(default=None, max_length=64)
    component_candidates: list[ComponentCandidateOut] = Field(default_factory=list, max_length=10)
    role_candidates: list[RoleCandidateOut] = Field(default_factory=list, max_length=20)
    reasoning_notes: list[Note] = Field(default_factory=list, max_length=10)
    unknown_reason: Literal["PROVIDER_UNCERTAIN"] | None = None

    @model_validator(mode="after")
    def _outcome_consistency(self) -> "CodeInterpretationOut":
        if self.outcome == "INTERPRETED":
            if self.unknown_reason is not None:
                raise ValueError("INTERPRETED must not carry an unknown_reason")
        else:  # UNKNOWN
            if (
                self.component_candidates
                or self.role_candidates
                or self.unknown_reason != "PROVIDER_UNCERTAIN"
            ):
                raise ValueError(
                    "UNKNOWN requires zero candidates and unknown_reason=PROVIDER_UNCERTAIN"
                )
        return self
