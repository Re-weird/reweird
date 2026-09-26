import pytest

from app.analytics_sink import (
    AnalyticsSinkUnavailableError,
    InMemoryAnalyticsSink,
    SnowflakeAnalyticsSink,
    build_session_summary,
)
from app.catalog import get_default_catalog
from app.patch_provider import FakePatchProposalProvider
from app.patch_proposals import create_patch_proposal, record_external_result, verify_proposal
from app.planner import TestPlanner
from app.providers.fake import FakeAIProvider
from app.schemas import PatchProposalRequest, RecordExternalResultRequest
from app.sessions import create_session
from tests.conftest import load_evidence

catalog = get_default_catalog()
provider = FakeAIProvider()
planner = TestPlanner()
patch_provider = FakePatchProposalProvider()


# ---------------------------------------------------------------------------
# 15-16: safe summary fields + fake sink receives it
# ---------------------------------------------------------------------------


def test_analytics_summary_contains_expected_safe_fields() -> None:
    session = create_session(
        load_evidence("hc_sr04_trig_and_echo_missing"), "hc-sr04", catalog, provider, planner
    )
    summary = build_session_summary(session, proposals=[])

    assert summary.session_id == session.session_id
    assert summary.component_id == "hc-sr04"
    assert summary.probe == "P3"
    assert summary.role == "ECHO"
    assert summary.step_count == 1
    assert summary.final_status == session.status
    assert summary.rule_failure_count >= 1
    assert summary.patch_proposed is False
    assert summary.verification_outcome is None
    # No raw evidence, no source code, no free-text explanations anywhere.
    dumped = summary.model_dump()
    assert "measurements" not in dumped
    assert "explanation" not in dumped
    assert "hypotheses" not in dumped


def test_fake_snowflake_sink_receives_session_and_verification_summary() -> None:
    session = create_session(
        load_evidence("hc_sr04_trig_and_echo_missing"), "hc-sr04", catalog, provider, planner
    )
    proposal = create_patch_proposal(
        session,
        PatchProposalRequest(target_probe="P2", target_role="TRIG", patch_type="TEMPORARY_SIGNAL_EMULATION"),
        patch_provider,
    )
    approved = record_external_result(proposal, RecordExternalResultRequest(external_status="APPROVED_EXTERNALLY"))
    executed = record_external_result(approved, RecordExternalResultRequest(external_status="EXECUTED_EXTERNALLY"))
    _, verified_proposal, verification, _ = verify_proposal(
        session, executed, load_evidence("hc_sr04_healthy"), catalog, provider, planner
    )

    summary = build_session_summary(session, proposals=[verified_proposal])
    assert summary.patch_proposed is True
    assert summary.verification_outcome == "SUPPORTED"

    sink = InMemoryAnalyticsSink()
    sink.record_session_summary(summary)
    assert sink.summaries == [summary]


# ---------------------------------------------------------------------------
# 17: Snowflake failure never alters the deterministic diagnosis
# ---------------------------------------------------------------------------


class _FailingConnection:
    def cursor(self):
        raise ConnectionError("simulated Snowflake outage")


def test_snowflake_write_failure_does_not_alter_deterministic_diagnosis() -> None:
    session = create_session(
        load_evidence("hc_sr04_trig_and_echo_missing"), "hc-sr04", catalog, provider, planner
    )
    outcome_before = session.steps[0].result.outcome
    rules_before = [r.model_copy() for r in session.steps[0].evaluated_evidence.rule_results]

    sink = SnowflakeAnalyticsSink(
        account="a", user="u", password="p", database="d", warehouse="w", connection=_FailingConnection()
    )
    summary = build_session_summary(session, proposals=[])
    with pytest.raises(AnalyticsSinkUnavailableError):
        sink.record_session_summary(summary)

    # The session/diagnosis object itself is completely untouched by the
    # sink failure - nothing about it could have changed, since the sink
    # never receives a reference capable of mutating it.
    assert session.steps[0].result.outcome == outcome_before
    assert session.steps[0].evaluated_evidence.rule_results == rules_before


def test_snowflake_sink_without_package_or_connection_fails_clearly(monkeypatch) -> None:
    import builtins

    real_import = builtins.__import__

    def _blocked_import(name, *args, **kwargs):
        if name == "snowflake.connector" or name.startswith("snowflake"):
            raise ImportError("snowflake.connector not installed (simulated)")
        return real_import(name, *args, **kwargs)

    monkeypatch.setattr(builtins, "__import__", _blocked_import)
    with pytest.raises(AnalyticsSinkUnavailableError, match="not installed"):
        SnowflakeAnalyticsSink(account="a", user="u", password="p", database="d", warehouse="w")
