import os
from functools import lru_cache
from typing import Literal

from pydantic import BaseModel


class ProbeSettings(BaseModel):
    ai_provider: Literal["fake", "gemini"] = "fake"
    gemini_api_key: str | None = None
    gemini_model: str = "gemini-flash-latest"
    gemini_timeout_seconds: float = 10.0


@lru_cache
def get_settings() -> ProbeSettings:
    ai_provider = os.environ.get("PROBE_AI_PROVIDER", "fake").lower()
    if ai_provider not in ("fake", "gemini"):
        ai_provider = "fake"
    return ProbeSettings(
        ai_provider=ai_provider,  # type: ignore[arg-type]
        gemini_api_key=os.environ.get("GEMINI_API_KEY") or None,
        gemini_model=os.environ.get("GEMINI_MODEL", "gemini-flash-latest"),
        gemini_timeout_seconds=float(os.environ.get("GEMINI_TIMEOUT_SECONDS", "10")),
    )
