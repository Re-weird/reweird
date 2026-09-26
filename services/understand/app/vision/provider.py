import logging
from typing import Protocol

from pydantic import ValidationError

from app.catalog import CatalogEntry
from app.config import UnderstandSettings
from app.schemas import VisionAnalysisResult
from app.vision.gemini_schema import VisionInterpretationOut
from app.vision.gemini_validation import validate_vision_output

logger = logging.getLogger("app.vision.provider")

_SYSTEM_INSTRUCTION = """\
You are the vision-interpretation step of ReWeird's Project Understanding \
pipeline. You are given a photo of an electronics project and a closed \
list of known hardware component catalog entries.

Identify which catalog components are plausibly visible. You must NEVER \
claim an exact electrical measurement (voltage, frequency, resistance, or \
any other numeric reading) from the image - visual identification only. \
You may only cite a "catalog_id" that appears in the attached catalog \
list. If you are not confident, respond with outcome "UNKNOWN" and \
unknown_reason "PROVIDER_UNCERTAIN" instead of guessing. Respond with \
exactly one JSON object matching the required schema.
"""


class VisionClient(Protocol):
    def generate(self, system_instruction: str, image_bytes: bytes, mime_type: str, catalog: dict) -> str: ...


def _provider_error(message: str) -> VisionAnalysisResult:
    return VisionAnalysisResult(
        outcome="UNKNOWN", unknown_reason="PROVIDER_ERROR", reasoning_notes=[message]
    )


def _log_failure(event: str, detail: str | None) -> None:
    if detail:
        logger.error("%s: %s", event, detail)
    else:
        logger.error(event)


_MAX_IMAGE_BYTES = 8 * 1024 * 1024
_ALLOWED_MIME_TYPES = {"image/png", "image/jpeg", "image/webp"}


class _RealGeminiVisionClient:
    def __init__(self, settings: UnderstandSettings):
        from google import genai

        self._client = genai.Client(api_key=settings.gemini_api_key)
        self._model = settings.gemini_vision_model
        self._timeout_ms = int(settings.gemini_timeout_seconds * 1000)

    def generate(
        self, system_instruction: str, image_bytes: bytes, mime_type: str, catalog: dict
    ) -> str:
        import json

        from google.genai import types

        parts = [
            types.Part.from_text(text=json.dumps({"catalog": catalog})),
            types.Part.from_bytes(data=image_bytes, mime_type=mime_type),
        ]
        response = self._client.models.generate_content(
            model=self._model,
            contents=parts,
            config=types.GenerateContentConfig(
                system_instruction=system_instruction,
                response_mime_type="application/json",
                response_schema=VisionInterpretationOut,
                http_options=types.HttpOptions(timeout=self._timeout_ms),
            ),
        )
        return response.text or ""


class NullVisionInterpretationProvider:
    """No-op interpreter used when no AI provider is configured (default).

    Never calls any network client; always resolves to UNKNOWN/PROVIDER_UNCERTAIN.
    """

    def interpret(
        self, image_bytes: bytes, mime_type: str, catalog: dict[str, CatalogEntry]
    ) -> VisionAnalysisResult:
        return VisionAnalysisResult(
            outcome="UNKNOWN",
            unknown_reason="PROVIDER_UNCERTAIN",
            reasoning_notes=["No AI interpretation provider is configured (ai_provider=fake)."],
        )


class VisionInterpretationProvider:
    def __init__(self, settings: UnderstandSettings, client: VisionClient | None = None):
        if client is not None:
            self._client: VisionClient = client
        elif settings.ai_provider == "gemini":
            if not settings.gemini_api_key:
                raise RuntimeError(
                    "GEMINI_API_KEY is required when UNDERSTAND_AI_PROVIDER=gemini"
                )
            self._client = _RealGeminiVisionClient(settings)
        else:
            raise RuntimeError(
                "VisionInterpretationProvider constructed without ai_provider=gemini or "
                "an injected client"
            )

    def interpret(
        self, image_bytes: bytes, mime_type: str, catalog: dict[str, CatalogEntry]
    ) -> VisionAnalysisResult:
        if mime_type not in _ALLOWED_MIME_TYPES:
            return VisionAnalysisResult(
                outcome="UNKNOWN",
                unknown_reason="INVALID_PROVIDER_OUTPUT",
                reasoning_notes=[f"Unsupported image mime type '{mime_type}'."],
            )
        if len(image_bytes) > _MAX_IMAGE_BYTES:
            return VisionAnalysisResult(
                outcome="UNKNOWN",
                unknown_reason="INVALID_PROVIDER_OUTPUT",
                reasoning_notes=["Image exceeds the maximum allowed size."],
            )

        catalog_index = {
            "entries": [
                {"id": entry.id, "name": entry.name, "interfaces": entry.interfaces}
                for entry in catalog.values()
            ]
        }

        try:
            raw_text = self._client.generate(
                _SYSTEM_INSTRUCTION, image_bytes, mime_type, catalog_index
            )
        except TimeoutError:
            _log_failure("understand_vision_timeout", None)
            return _provider_error("Gemini vision request timed out.")
        except Exception as exc:
            _log_failure("understand_vision_request_failed", type(exc).__name__)
            return _provider_error("Gemini vision request failed.")

        try:
            raw = VisionInterpretationOut.model_validate_json(raw_text)
        except ValidationError as exc:
            _log_failure(
                "understand_vision_schema_invalid", f"{len(exc.errors())} field error(s)"
            )
            return _provider_error("Gemini vision response did not match the expected schema.")

        return validate_vision_output(raw, catalog)
