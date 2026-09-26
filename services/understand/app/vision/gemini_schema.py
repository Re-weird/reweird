from typing import Annotated, Literal

from pydantic import BaseModel, ConfigDict, Field, model_validator

CatalogId = Annotated[str, Field(min_length=1, max_length=64)]
Note = Annotated[str, Field(min_length=1, max_length=500)]


class VisionComponentCandidateOut(BaseModel):
    model_config = ConfigDict(extra="forbid")

    # Deliberately NO numeric/measurement field anywhere in this model - a
    # structural guarantee that vision output can never claim an exact
    # electrical measurement, not merely a prompt-level request not to.
    catalog_id: CatalogId
    confidence: float = Field(ge=0.0, le=1.0)
    rationale: str = Field(min_length=1, max_length=500)


class VisionInterpretationOut(BaseModel):
    model_config = ConfigDict(extra="forbid")

    outcome: Literal["INTERPRETED", "UNKNOWN"]
    component_candidates: list[VisionComponentCandidateOut] = Field(
        default_factory=list, max_length=10
    )
    reasoning_notes: list[Note] = Field(default_factory=list, max_length=10)
    unknown_reason: Literal["PROVIDER_UNCERTAIN"] | None = None

    @model_validator(mode="after")
    def _outcome_consistency(self) -> "VisionInterpretationOut":
        if self.outcome == "INTERPRETED":
            if self.unknown_reason is not None:
                raise ValueError("INTERPRETED must not carry an unknown_reason")
        else:  # UNKNOWN
            if self.component_candidates or self.unknown_reason != "PROVIDER_UNCERTAIN":
                raise ValueError(
                    "UNKNOWN requires zero candidates and unknown_reason=PROVIDER_UNCERTAIN"
                )
        return self
