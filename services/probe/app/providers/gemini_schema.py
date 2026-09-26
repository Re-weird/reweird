from typing import Annotated, Literal

from pydantic import BaseModel, ConfigDict, Field, field_validator, model_validator

# Per-item bounds, not just list-length bounds: each element of grounded_in/
# supporting_rule_ids/reasoning_notes is independently constrained so an
# empty string or an oversized item is rejected at parse time, the same as
# an oversized or over-long list would be.
RefId = Annotated[str, Field(min_length=1, max_length=128)]
RuleId = Annotated[str, Field(min_length=1, max_length=64)]
Note = Annotated[str, Field(min_length=1, max_length=500)]


class GeminiHypothesisOut(BaseModel):
    model_config = ConfigDict(extra="forbid")

    label: str = Field(min_length=1, max_length=160)
    explanation: str = Field(min_length=1, max_length=1000)
    confidence: float = Field(ge=0.0, le=1.0)
    grounded_in: list[RefId] = Field(min_length=1, max_length=20)
    supporting_rule_ids: list[RuleId] = Field(min_length=1, max_length=10)

    @field_validator("grounded_in", "supporting_rule_ids")
    @classmethod
    def _no_duplicates(cls, v: list[str]) -> list[str]:
        if len(set(v)) != len(v):
            raise ValueError("duplicate entries are not allowed")
        return v


class GeminiResponseOut(BaseModel):
    model_config = ConfigDict(extra="forbid")

    outcome: Literal["DIAGNOSED", "UNKNOWN"]
    hypotheses: list[GeminiHypothesisOut] = Field(default_factory=list, max_length=10)
    reasoning_notes: list[Note] = Field(default_factory=list, max_length=10)
    unknown_reason: Literal["PROVIDER_UNCERTAIN"] | None = None

    @model_validator(mode="after")
    def _outcome_consistency(self) -> "GeminiResponseOut":
        if self.outcome == "DIAGNOSED":
            if not self.hypotheses or self.unknown_reason is not None:
                raise ValueError("DIAGNOSED requires >=1 hypothesis and no unknown_reason")
        else:  # UNKNOWN
            if self.hypotheses or self.unknown_reason != "PROVIDER_UNCERTAIN":
                raise ValueError(
                    "UNKNOWN requires zero hypotheses and unknown_reason=PROVIDER_UNCERTAIN"
                )
        return self
