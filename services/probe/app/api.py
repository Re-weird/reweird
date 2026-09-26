from contextlib import asynccontextmanager
from functools import lru_cache

from fastapi import Depends, FastAPI

from app.config import get_settings
from app.planner import TestPlanner
from app.probe import run_probe
from app.providers.base import AIProvider
from app.providers.fake import FakeAIProvider
from app.providers.gemini import GeminiAIProvider
from app.schemas import ProbeRequest, ProbeResponse


@lru_cache
def get_provider() -> AIProvider:
    settings = get_settings()
    if settings.ai_provider == "gemini":
        return GeminiAIProvider(settings)
    return FakeAIProvider()


@asynccontextmanager
async def lifespan(app: FastAPI):
    get_provider()  # forces construction + config validation at startup, not on first request
    yield


app = FastAPI(title="reweird-probe", version="0.1.0", lifespan=lifespan)


def get_planner() -> TestPlanner:
    return TestPlanner()


@app.post("/probe", response_model=ProbeResponse)
def post_probe(
    request: ProbeRequest,
    provider: AIProvider = Depends(get_provider),
    planner: TestPlanner = Depends(get_planner),
) -> ProbeResponse:
    return run_probe(request.evidence, provider, planner)
