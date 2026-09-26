import json

from fastapi.testclient import TestClient

from app.api import app, get_code_provider, get_vision_provider
from tests.conftest import load_code_fixture


class _FakeCodeClient:
    def __init__(self, response_text: str):
        self._response_text = response_text

    def generate(self, system_instruction: str, user_content: dict) -> str:
        return self._response_text


class _FakeCodeProvider:
    """Wraps CodeInterpretationProvider's real preflight+validation logic
    but injects a fake network client, so /understand exercises the full
    pipeline without ever touching a real Gemini endpoint."""

    def __init__(self, response_text: str):
        from app.config import UnderstandSettings
        from app.code_analysis.provider import CodeInterpretationProvider

        self._inner = CodeInterpretationProvider(
            UnderstandSettings(ai_provider="gemini", gemini_api_key=None),
            client=_FakeCodeClient(response_text),
        )

    def interpret(self, facts, catalog):
        return self._inner.interpret(facts, catalog)


def _healthy_response_text(trig_ref: str) -> str:
    return json.dumps(
        {
            "outcome": "INTERPRETED",
            "controller": "ESP32",
            "expected_behavior_summary": "Measures distance via ultrasonic trigger/echo.",
            "component_candidates": [
                {
                    "catalog_id": "hc-sr04",
                    "confidence": 0.85,
                    "grounded_in": [trig_ref],
                    "rationale": "TRIG/ECHO pin pattern.",
                }
            ],
            "role_candidates": [],
            "reasoning_notes": [],
        }
    )


def _trig_ref_for(source_name: str) -> str:
    from app.code_analysis.extractor import analyze_files

    result = analyze_files({source_name: load_code_fixture(source_name)})
    return next(f.ref_id for f in result.facts if f.kind == "pin_constant" and f.symbol == "TRIG_PIN")


def test_post_understand_code_only_returns_proposed(client: TestClient) -> None:
    trig_ref = _trig_ref_for("healthy_ultrasonic.cpp")
    app.dependency_overrides[get_code_provider] = lambda: _FakeCodeProvider(
        _healthy_response_text(trig_ref)
    )
    try:
        response = client.post(
            "/understand",
            json={
                "files": [
                    {
                        "path": "healthy_ultrasonic.cpp",
                        "content": load_code_fixture("healthy_ultrasonic.cpp").decode(),
                    }
                ]
            },
        )
        assert response.status_code == 200
        body = response.json()
        assert body["status"] == "PROPOSED"
        assert body["project"]["controller"] == "ESP32"
        assert body["project"]["components"][0]["catalog_id"] == "hc-sr04"
        assert body["vision_analysis"] is None
        print(json.dumps(body, indent=2))
    finally:
        app.dependency_overrides.pop(get_code_provider, None)


def test_post_understand_unsupported_language_only_is_insufficient_information(
    client: TestClient,
) -> None:
    app.dependency_overrides[get_code_provider] = lambda: _FakeCodeProvider(
        "should never be called"
    )
    try:
        response = client.post(
            "/understand",
            json={
                "files": [
                    {
                        "path": "unsupported_language.py",
                        "content": load_code_fixture("unsupported_language.py").decode(),
                    }
                ]
            },
        )
        assert response.status_code == 200
        body = response.json()
        assert body["status"] == "INSUFFICIENT_INFORMATION"
        assert body["project"] is None
    finally:
        app.dependency_overrides.pop(get_code_provider, None)


def test_post_understand_malformed_request_body_returns_422(client: TestClient) -> None:
    response = client.post("/understand", json={})
    assert response.status_code == 422


def test_post_understand_rejects_too_many_source_files(client: TestClient) -> None:
    response = client.post(
        "/understand",
        json={"files": [{"path": f"file-{index}.ino", "content": "x"} for index in range(9)]},
    )
    assert response.status_code == 422


def test_post_understand_rejects_oversized_and_malformed_images(client: TestClient) -> None:
    import base64

    oversized = base64.b64encode(b"x" * (5 * 1024 * 1024 + 1)).decode()
    response = client.post("/understand", json={"files": [], "image": {"mime_type": "image/png", "data_base64": oversized}})
    assert response.status_code == 422
    response = client.post("/understand", json={"files": [], "image": {"mime_type": "image/png", "data_base64": "not-base64"}})
    assert response.status_code == 422


def test_post_understand_rejects_body_over_8_mib(client: TestClient) -> None:
    response = client.post("/understand", content=b"x" * (8 * 1024 * 1024 + 1))
    assert response.status_code == 413
