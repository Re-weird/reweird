from app.catalog import get_default_catalog
from app.export import build_diagnostic_export
from app.patch_provider import FakePatchProposalProvider
from app.patch_proposals import create_patch_proposal
from app.planner import TestPlanner
from app.providers.fake import FakeAIProvider
from app.schemas import PatchProposalRequest
from app.sessions import create_session, submit_evidence
from tests.conftest import load_evidence

catalog = get_default_catalog()
provider = FakeAIProvider()
planner = TestPlanner()
patch_provider = FakePatchProposalProvider()


# ---------------------------------------------------------------------------
# 18: JSON export preserves history
# ---------------------------------------------------------------------------


def test_export_preserves_full_history() -> None:
    session = create_session(
        load_evidence("hc_sr04_trig_and_echo_missing"), "hc-sr04", catalog, provider, planner
    )
    session, _ = submit_evidence(session, load_evidence("hc_sr04_healthy"), catalog, provider, planner)
    proposal = create_patch_proposal(
        session,
        PatchProposalRequest(
            source_step_number=1, target_probe="P2", target_role="TRIG", patch_type="TEMPORARY_SIGNAL_EMULATION"
        ),
        patch_provider,
    )

    export = build_diagnostic_export(session, [proposal])

    assert export["metadata"]["session_id"] == session.session_id
    assert export["metadata"]["component_id"] == "hc-sr04"
    assert len(export["timeline"]) == 2
    assert export["timeline"][0]["step_number"] == 1
    assert export["timeline"][1]["step_number"] == 2
    assert export["timeline"][0]["evaluated_evidence"]["rule_results"]
    assert export["patch_proposals"][0]["proposal_id"] == proposal.proposal_id
    assert export["final_status"] == session.status


# ---------------------------------------------------------------------------
# 19: export contains no configured secrets
# ---------------------------------------------------------------------------


def test_export_never_contains_configured_secrets() -> None:
    session = create_session(
        load_evidence("hc_sr04_trig_and_echo_missing"), "hc-sr04", catalog, provider, planner
    )
    export = build_diagnostic_export(session, [])
    serialized = str(export)
    for secret_looking_value in ("sk-super-secret-gemini-key", "hunter2-db-password", "mongodb+srv://user:pass@"):
        assert secret_looking_value not in serialized


def test_export_redacts_secret_shaped_keys_defensively() -> None:
    from app.export import _redact_secrets

    poisoned = {
        "gemini_api_key": "sk-should-never-appear",
        "database_password": "hunter2",
        "authorization": "Bearer abc123",
        "nested": {"api_key": "also-secret", "safe_field": "keep-me"},
        "safe_top_level": "keep-me-too",
    }
    redacted = _redact_secrets(poisoned)
    assert redacted["gemini_api_key"] == "[REDACTED]"
    assert redacted["database_password"] == "[REDACTED]"
    assert redacted["authorization"] == "[REDACTED]"
    assert redacted["nested"]["api_key"] == "[REDACTED]"
    assert redacted["nested"]["safe_field"] == "keep-me"
    assert redacted["safe_top_level"] == "keep-me-too"
