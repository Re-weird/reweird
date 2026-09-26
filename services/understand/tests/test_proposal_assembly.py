from app.code_analysis.gemini_validation import CodeInterpretationResult
from app.proposal import assemble_proposal
from app.schemas import CodeAnalysisResult, ComponentProposal, VisionAnalysisResult


def _empty_code_analysis() -> CodeAnalysisResult:
    return CodeAnalysisResult(files_analyzed=["main.cpp"], language="cpp", facts=[])


def test_insufficient_information_when_both_modalities_unusable() -> None:
    code_result = CodeInterpretationResult(
        outcome="UNKNOWN",
        components=[],
        roles=[],
        controller=None,
        expected_behavior=None,
        unknown_reason="INSUFFICIENT_INFORMATION",
        reasoning_notes=["No code facts."],
    )
    proposal = assemble_proposal(_empty_code_analysis(), code_result, None, catalog={})
    assert proposal.status == "INSUFFICIENT_INFORMATION"
    assert proposal.project is None


def test_code_only_success_produces_proposed_without_vision() -> None:
    code_result = CodeInterpretationResult(
        outcome="INTERPRETED",
        components=[
            ComponentProposal(
                catalog_id="hc-sr04",
                name="HC-SR04 Ultrasonic Distance Sensor",
                source="component-catalog/hc-sr04",
                confidence=0.8,
                sources=["code_analysis"],
                grounded_in=["fact:main-cpp:1:pin_constant:trig_pin"],
            )
        ],
        roles=[],
        controller="ESP32",
        expected_behavior="Measures distance.",
        unknown_reason=None,
        reasoning_notes=[],
    )
    proposal = assemble_proposal(_empty_code_analysis(), code_result, None, catalog={})
    assert proposal.status == "PROPOSED"
    assert proposal.project is not None
    assert proposal.project.controller == "ESP32"
    assert proposal.project.components[0].catalog_id == "hc-sr04"
    assert proposal.vision_analysis is None


def test_conflicting_code_and_vision_yields_proposed_with_conflicts() -> None:
    code_result = CodeInterpretationResult(
        outcome="INTERPRETED",
        components=[
            ComponentProposal(
                catalog_id="hc-sr04",
                name="HC-SR04",
                source="component-catalog/hc-sr04",
                confidence=0.7,
                sources=["code_analysis"],
                grounded_in=["fact:main-cpp:1:pin_constant:trig_pin"],
            )
        ],
        roles=[],
        controller="ESP32",
        expected_behavior=None,
        unknown_reason=None,
        reasoning_notes=[],
    )
    vision_result = VisionAnalysisResult(
        outcome="INTERPRETED",
        component_candidates=[
            ComponentProposal(
                catalog_id="some-other-sensor",
                name="Some Other Sensor",
                source="component-catalog/some-other-sensor",
                confidence=0.6,
                sources=["vision"],
                grounded_in=[],
            )
        ],
    )
    proposal = assemble_proposal(_empty_code_analysis(), code_result, vision_result, catalog={})
    assert proposal.status == "PROPOSED_WITH_CONFLICTS"
    assert len(proposal.conflicts) == 1
    assert proposal.conflicts[0].code_catalog_id == "hc-sr04"
    assert proposal.conflicts[0].vision_catalog_id == "some-other-sensor"
    assert proposal.unresolved_questions  # a human must be asked, nothing auto-resolved
    # both candidates remain visible - neither is silently dropped
    catalog_ids = {c.catalog_id for c in proposal.project.components}
    assert catalog_ids == {"hc-sr04", "some-other-sensor"}


def test_agreeing_code_and_vision_merge_without_conflict() -> None:
    code_result = CodeInterpretationResult(
        outcome="INTERPRETED",
        components=[
            ComponentProposal(
                catalog_id="hc-sr04",
                name="HC-SR04",
                source="component-catalog/hc-sr04",
                confidence=0.7,
                sources=["code_analysis"],
                grounded_in=["fact:main-cpp:1:pin_constant:trig_pin"],
            )
        ],
        roles=[],
        controller="ESP32",
        expected_behavior=None,
        unknown_reason=None,
        reasoning_notes=[],
    )
    vision_result = VisionAnalysisResult(
        outcome="INTERPRETED",
        component_candidates=[
            ComponentProposal(
                catalog_id="hc-sr04",
                name="HC-SR04",
                source="component-catalog/hc-sr04",
                confidence=0.9,
                sources=["vision"],
                grounded_in=[],
            )
        ],
    )
    proposal = assemble_proposal(_empty_code_analysis(), code_result, vision_result, catalog={})
    assert proposal.status == "PROPOSED"
    assert proposal.conflicts == []
    assert len(proposal.project.components) == 1
    merged = proposal.project.components[0]
    assert set(merged.sources) == {"code_analysis", "vision"}
    assert merged.confidence == 0.9  # max of the two agreeing confidences


def test_vision_failure_does_not_block_code_only_success() -> None:
    code_result = CodeInterpretationResult(
        outcome="INTERPRETED",
        components=[
            ComponentProposal(
                catalog_id="hc-sr04",
                name="HC-SR04",
                source="component-catalog/hc-sr04",
                confidence=0.8,
                sources=["code_analysis"],
                grounded_in=["fact:main-cpp:1:pin_constant:trig_pin"],
            )
        ],
        roles=[],
        controller="ESP32",
        expected_behavior=None,
        unknown_reason=None,
        reasoning_notes=[],
    )
    vision_result = VisionAnalysisResult(
        outcome="UNKNOWN", unknown_reason="PROVIDER_ERROR", reasoning_notes=["vision timed out"]
    )
    proposal = assemble_proposal(_empty_code_analysis(), code_result, vision_result, catalog={})
    assert proposal.status == "PROPOSED"
    assert proposal.project is not None
    assert proposal.project.components[0].catalog_id == "hc-sr04"


def test_proposal_never_contains_a_p1_p6_probe_field() -> None:
    """Structural guarantee: nothing in the proposed roles can claim a
    ReWeird harness channel assignment (P1-P6) - that's a later human step."""
    from app.schemas import ProposedRole

    assert "probe" not in ProposedRole.model_fields
