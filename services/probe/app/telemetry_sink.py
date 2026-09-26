"""Tiger Data (PostgreSQL-compatible time-series) telemetry sink.

Optional and non-authoritative: a TelemetrySink only ever RECEIVES an
already-computed, already-provenanced EvidenceFact for storage/analysis. It
never computes, converts, or judges anything, and its failure must never
alter or block the actual diagnostic result - see app/api.py's best-effort,
exception-swallowed call sites.
"""

import logging
from typing import Protocol

from pydantic import BaseModel

from app.schemas import EvidenceProvenance

logger = logging.getLogger("app.telemetry_sink")

# AI-provenanced content is a hypothesis/explanation, never a measurement.
# Structurally, TelemetryRecord.provenance below is typed with this narrowed
# Literal via EvidenceProvenance minus AI_INTERPRETATION would be ideal, but
# Pydantic can't easily subtract a Literal member, so this is enforced by a
# runtime guard in record_measurement() instead - see _reject_ai_provenance.
_MEASUREMENT_PROVENANCES = {"MEASURED", "DERIVED", "SPECIFICATION", "BASELINE", "SOFTWARE"}


class TelemetrySinkUnavailableError(Exception):
    """Raised by a real sink on connection/write failure. Callers (app.api)
    catch this, log it, and continue - telemetry is optional history, never
    a gate on the actual diagnostic response."""


class TelemetryRecord(BaseModel):
    session_id: str
    probe: str
    role: str
    name: str
    value: float
    unit: str | None
    provenance: EvidenceProvenance
    timestamp_ms: int


def _reject_ai_provenance(record: TelemetryRecord) -> None:
    if record.provenance not in _MEASUREMENT_PROVENANCES:
        raise ValueError(
            f"Refusing to record a '{record.provenance}' fact as telemetry - only "
            f"{sorted(_MEASUREMENT_PROVENANCES)} may ever be written to the "
            "measurement time-series sink."
        )


class TelemetrySink(Protocol):
    def record_measurement(self, record: TelemetryRecord) -> None: ...


class NullTelemetrySink:
    """Default: does nothing. TELEMETRY_SINK=none."""

    def record_measurement(self, record: TelemetryRecord) -> None:
        _reject_ai_provenance(record)


class InMemoryTelemetrySink:
    """Offline test fake - never touches a network."""

    def __init__(self) -> None:
        self.records: list[TelemetryRecord] = []

    def record_measurement(self, record: TelemetryRecord) -> None:
        _reject_ai_provenance(record)
        self.records.append(record)


class TigerTelemetrySink:
    """Real adapter over a PostgreSQL-compatible (Tiger Data/Timescale)
    connection. Accepts an injected, duck-typed `connection` (anything with
    `.cursor()` returning a context manager with `.execute()`, plus
    `.commit()` - the same shape `psycopg`'s Connection already has) for
    testability, mirroring every other real-client adapter in this service.
    """

    _TABLE_DDL = (
        "CREATE TABLE IF NOT EXISTS probe_measurements ("
        "session_id TEXT, probe TEXT, role TEXT, name TEXT, value DOUBLE PRECISION, "
        "unit TEXT, provenance TEXT, timestamp_ms BIGINT)"
    )
    _INSERT_SQL = (
        "INSERT INTO probe_measurements "
        "(session_id, probe, role, name, value, unit, provenance, timestamp_ms) "
        "VALUES (%s, %s, %s, %s, %s, %s, %s, %s)"
    )

    def __init__(self, database_url: str, connection: object | None = None):
        self._database_url = database_url
        self._connection = connection

    def _ensure_connection(self):
        if self._connection is not None:
            return self._connection
        try:
            import psycopg
        except Exception as exc:  # noqa: BLE001
            raise TelemetrySinkUnavailableError(
                "psycopg is not installed; TELEMETRY_SINK=tiger requires it"
            ) from exc
        try:
            self._connection = psycopg.connect(self._database_url)
            with self._connection.cursor() as cur:
                cur.execute(self._TABLE_DDL)
            self._connection.commit()
        except Exception as exc:  # noqa: BLE001
            raise TelemetrySinkUnavailableError(f"Tiger Data connection failed: {type(exc).__name__}") from exc
        return self._connection

    def record_measurement(self, record: TelemetryRecord) -> None:
        _reject_ai_provenance(record)
        connection = self._ensure_connection()
        try:
            with connection.cursor() as cur:
                cur.execute(
                    self._INSERT_SQL,
                    (
                        record.session_id,
                        record.probe,
                        record.role,
                        record.name,
                        record.value,
                        record.unit,
                        record.provenance,
                        record.timestamp_ms,
                    ),
                )
            connection.commit()
        except Exception as exc:  # noqa: BLE001
            logger.error("tiger_telemetry_write_failed: %s", type(exc).__name__)
            raise TelemetrySinkUnavailableError(f"Tiger Data write failed: {type(exc).__name__}") from exc


def extract_telemetry_records(session_id: str, evidence, timestamp_ms: int) -> list[TelemetryRecord]:
    """Pure helper: pulls only numeric MEASURED/DERIVED-family facts out of
    an (evaluated) StructuredEvidence for telemetry recording. Never reads
    evidence.rule_results or any Hypothesis - only the typed, unit-carrying
    fact lists that already exclude AI_INTERPRETATION by construction."""
    records: list[TelemetryRecord] = []
    all_facts = [
        *(evidence.measurements or []),
        *(evidence.derived_facts or []),
        *(evidence.specification_results or []),
        *(evidence.baseline_comparison or []),
    ]
    for fact in all_facts:
        if not isinstance(fact.value, (int, float)) or isinstance(fact.value, bool):
            continue
        records.append(
            TelemetryRecord(
                session_id=session_id,
                probe=fact.probe or evidence.probe,
                role=evidence.role,
                name=fact.name,
                value=float(fact.value),
                unit=fact.unit,
                provenance=fact.provenance,
                timestamp_ms=timestamp_ms,
            )
        )
    return records
