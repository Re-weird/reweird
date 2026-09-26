import os
from functools import lru_cache
from typing import Literal

from pydantic import BaseModel


class ProbeSettings(BaseModel):
    ai_provider: Literal["fake", "gemini"] = "fake"
    gemini_api_key: str | None = None
    gemini_model: str = "gemini-flash-latest"
    gemini_timeout_seconds: float = 10.0

    # Milestone 7: diagnostic history persistence. Safe by default - a
    # developer can clone this repo and run the full test suite without
    # MongoDB, Tiger Data, Snowflake, or any network access at all.
    diagnostic_repository: Literal["memory", "mongodb"] = "memory"
    mongodb_uri: str | None = None
    mongodb_database: str = "reweird_probe"

    telemetry_sink: Literal["none", "tiger"] = "none"
    tiger_database_url: str | None = None

    analytics_sink: Literal["none", "snowflake"] = "none"
    snowflake_account: str | None = None
    snowflake_user: str | None = None
    snowflake_password: str | None = None
    snowflake_database: str | None = None
    snowflake_warehouse: str | None = None


def _literal_env(name: str, default: str, allowed: tuple[str, ...]) -> str:
    value = os.environ.get(name, default).lower()
    return value if value in allowed else default


@lru_cache
def get_settings() -> ProbeSettings:
    ai_provider = _literal_env("PROBE_AI_PROVIDER", "fake", ("fake", "gemini"))
    diagnostic_repository = _literal_env("DIAGNOSTIC_REPOSITORY", "memory", ("memory", "mongodb"))
    telemetry_sink = _literal_env("TELEMETRY_SINK", "none", ("none", "tiger"))
    analytics_sink = _literal_env("ANALYTICS_SINK", "none", ("none", "snowflake"))

    return ProbeSettings(
        ai_provider=ai_provider,  # type: ignore[arg-type]
        gemini_api_key=os.environ.get("GEMINI_API_KEY") or None,
        gemini_model=os.environ.get("GEMINI_MODEL", "gemini-flash-latest"),
        gemini_timeout_seconds=float(os.environ.get("GEMINI_TIMEOUT_SECONDS", "10")),
        diagnostic_repository=diagnostic_repository,  # type: ignore[arg-type]
        mongodb_uri=os.environ.get("MONGODB_URI") or None,
        mongodb_database=os.environ.get("MONGODB_DATABASE", "reweird_probe"),
        telemetry_sink=telemetry_sink,  # type: ignore[arg-type]
        tiger_database_url=os.environ.get("TIGER_DATABASE_URL") or None,
        analytics_sink=analytics_sink,  # type: ignore[arg-type]
        snowflake_account=os.environ.get("SNOWFLAKE_ACCOUNT") or None,
        snowflake_user=os.environ.get("SNOWFLAKE_USER") or None,
        snowflake_password=os.environ.get("SNOWFLAKE_PASSWORD") or None,
        snowflake_database=os.environ.get("SNOWFLAKE_DATABASE") or None,
        snowflake_warehouse=os.environ.get("SNOWFLAKE_WAREHOUSE") or None,
    )
