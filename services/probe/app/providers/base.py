from typing import Literal, Protocol

from pydantic import BaseModel, Field

from app.schemas import GroundedItem, Hypothesis, StructuredEvidence, UnknownReason


class AIProviderResult(BaseModel):
    outcome: Literal["DIAGNOSED", "UNKNOWN"]
    hypotheses: list[Hypothesis]
    reasoning_notes: list[str] = Field(default_factory=list)
    unknown_reason: UnknownReason | None = None


class AIProvider(Protocol):
    def interpret(
        self, evidence: StructuredEvidence, grounded: list[GroundedItem]
    ) -> AIProviderResult: ...
