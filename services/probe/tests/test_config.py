from app.config import get_settings


def test_defaults_with_no_env_set(monkeypatch) -> None:
    get_settings.cache_clear()
    monkeypatch.delenv("PROBE_AI_PROVIDER", raising=False)
    monkeypatch.delenv("GEMINI_API_KEY", raising=False)
    monkeypatch.delenv("GEMINI_MODEL", raising=False)
    monkeypatch.delenv("GEMINI_TIMEOUT_SECONDS", raising=False)

    settings = get_settings()
    assert settings.ai_provider == "fake"
    assert settings.gemini_api_key is None
    assert settings.gemini_model == "gemini-flash-latest"
    assert settings.gemini_timeout_seconds == 10.0
    get_settings.cache_clear()


def test_env_overrides_all_four_vars(monkeypatch) -> None:
    get_settings.cache_clear()
    monkeypatch.setenv("PROBE_AI_PROVIDER", "gemini")
    monkeypatch.setenv("GEMINI_API_KEY", "dummy-key")
    monkeypatch.setenv("GEMINI_MODEL", "gemini-custom")
    monkeypatch.setenv("GEMINI_TIMEOUT_SECONDS", "5.5")

    settings = get_settings()
    assert settings.ai_provider == "gemini"
    assert settings.gemini_api_key == "dummy-key"
    assert settings.gemini_model == "gemini-custom"
    assert settings.gemini_timeout_seconds == 5.5
    get_settings.cache_clear()


def test_invalid_ai_provider_falls_back_to_fake(monkeypatch) -> None:
    get_settings.cache_clear()
    monkeypatch.setenv("PROBE_AI_PROVIDER", "not-a-real-provider")
    settings = get_settings()
    assert settings.ai_provider == "fake"
    get_settings.cache_clear()


def test_blank_api_key_is_treated_as_none(monkeypatch) -> None:
    get_settings.cache_clear()
    monkeypatch.setenv("GEMINI_API_KEY", "")
    settings = get_settings()
    assert settings.gemini_api_key is None
    get_settings.cache_clear()
