# Build from the REPOSITORY ROOT (not this directory), because
# app/catalog.py reads packages/component-catalog three levels above
# services/probe/app/ at runtime, exactly as it does locally:
#
#   docker build -f services/probe/Dockerfile -t reweird-probe .
#   docker run --rm -p 8091:8091 reweird-probe
#
# This intentionally does not touch the root docker-compose.yml (owned by
# another subsystem) - it is a standalone, service-local artifact.

FROM python:3.11-slim AS base

RUN pip install --no-cache-dir uv

WORKDIR /app/services/probe
COPY services/probe/pyproject.toml services/probe/uv.lock ./
RUN uv sync --frozen --no-dev --no-install-project

COPY services/probe/app ./app
COPY packages/component-catalog /app/packages/component-catalog
RUN uv sync --frozen --no-dev

RUN adduser --disabled-password --uid 10001 reweird && chown -R reweird /app
USER reweird

# Safe defaults - no secrets baked into the image, no external adapters
# required to boot: the service runs correctly with the in-memory
# repository and FakeAIProvider using nothing but this Dockerfile. Override
# these (and GEMINI_API_KEY/MONGODB_URI/etc.) at `docker run` time, never
# by editing the image.
ENV PROBE_AI_PROVIDER=fake \
    DIAGNOSTIC_REPOSITORY=memory \
    TELEMETRY_SINK=none \
    ANALYTICS_SINK=none

EXPOSE 8091
HEALTHCHECK --interval=30s --timeout=3s CMD python -c "import urllib.request; urllib.request.urlopen('http://localhost:8091/health')" || exit 1
CMD ["uv", "run", "uvicorn", "app.api:app", "--host", "0.0.0.0", "--port", "8091"]
