import pytest

from app.catalog import get_default_catalog
from app.planner import TestPlanner
from app.providers.fake import FakeAIProvider
from app.schemas import EvidenceFact
from app.sessions import create_session
from app.telemetry_sink import (
    InMemoryTelemetrySink,
    TelemetryRecord,
    TelemetrySinkUnavailableError,
    TigerTelemetrySink,
    extract_telemetry_records,
)
from tests.conftest import load_evidence

catalog = get_default_catalog()
provider = FakeAIProvider()
planner = TestPlanner()


# ---------------------------------------------------------------------------
# 11-12: extraction correctness + AI_INTERPRETATION guard
# ---------------------------------------------------------------------------


def test_telemetry_extracts_numeric_measurement_facts_correctly() -> None:
    session = create_session(
        load_evidence("hc_sr04_trig_and_echo_missing"), "hc-sr04", catalog, provider, planner
    )
    records = extract_telemetry_records(
        session.session_id, session.steps[0].evaluated_evidence, session.steps[0].created_at_ms
    )
    names = {r.name for r in records}
    assert "pulse_count" in names
    for record in records:
        assert isinstance(record.value, float)
        assert record.provenance in ("MEASURED", "DERIVED", "SPECIFICATION", "BASELINE", "SOFTWARE")


def test_telemetry_never_labels_ai_interpretation_as_measured() -> None:
    sink = InMemoryTelemetrySink()
    bad_record = TelemetryRecord(
        session_id="sess_x", probe="P3", role="ECHO", name="fake_confidence",
        value=0.9, unit=None, provenance="AI_INTERPRETATION", timestamp_ms=0,
    )
    with pytest.raises(ValueError, match="AI_INTERPRETATION"):
        sink.record_measurement(bad_record)
    assert sink.records == []  # rejected, never stored


def test_extract_telemetry_records_never_reads_hypotheses() -> None:
    """Structural check: extract_telemetry_records only accepts a
    StructuredEvidence-shaped object, which has no field capable of holding
    a Hypothesis at all - the ProbeResponse.hypotheses list is a completely
    separate object never passed into this function."""
    import ast
    import inspect
    import textwrap

    tree = ast.parse(textwrap.dedent(inspect.getsource(extract_telemetry_records)))
    accessed_attrs = {node.attr for node in ast.walk(tree) if isinstance(node, ast.Attribute)}
    assert "hypotheses" not in accessed_attrs
    assert "rule_results" not in accessed_attrs  # rules are deterministic outcomes, not raw measurements


# ---------------------------------------------------------------------------
# 13-14: fake Tiger sink + failure isolation
# ---------------------------------------------------------------------------


def test_fake_tiger_sink_receives_expected_time_series_records() -> None:
    sink = InMemoryTelemetrySink()
    record = TelemetryRecord(
        session_id="sess_1", probe="P3", role="ECHO", name="pulse_count",
        value=12.0, unit=None, provenance="MEASURED", timestamp_ms=1234,
    )
    sink.record_measurement(record)
    assert sink.records == [record]


class _FailingConnection:
    def cursor(self):
        raise ConnectionError("simulated Tiger Data outage")


def test_tiger_failure_does_not_become_diagnostic_evidence() -> None:
    """The sink itself raises a typed, clear error - never silently
    succeeds - but callers (app.api's best-effort wrapper) are responsible
    for ensuring this never alters the diagnostic response; this test
    verifies the sink's own contract: it fails loudly, not silently."""
    sink = TigerTelemetrySink("postgresql://fake", connection=_FailingConnection())
    record = TelemetryRecord(
        session_id="sess_1", probe="P3", role="ECHO", name="pulse_count",
        value=0.0, unit=None, provenance="MEASURED", timestamp_ms=1234,
    )
    with pytest.raises(TelemetrySinkUnavailableError):
        sink.record_measurement(record)


def test_tiger_sink_without_psycopg_or_connection_fails_clearly(monkeypatch) -> None:
    import builtins

    real_import = builtins.__import__

    def _blocked_import(name, *args, **kwargs):
        if name == "psycopg":
            raise ImportError("psycopg not installed (simulated)")
        return real_import(name, *args, **kwargs)

    monkeypatch.setattr(builtins, "__import__", _blocked_import)
    sink = TigerTelemetrySink("postgresql://fake")
    record = TelemetryRecord(
        session_id="sess_1", probe="P3", role="ECHO", name="pulse_count",
        value=1.0, unit=None, provenance="MEASURED", timestamp_ms=1,
    )
    with pytest.raises(TelemetrySinkUnavailableError):
        sink.record_measurement(record)
