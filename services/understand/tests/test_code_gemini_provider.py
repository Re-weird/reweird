import json

import pytest

from app.catalog import get_default_catalog
from app.code_analysis.extractor import analyze_files
from app.code_analysis.provider import CodeInterpretationProvider
from app.config import UnderstandSettings
from tests.conftest import load_code_fixture

_SETTINGS = UnderstandSettings(ai_provider="gemini", gemini_api_key=None)


class _CountingClient:
    def __init__(self, response_text: str | None = None, exc: Exception | None = None):
        self.calls = 0
        self._response_text = response_text
        self._exc = exc

    def generate(self, system_instruction: str, user_content: dict) -> str:
        self.calls += 1
        if self._exc is not None:
            raise self._exc
        assert self._response_text is not None
        return self._response_text


def _facts_and_catalog():
    files = {"healthy_ultrasonic.cpp": load_code_fixture("healthy_ultrasonic.cpp")}
    result = analyze_files(files)
    return result.facts, get_default_catalog()


def test_no_facts_never_calls_the_client() -> None:
    client = _CountingClient(response_text="should never be used")
    provider = CodeInterpretationProvider(_SETTINGS, client=client)

    result = provider.interpret([], get_default_catalog())

    assert result.outcome == "UNKNOWN"
    assert result.unknown_reason == "INSUFFICIENT_INFORMATION"
    assert client.calls == 0


def test_well_formed_response_produces_interpreted_result() -> None:
    facts, catalog = _facts_and_catalog()
    trig_ref = next(f.ref_id for f in facts if f.kind == "pin_constant" and f.symbol == "TRIG_PIN")
    response_text = json.dumps(
        {
            "outcome": "INTERPRETED",
            "component_candidates": [
                {
                    "catalog_id": "hc-sr04",
                    "confidence": 0.8,
                    "grounded_in": [trig_ref],
                    "rationale": "TRIG/ECHO pattern",
                }
            ],
            "role_candidates": [],
            "reasoning_notes": [],
        }
    )
    client = _CountingClient(response_text=response_text)
    provider = CodeInterpretationProvider(_SETTINGS, client=client)

    result = provider.interpret(facts, catalog)

    assert result.outcome == "INTERPRETED"
    assert client.calls == 1
    assert result.components[0].catalog_id == "hc-sr04"


def test_timeout_resolves_to_provider_error() -> None:
    facts, catalog = _facts_and_catalog()
    client = _CountingClient(exc=TimeoutError("timed out"))
    provider = CodeInterpretationProvider(_SETTINGS, client=client)

    result = provider.interpret(facts, catalog)

    assert result.outcome == "UNKNOWN"
    assert result.unknown_reason == "PROVIDER_ERROR"


def test_generic_exception_does_not_leak_message() -> None:
    facts, catalog = _facts_and_catalog()
    secret_looking = "auth failed for key sk-super-secret-123"
    client = _CountingClient(exc=RuntimeError(secret_looking))
    provider = CodeInterpretationProvider(_SETTINGS, client=client)

    result = provider.interpret(facts, catalog)

    assert result.outcome == "UNKNOWN"
    assert result.unknown_reason == "PROVIDER_ERROR"
    assert secret_looking not in " ".join(result.reasoning_notes)


def test_non_json_response_resolves_to_provider_error() -> None:
    facts, catalog = _facts_and_catalog()
    client = _CountingClient(response_text="not json {{{")
    provider = CodeInterpretationProvider(_SETTINGS, client=client)

    result = provider.interpret(facts, catalog)

    assert result.outcome == "UNKNOWN"
    assert result.unknown_reason == "PROVIDER_ERROR"


def test_construction_without_api_key_or_client_raises() -> None:
    with pytest.raises(RuntimeError):
        CodeInterpretationProvider(UnderstandSettings(ai_provider="gemini", gemini_api_key=None))


def test_construction_with_injected_client_never_requires_api_key() -> None:
    provider = CodeInterpretationProvider(
        UnderstandSettings(ai_provider="gemini", gemini_api_key=None),
        client=_CountingClient(response_text="{}"),
    )
    assert provider is not None
