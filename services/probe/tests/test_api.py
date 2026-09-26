import json

from fastapi.testclient import TestClient

from tests.conftest import FIXTURES_DIR


def _fixture_payload(name: str) -> dict:
    return {"evidence": json.loads((FIXTURES_DIR / f"{name}.json").read_text())}


def test_post_probe_intermittent_echo_end_to_end(client: TestClient) -> None:
    response = client.post("/probe", json=_fixture_payload("intermittent_echo"))
    assert response.status_code == 200

    body = response.json()
    assert body["outcome"] == "DIAGNOSED"
    assert body["probe"] == "P3"
    assert body["role"] == "ECHO"
    assert len(body["hypotheses"]) == 1
    assert body["hypotheses"][0]["supporting_rule_ids"] == ["unexpected-dropout"]
    assert "movement correlation" in body["recommended_test"].lower()
    print(json.dumps(body, indent=2))


def test_post_probe_missing_evidence_field_returns_422(client: TestClient) -> None:
    response = client.post("/probe", json={})
    assert response.status_code == 422


def test_post_probe_conflicting_evidence_returns_unknown(client: TestClient) -> None:
    response = client.post("/probe", json=_fixture_payload("conflicting_evidence"))
    assert response.status_code == 200

    body = response.json()
    assert body["outcome"] == "UNKNOWN"
    assert body["unknown_reason"] == "CONFLICTING_RULES"
    assert body["hypotheses"] == []
