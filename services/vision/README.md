# Vision service

The implemented Gemini adapter lives behind `internal/vision.Analyzer`. It sends
only a validated, bounded project image and a constrained extraction prompt to
Gemini when `GEMINI_API_KEY` is configured. The model name is configurable with
`GEMINI_MODEL`; the default is the stable multimodal `gemini-3.5-flash-lite`.

The response is parsed as structured JSON, validated, and assigned `VISION_AI`
provenance by server code. Candidate components, readable labels, and possible
relationships are suggestions only. They cannot confirm wiring, override static
code facts, or mark a Project Profile confirmed.

Without a key the adapter returns `VISION_SKIPPED`; code analysis, catalog
enrichment, draft generation, correction, confirmation, and probe planning still
work.
