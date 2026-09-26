import json

from app.config import get_settings
from tests.conftest import FIXTURES_DIR, load_evidence


def _payload(fixture_name: str, component_id: str | None = None) -> dict:
    body = {"evidence": json.loads((FIXTURES_DIR / f"{fixture_name}.json").read_text())}
    if component_id is not None:
        body["component_id"] = component_id
    return body


# ---------------------------------------------------------------------------
# 20: session list/history works
# ---------------------------------------------------------------------------


def test_list_sessions_and_get_session_history(client) -> None:
    create_response = client.post("/sessions", json=_payload("hc_sr04_not_evaluable", "hc-sr04"))
    assert create_response.status_code == 200
    session_id = create_response.json()["session_id"]

    list_response = client.get("/sessions")
    assert list_response.status_code == 200
    ids = [s["session_id"] for s in list_response.json()]
    assert session_id in ids

    get_response = client.get(f"/sessions/{session_id}")
    assert get_response.status_code == 200
    assert len(get_response.json()["steps"]) == 1


def test_get_session_export_endpoint(client) -> None:
    create_response = client.post("/sessions", json=_payload("hc_sr04_trig_and_echo_missing", "hc-sr04"))
    session_id = create_response.json()["session_id"]

    export_response = client.get(f"/sessions/{session_id}/export")
    assert export_response.status_code == 200
    body = export_response.json()
    assert body["metadata"]["session_id"] == session_id
    assert body["timeline"][0]["step_number"] == 1
    assert "final_status" in body


def test_health_endpoint_reports_safe_defaults(client) -> None:
    get_settings.cache_clear()
    response = client.get("/health")
    assert response.status_code == 200
    body = response.json()
    assert body["status"] == "ok"
    assert body["diagnostic_repository"] == "memory"
    assert body["telemetry_sink"] == "none"
    assert body["analytics_sink"] == "none"
    get_settings.cache_clear()


# ---------------------------------------------------------------------------
# 21-23: M5/M6/existing POST /probe unchanged through the new repository wiring
# ---------------------------------------------------------------------------


def test_m5_closed_loop_behavior_unchanged_through_repository(client) -> None:
    create_response = client.post("/sessions", json=_payload("hc_sr04_not_evaluable", "hc-sr04"))
    session_id = create_response.json()["session_id"]

    evidence_response = client.post(
        f"/sessions/{session_id}/evidence", json={"evidence": json.loads((FIXTURES_DIR / "hc_sr04_missing_echo_activity.json").read_text())}
    )
    assert evidence_response.status_code == 200
    body = evidence_response.json()
    assert body["duplicate"] is False
    assert len(body["session"]["steps"]) == 2
    assert body["session"]["steps"][1]["result"]["hypotheses"][0]["supporting_rule_ids"] == ["missing-signal"]

    stop_response = client.post(f"/sessions/{session_id}/stop")
    assert stop_response.status_code == 200
    assert stop_response.json()["status"] == "STOPPED"

    rejected = client.post(f"/sessions/{session_id}/evidence", json={"evidence": json.loads((FIXTURES_DIR / "hc_sr04_healthy.json").read_text())})
    assert rejected.status_code == 409


def test_m6_patch_and_verify_behavior_unchanged_through_repository(client, monkeypatch) -> None:
    monkeypatch.setenv("PROBE_REPORTER_TOKEN", "test-reporter-token-with-32-plus-characters")
    monkeypatch.setenv("PROBE_REPORTER_ID", "test-operator")
    reporter_headers = {"Authorization": "Bearer test-reporter-token-with-32-plus-characters"}
    create_response = client.post("/sessions", json=_payload("hc_sr04_trig_and_echo_missing", "hc-sr04"))
    session_id = create_response.json()["session_id"]

    proposal_response = client.post(
        f"/sessions/{session_id}/patch-proposals",
        json={"target_probe": "P2", "target_role": "TRIG", "patch_type": "TEMPORARY_SIGNAL_EMULATION"},
    )
    assert proposal_response.status_code == 200
    proposal_id = proposal_response.json()["proposal"]["proposal_id"]

    client.post(f"/sessions/{session_id}/patch-proposals/{proposal_id}/result", json={"external_status": "APPROVED_EXTERNALLY"}, headers=reporter_headers)
    exec_response = client.post(f"/sessions/{session_id}/patch-proposals/{proposal_id}/result", json={"external_status": "EXECUTED_EXTERNALLY"}, headers=reporter_headers)
    assert exec_response.json()["status"] == "EXECUTED_EXTERNALLY"
    assert [action["reported_status"] for action in exec_response.json()["action_history"]] == ["APPROVED_EXTERNALLY", "EXECUTED_EXTERNALLY"]
    assert all(action["actor_id"] == "test-operator" and action["record_source"] == "HUMAN_REPORTED" and action["execution_verified"] is False for action in exec_response.json()["action_history"])
    assert exec_response.json()["action_history"][-1]["display_label"] == "User reported action completed"

    verify_response = client.post(
        f"/sessions/{session_id}/patch-proposals/{proposal_id}/verify",
        json={"evidence": json.loads((FIXTURES_DIR / "hc_sr04_healthy.json").read_text())},
    )
    assert verify_response.status_code == 200
    body = verify_response.json()
    assert body["verification"]["outcome"] == "SUPPORTED"
    assert body["proposal"]["status"] == "VERIFIED"


def test_post_probe_remains_unchanged(client) -> None:
    evidence = load_evidence("healthy")
    response = client.post("/probe", json={"evidence": evidence.model_dump(mode="json")})
    assert response.status_code == 200
    assert response.json()["outcome"] == "DIAGNOSED"


# ---------------------------------------------------------------------------
# 25: default config uses memory/no external sinks
# ---------------------------------------------------------------------------


def test_default_settings_use_memory_and_no_sinks(monkeypatch) -> None:
    get_settings.cache_clear()
    monkeypatch.delenv("DIAGNOSTIC_REPOSITORY", raising=False)
    monkeypatch.delenv("TELEMETRY_SINK", raising=False)
    monkeypatch.delenv("ANALYTICS_SINK", raising=False)
    settings = get_settings()
    assert settings.diagnostic_repository == "memory"
    assert settings.telemetry_sink == "none"
    assert settings.analytics_sink == "none"
    get_settings.cache_clear()
