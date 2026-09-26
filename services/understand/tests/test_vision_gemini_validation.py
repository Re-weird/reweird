from app.catalog import get_default_catalog
from app.vision.gemini_schema import VisionInterpretationOut
from app.vision.gemini_validation import validate_vision_output


def test_valid_candidate_produces_interpreted_result() -> None:
    catalog = get_default_catalog()
    raw = VisionInterpretationOut(
        outcome="INTERPRETED",
        component_candidates=[
            {"catalog_id": "hc-sr04", "confidence": 0.7, "rationale": "Visible module."}
        ],
    )
    result = validate_vision_output(raw, catalog)
    assert result.outcome == "INTERPRETED"
    assert result.component_candidates[0].catalog_id == "hc-sr04"
    assert result.component_candidates[0].sources == ["vision"]


def test_hallucinated_catalog_id_invalidates_result() -> None:
    catalog = get_default_catalog()
    raw = VisionInterpretationOut(
        outcome="INTERPRETED",
        component_candidates=[
            {"catalog_id": "not-a-real-catalog-id", "confidence": 0.7, "rationale": "x"}
        ],
    )
    result = validate_vision_output(raw, catalog)
    assert result.outcome == "UNKNOWN"
    assert result.unknown_reason == "INVALID_PROVIDER_OUTPUT"


def test_model_declared_unknown_passes_through() -> None:
    catalog = get_default_catalog()
    raw = VisionInterpretationOut(outcome="UNKNOWN", unknown_reason="PROVIDER_UNCERTAIN")
    result = validate_vision_output(raw, catalog)
    assert result.outcome == "UNKNOWN"
    assert result.unknown_reason == "PROVIDER_UNCERTAIN"
    assert result.component_candidates == []
