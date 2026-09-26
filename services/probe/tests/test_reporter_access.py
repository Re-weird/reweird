from app.api import get_repository
from app.patch_proposals import create_patch_proposal
from app.patch_provider import FakePatchProposalProvider
from app.schemas import PatchProposalRequest

from tests.conftest import load_evidence
from app.catalog import get_default_catalog
from app.planner import TestPlanner
from app.providers.fake import FakeAIProvider
from app.sessions import create_session


def test_external_result_requires_configured_authenticated_reporter(client, monkeypatch) -> None:
    session = create_session(load_evidence("hc_sr04_trig_and_echo_missing"), "hc-sr04", get_default_catalog(), FakeAIProvider(), TestPlanner())
    proposal = create_patch_proposal(
        session,
        PatchProposalRequest(target_probe="P2", target_role="TRIG", patch_type="TEMPORARY_SIGNAL_EMULATION"),
        FakePatchProposalProvider(),
    )
    repository = get_repository()
    repository.save_session(session)
    repository.save_patch_proposal(proposal)
    path = f"/sessions/{session.session_id}/patch-proposals/{proposal.proposal_id}/result"

    monkeypatch.delenv("PROBE_REPORTER_TOKEN", raising=False)
    monkeypatch.delenv("PROBE_REPORTER_ID", raising=False)
    payload = {"external_status": "APPROVED_EXTERNALLY", "actor_id": "forged-admin"}
    assert client.post(path, json=payload).status_code == 503

    monkeypatch.setenv("PROBE_REPORTER_TOKEN", "test-reporter-token-with-32-plus-characters")
    monkeypatch.setenv("PROBE_REPORTER_ID", "authenticated-operator")
    assert client.post(path, json=payload).status_code == 401
    assert client.post(path, json=payload, headers={"Authorization": "Bearer wrong"}).status_code == 401
    assert repository.get_patch_proposal(proposal.proposal_id).status == "PROPOSED"

    response = client.post(path, json=payload, headers={"Authorization": "Bearer test-reporter-token-with-32-plus-characters"})
    assert response.status_code == 200
    action = response.json()["action_history"][0]
    assert action["actor_id"] == "authenticated-operator"
    assert action["record_source"] == "HUMAN_REPORTED"
    assert action["recorded_at_ms"] > 0
    assert action["execution_verified"] is False
    event = repository.list_events(session.session_id)[-1]
    assert event.reference["actor_id"] == "authenticated-operator"
    assert event.reference["record_source"] == "HUMAN_REPORTED"
    assert event.reference["execution_verified"] is False
