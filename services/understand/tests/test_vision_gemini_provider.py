import json

import pytest

from app.catalog import get_default_catalog
from app.config import UnderstandSettings
from app.vision.provider import VisionInterpretationProvider

_SETTINGS = UnderstandSettings(ai_provider="gemini", gemini_api_key=None)
_TINY_PNG = bytes.fromhex(
    "89504e470d0a1a0a0000000d49484452000000010000000108020000009077"
    "3df80000000a49444154789c6360000002000155a723d00000000049454e44ae426082"
)


class _FakeVisionClient:
    def __init__(self, response_text: str | None = None, exc: Exception | None = None):
        self.calls = 0
        self._response_text = response_text
        self._exc = exc

    def generate(self, system_instruction, image_bytes, mime_type, catalog) -> str:
        self.calls += 1
        if self._exc is not None:
            raise self._exc
        assert self._response_text is not None
        return self._response_text


def test_well_formed_response_produces_interpreted_result() -> None:
    response_text = json.dumps(
        {
            "outcome": "INTERPRETED",
            "component_candidates": [
                {"catalog_id": "hc-sr04", "confidence": 0.6, "rationale": "Visible sensor."}
            ],
            "reasoning_notes": [],
        }
    )
    client = _FakeVisionClient(response_text=response_text)
    provider = VisionInterpretationProvider(_SETTINGS, client=client)

    result = provider.interpret(_TINY_PNG, "image/png", get_default_catalog())

    assert result.outcome == "INTERPRETED"
    assert client.calls == 1
    assert result.component_candidates[0].catalog_id == "hc-sr04"


def test_unsupported_mime_type_never_calls_the_client() -> None:
    client = _FakeVisionClient(response_text="should never be used")
    provider = VisionInterpretationProvider(_SETTINGS, client=client)

    result = provider.interpret(_TINY_PNG, "application/pdf", get_default_catalog())

    assert result.outcome == "UNKNOWN"
    assert client.calls == 0


def test_oversized_image_never_calls_the_client() -> None:
    client = _FakeVisionClient(response_text="should never be used")
    provider = VisionInterpretationProvider(_SETTINGS, client=client)

    oversized = b"0" * (9 * 1024 * 1024)
    result = provider.interpret(oversized, "image/png", get_default_catalog())

    assert result.outcome == "UNKNOWN"
    assert client.calls == 0


def test_timeout_resolves_to_provider_error() -> None:
    client = _FakeVisionClient(exc=TimeoutError("timed out"))
    provider = VisionInterpretationProvider(_SETTINGS, client=client)

    result = provider.interpret(_TINY_PNG, "image/png", get_default_catalog())

    assert result.outcome == "UNKNOWN"
    assert result.unknown_reason == "PROVIDER_ERROR"


def test_generic_exception_does_not_leak_message() -> None:
    secret_looking = "auth failed for key sk-super-secret-123"
    client = _FakeVisionClient(exc=RuntimeError(secret_looking))
    provider = VisionInterpretationProvider(_SETTINGS, client=client)

    result = provider.interpret(_TINY_PNG, "image/png", get_default_catalog())

    assert secret_looking not in " ".join(result.reasoning_notes)


def test_construction_without_api_key_or_client_raises() -> None:
    with pytest.raises(RuntimeError):
        VisionInterpretationProvider(UnderstandSettings(ai_provider="gemini", gemini_api_key=None))
