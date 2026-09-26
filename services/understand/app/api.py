import base64
from contextlib import asynccontextmanager
from functools import lru_cache
from typing import Protocol

from fastapi import Depends, FastAPI
from pydantic import BaseModel

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


class SourceFile(BaseModel):
    path: str
    content: str


class ImageInput(BaseModel):
    mime_type: str
    data_base64: str


class UnderstandRequest(BaseModel):
    files: list[SourceFile]
    image: ImageInput | None = None


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
