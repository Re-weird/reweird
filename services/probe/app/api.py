from fastapi import Depends, FastAPI

from app.planner import TestPlanner
from app.probe import run_probe
from app.providers.base import AIProvider
from app.providers.fake import FakeAIProvider
from app.schemas import ProbeRequest, ProbeResponse

app = FastAPI(title="reweird-probe", version="0.1.0")


def get_provider() -> AIProvider:
    return FakeAIProvider()


def get_planner() -> TestPlanner:
    return TestPlanner()


@app.post("/probe", response_model=ProbeResponse)
def post_probe(
    request: ProbeRequest,
    provider: AIProvider = Depends(get_provider),
    planner: TestPlanner = Depends(get_planner),
) -> ProbeResponse:
    return run_probe(request.evidence, provider, planner)
