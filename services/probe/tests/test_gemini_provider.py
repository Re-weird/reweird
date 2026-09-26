import json

import pytest

from app.config import ProbeSettings
from app.providers.gemini import GeminiAIProvider
from tests.conftest import load_evidence

_SETTINGS = ProbeSettings(ai_provider="gemini", gemini_api_key=None)


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


def _valid_response_text(rule_id: str, ref_id: str) -> str:
    return json.dumps(
        {
            "outcome": "DIAGNOSED",
            "hypotheses": [
                {
                    "label": "Unexpected signal dropout",
                    "explanation": "ECHO shows dropouts",
                    "confidence": 0.7,
                    "grounded_in": [ref_id],
                    "supporting_rule_ids": [rule_id],
                }
            ],
            "reasoning_notes": [],
        }
    )


def test_preflight_failure_never_calls_the_client() -> None:
    evidence = load_evidence("insufficient_evidence")
    from app.grounding import ground_evidence

    grounded = ground_evidence(evidence)
    client = _CountingClient(response_text="should never be used")
    provider = GeminiAIProvider(_SETTINGS, client=client)

    result = provider.interpret(evidence, grounded)

    assert result.outcome == "UNKNOWN"
    assert result.unknown_reason == "NO_EVIDENCE"
    assert client.calls == 0


def test_well_formed_response_produces_diagnosed_result() -> None:
    from app.grounding import ground_evidence

    evidence = load_evidence("intermittent_echo")
    grounded = ground_evidence(evidence)
    rule_ref = next(item.ref_id for item in grounded if item.category == "rule_result")

    client = _CountingClient(response_text=_valid_response_text("unexpected-dropout", rule_ref))
    provider = GeminiAIProvider(_SETTINGS, client=client)

    result = provider.interpret(evidence, grounded)

    assert result.outcome == "DIAGNOSED"
    assert client.calls == 1
    assert len(result.hypotheses) == 1
    assert result.hypotheses[0].supporting_rule_ids == ["unexpected-dropout"]


def test_timeout_error_resolves_to_provider_error() -> None:
    from app.grounding import ground_evidence

    evidence = load_evidence("intermittent_echo")
    grounded = ground_evidence(evidence)
    client = _CountingClient(exc=TimeoutError("timed out"))
    provider = GeminiAIProvider(_SETTINGS, client=client)

    result = provider.interpret(evidence, grounded)

    assert result.outcome == "UNKNOWN"
    assert result.unknown_reason == "PROVIDER_ERROR"


def test_generic_exception_resolves_to_provider_error_without_leaking_message() -> None:
    from app.grounding import ground_evidence

    evidence = load_evidence("intermittent_echo")
    grounded = ground_evidence(evidence)
    secret_looking_message = "auth failed for key sk-super-secret-123"
    client = _CountingClient(exc=RuntimeError(secret_looking_message))
    provider = GeminiAIProvider(_SETTINGS, client=client)

    result = provider.interpret(evidence, grounded)

    assert result.outcome == "UNKNOWN"
    assert result.unknown_reason == "PROVIDER_ERROR"
    joined_notes = " ".join(result.reasoning_notes)
    assert secret_looking_message not in joined_notes


def test_non_json_response_resolves_to_provider_error() -> None:
    from app.grounding import ground_evidence

    evidence = load_evidence("intermittent_echo")
    grounded = ground_evidence(evidence)
    client = _CountingClient(response_text="not json at all {{{")
    provider = GeminiAIProvider(_SETTINGS, client=client)

    result = provider.interpret(evidence, grounded)

    assert result.outcome == "UNKNOWN"
    assert result.unknown_reason == "PROVIDER_ERROR"


def test_schema_bound_violation_resolves_to_provider_error() -> None:
    from app.grounding import ground_evidence

    evidence = load_evidence("intermittent_echo")
    grounded = ground_evidence(evidence)
    rule_ref = next(item.ref_id for item in grounded if item.category == "rule_result")
    bad_text = json.dumps(
        {
            "outcome": "DIAGNOSED",
            "hypotheses": [
                {
                    "label": "x",
                    "explanation": "y",
                    "confidence": 2.0,  # out of range
                    "grounded_in": [rule_ref],
                    "supporting_rule_ids": ["unexpected-dropout"],
                }
            ],
            "reasoning_notes": [],
        }
    )
    client = _CountingClient(response_text=bad_text)
    provider = GeminiAIProvider(_SETTINGS, client=client)

    result = provider.interpret(evidence, grounded)

    assert result.outcome == "UNKNOWN"
    assert result.unknown_reason == "PROVIDER_ERROR"


def test_construction_without_api_key_or_client_raises() -> None:
    with pytest.raises(RuntimeError):
        GeminiAIProvider(ProbeSettings(ai_provider="gemini", gemini_api_key=None))


def test_construction_with_injected_client_never_requires_api_key() -> None:
    provider = GeminiAIProvider(
        ProbeSettings(ai_provider="gemini", gemini_api_key=None),
        client=_CountingClient(response_text="{}"),
    )
    assert provider is not None


def test_get_provider_is_cached_under_default_settings(monkeypatch) -> None:
    from app.api import get_provider
    from app.config import get_settings

    get_settings.cache_clear()
    get_provider.cache_clear()
    monkeypatch.delenv("PROBE_AI_PROVIDER", raising=False)

    first = get_provider()
    second = get_provider()
    assert first is second

    get_provider.cache_clear()
    get_settings.cache_clear()
