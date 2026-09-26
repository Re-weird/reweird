import base64
import binascii
from contextlib import asynccontextmanager
from functools import lru_cache
from typing import Literal, Protocol

from fastapi import Depends, FastAPI
from fastapi.responses import JSONResponse
from pydantic import BaseModel, Field, field_validator, model_validator

from app.catalog import CatalogEntry, get_default_catalog
from app.code_analysis.extractor import analyze_files
from app.code_analysis.gemini_validation import CodeInterpretationResult
from app.code_analysis.provider import (
    CodeInterpretationProvider,
    NullCodeInterpretationProvider,
)
from app.config import get_settings
from app.proposal import assemble_proposal
from app.schemas import CodeFact, ProjectProfileProposal, VisionAnalysisResult
from app.vision.provider import NullVisionInterpretationProvider, VisionInterpretationProvider


class CodeProvider(Protocol):
    def interpret(
        self, facts: list[CodeFact], catalog: dict[str, CatalogEntry]
    ) -> CodeInterpretationResult: ...


class VisionProvider(Protocol):
    def interpret(
        self, image_bytes: bytes, mime_type: str, catalog: dict[str, CatalogEntry]
    ) -> VisionAnalysisResult: ...


MAX_REQUEST_BYTES = 8 * 1024 * 1024
MAX_SOURCE_FILES = 8
MAX_SOURCE_FILE_BYTES = 512 * 1024
MAX_SOURCE_TOTAL_BYTES = 1024 * 1024
MAX_IMAGE_BYTES = 5 * 1024 * 1024
MAX_IMAGE_BASE64_CHARS = 4 * ((MAX_IMAGE_BYTES + 2) // 3)


class SourceFile(BaseModel):
    path: str = Field(max_length=240)
    content: str = Field(max_length=MAX_SOURCE_FILE_BYTES)

    @field_validator("content")
    @classmethod
    def bound_encoded_content(cls, value: str) -> str:
        if len(value.encode("utf-8")) > MAX_SOURCE_FILE_BYTES:
            raise ValueError("source file exceeds the 512 KiB limit")
        return value


class ImageInput(BaseModel):
    mime_type: Literal["image/png", "image/jpeg"]
    data_base64: str = Field(max_length=MAX_IMAGE_BASE64_CHARS)

    @field_validator("data_base64")
    @classmethod
    def validate_image_payload(cls, value: str) -> str:
        try:
            decoded = base64.b64decode(value, validate=True)
        except (binascii.Error, ValueError) as error:
            raise ValueError("image must be valid base64") from error
        if len(decoded) > MAX_IMAGE_BYTES:
            raise ValueError("image exceeds the 5 MiB limit")
        return value


class UnderstandRequest(BaseModel):
    files: list[SourceFile] = Field(max_length=MAX_SOURCE_FILES)
    image: ImageInput | None = None

    @model_validator(mode="after")
    def bound_total_code(self) -> "UnderstandRequest":
        if sum(len(source.content.encode("utf-8")) for source in self.files) > MAX_SOURCE_TOTAL_BYTES:
            raise ValueError("source files exceed the 1 MiB aggregate limit")
        return self


@lru_cache
def get_code_provider() -> CodeProvider:
    settings = get_settings()
    if settings.ai_provider == "gemini":
        return CodeInterpretationProvider(settings)
    return NullCodeInterpretationProvider()


@lru_cache
def get_vision_provider() -> VisionProvider:
    settings = get_settings()
    if settings.ai_provider == "gemini":
        return VisionInterpretationProvider(settings)
    return NullVisionInterpretationProvider()


@asynccontextmanager
async def lifespan(app: FastAPI):
    get_code_provider()
    get_vision_provider()
    yield


app = FastAPI(title="reweird-understand", version="0.1.0", lifespan=lifespan)


class RequestBodyLimitMiddleware:
    def __init__(self, downstream):
        self.downstream = downstream

    async def __call__(self, scope, receive, send):
        if scope["type"] != "http" or scope.get("path") != "/understand":
            return await self.downstream(scope, receive, send)
        for name, value in scope.get("headers", []):
            if name.lower() == b"content-length" and value.isdigit() and int(value) > MAX_REQUEST_BYTES:
                return await JSONResponse(status_code=413, content={"detail": "Request exceeds the 8 MiB limit."})(scope, receive, send)
        body = bytearray()
        while True:
            message = await receive()
            if message["type"] == "http.disconnect":
                return
            if message["type"] != "http.request":
                continue
            chunk = message.get("body", b"")
            if len(body) + len(chunk) > MAX_REQUEST_BYTES:
                return await JSONResponse(status_code=413, content={"detail": "Request exceeds the 8 MiB limit."})(scope, receive, send)
            body.extend(chunk)
            if not message.get("more_body", False):
                break

        sent = False

        async def replay():
            nonlocal sent
            if not sent:
                sent = True
                return {"type": "http.request", "body": bytes(body), "more_body": False}
            return {"type": "http.disconnect"}

        return await self.downstream(scope, replay, send)


app.add_middleware(RequestBodyLimitMiddleware)


@app.post("/understand", response_model=ProjectProfileProposal)
def post_understand(
    request: UnderstandRequest,
    code_provider: CodeProvider = Depends(get_code_provider),
    vision_provider: VisionProvider = Depends(get_vision_provider),
) -> ProjectProfileProposal:
    catalog = get_default_catalog()

    files = {f.path: f.content.encode("utf-8") for f in request.files}
    code_analysis = analyze_files(files)
    code_interpretation = code_provider.interpret(code_analysis.facts, catalog)

    vision_analysis = None
    if request.image is not None:
        image_bytes = base64.b64decode(request.image.data_base64)
        vision_analysis = vision_provider.interpret(image_bytes, request.image.mime_type, catalog)

    return assemble_proposal(code_analysis, code_interpretation, vision_analysis, catalog)
