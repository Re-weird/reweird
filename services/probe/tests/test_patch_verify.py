from app.catalog import get_default_catalog
from app.patch_provider import FakePatchProposalProvider, PatchProposalDraft, PatchProposalProviderResult
from app.patch_proposals import (
    InvalidProposalTransitionError,
    PatchProposalRejected,
    ProposalAlreadyVerifiedError,
    ProposalNotExecutedError,
    create_patch_proposal,
    record_external_result,
    verify_proposal,
)
from app.planner import TestPlanner
from app.providers.base import AIProviderResult
from app.providers.fake import FakeAIProvider
from app.schemas import (
    Hypothesis,
    PatchProposalRequest,
    RecordExternalResultRequest,
)
from app.sessions import SessionStoppedError, create_session, stop_session
from app.verification import compare_rule_status, determine_outcome
from tests.conftest import load_evidence

catalog = get_default_catalog()
provider = FakeAIProvider()
planner = TestPlanner()
patch_provider = FakePatchProposalProvider()


def _fresh_session(component_id: str | None = "hc-sr04"):
    return create_session(
        load_evidence("hc_sr04_trig_and_echo_missing"), component_id, catalog, provider, planner
    )


def _trig_request(source_step_number: int | None = None) -> PatchProposalRequest:
    return PatchProposalRequest(
        source_step_number=source_step_number,
        target_probe="P2",
        target_role="TRIG",
        patch_type="TEMPORARY_SIGNAL_EMULATION",
    )


# ---------------------------------------------------------------------------
# 1-3: proposal generation + grounding
# ---------------------------------------------------------------------------


def test_grounded_diagnosis_can_create_valid_patch_proposal() -> None:
    session = _fresh_session()
    proposal = create_patch_proposal(session, _trig_request(), patch_provider)
    assert proposal.status == "PROPOSED"
    assert proposal.target_probe == "P2"
    assert proposal.patch_type == "TEMPORARY_SIGNAL_EMULATION"
    assert proposal.evidence_refs  # non-empty


def test_proposal_references_real_evidence() -> None:
    from app.grounding import ground_evidence, index_by_ref_id

    session = _fresh_session()
    proposal = create_patch_proposal(session, _trig_request(), patch_provider)
    source_step = session.steps[proposal.source_step_number - 1]
    grounded_index = index_by_ref_id(ground_evidence(source_step.evaluated_evidence))
    assert all(ref in grounded_index for ref in proposal.evidence_refs)
    assert any(grounded_index[ref].probe == "P2" for ref in proposal.evidence_refs)


def test_hallucinated_evidence_ref_is_rejected() -> None:
    class _HallucinatingProvider:
        def propose(self, source_step, target_probe, target_role, patch_type):
            return PatchProposalProviderResult(
                outcome="PROPOSED",
                draft=PatchProposalDraft(
                    purpose="Test a hallucinated ref.",
                    expected_effect="Should never be accepted.",
                    evidence_refs=["rule:p2:this-ref-id-does-not-exist"],
                ),
            )

    session = _fresh_session()
    try:
        create_patch_proposal(session, _trig_request(), _HallucinatingProvider())
        assert False, "expected PatchProposalRejected"
    except PatchProposalRejected as exc:
        assert exc.unknown_reason == "INVALID_PROVIDER_OUTPUT"
        assert "hallucinated" in exc.reasoning_notes[0].lower()


# ---------------------------------------------------------------------------
# 4-6: validation / safety
# ---------------------------------------------------------------------------


def test_unknown_source_step_is_rejected() -> None:
    session = _fresh_session()
    try:
        create_patch_proposal(session, _trig_request(source_step_number=999), patch_provider)
        assert False, "expected PatchProposalRejected"
    except PatchProposalRejected as exc:
        assert exc.unknown_reason == "NO_EVIDENCE"


def test_closed_patch_type_enum_enforced(client) -> None:
    session = _fresh_session()
    response = client.post(
        f"/sessions/{session.session_id}/patch-proposals",
        json={"target_probe": "P2", "target_role": "TRIG", "patch_type": "ARBITRARY_WAVEFORM"},
    )
    assert response.status_code == 422  # Pydantic Literal rejects it outright


def test_proposal_cannot_contain_executable_hardware_action() -> None:
    from app.grounding import ground_evidence

    session = _fresh_session()
    p2_refs = [
        item.ref_id
        for item in ground_evidence(session.steps[0].evaluated_evidence)
        if item.probe == "P2"
    ]

    class _HardwareInstructionProvider:
        def propose(self, source_step, target_probe, target_role, patch_type):
            return PatchProposalProviderResult(
                outcome="PROPOSED",
                draft=PatchProposalDraft(
                    purpose="digitalWrite(25, HIGH) to force the pin",
                    expected_effect="Pin goes high.",
                    evidence_refs=p2_refs,
                ),
            )

    try:
        create_patch_proposal(session, _trig_request(), _HardwareInstructionProvider())
        assert False, "expected PatchProposalRejected"
    except PatchProposalRejected as exc:
        assert exc.unknown_reason == "INVALID_PROVIDER_OUTPUT"
        assert "hardware instruction" in exc.reasoning_notes[0].lower()


def test_proposal_cannot_claim_execution_already_happened() -> None:
    class _ExecutionClaimingProvider:
        def propose(self, source_step, target_probe, target_role, patch_type):
            from app.grounding import ground_evidence

            refs = [item.ref_id for item in ground_evidence(source_step.evaluated_evidence) if item.probe == "P2"]
            return PatchProposalProviderResult(
                outcome="PROPOSED",
                draft=PatchProposalDraft(
                    purpose="The patch has already been executed successfully.",
                    expected_effect="Confirmed executed.",
                    evidence_refs=refs,
                ),
            )

    session = _fresh_session()
    try:
        create_patch_proposal(session, _trig_request(), _ExecutionClaimingProvider())
        assert False, "expected PatchProposalRejected"
    except PatchProposalRejected as exc:
        assert exc.unknown_reason == "INVALID_PROVIDER_OUTPUT"


# ---------------------------------------------------------------------------
# 4 (unknown session), 7: does not modify evidence history
# ---------------------------------------------------------------------------


def test_unknown_session_rejected_at_api_layer(client) -> None:
    response = client.post(
        "/sessions/sess_does_not_exist/patch-proposals",
        json={"target_probe": "P2", "target_role": "TRIG", "patch_type": "TEMPORARY_SIGNAL_EMULATION"},
    )
    assert response.status_code == 404


def test_creating_proposal_does_not_modify_evidence_history() -> None:
    session = _fresh_session()
    step_before = session.steps[0].model_copy(deep=True)
    create_patch_proposal(session, _trig_request(), patch_provider)
    assert session.steps[0] == step_before
    assert len(session.steps) == 1


# ---------------------------------------------------------------------------
# 8-9: external result recording
# ---------------------------------------------------------------------------


def test_external_approval_and_execution_metadata_can_be_recorded() -> None:
    session = _fresh_session()
    proposal = create_patch_proposal(session, _trig_request(), patch_provider)

    approved = record_external_result(
        proposal, RecordExternalResultRequest(external_status="APPROVED_EXTERNALLY", reason="safety-checked")
    )
    assert approved.status == "APPROVED_EXTERNALLY"
    assert approved.external_reason == "safety-checked"

    executed = record_external_result(
        approved, RecordExternalResultRequest(external_status="EXECUTED_EXTERNALLY")
    )
    assert executed.status == "EXECUTED_EXTERNALLY"


def test_invalid_transition_is_rejected() -> None:
    session = _fresh_session()
    proposal = create_patch_proposal(session, _trig_request(), patch_provider)
    try:
        record_external_result(proposal, RecordExternalResultRequest(external_status="EXECUTED_EXTERNALLY"))
        assert False, "expected InvalidProposalTransitionError"
    except InvalidProposalTransitionError:
        pass


def test_external_execution_metadata_is_not_treated_as_measurement_evidence() -> None:
    session = _fresh_session()
    proposal = create_patch_proposal(session, _trig_request(), patch_provider)
    approved = record_external_result(proposal, RecordExternalResultRequest(external_status="APPROVED_EXTERNALLY"))
    executed = record_external_result(approved, RecordExternalResultRequest(external_status="EXECUTED_EXTERNALLY"))

    # "executed" is a proposal-status transition only - it never appears
    # anywhere inside any DiagnosticStep's evidence.
    serialized_steps = session.model_dump_json()
    assert "EXECUTED_EXTERNALLY" not in serialized_steps
    for step in session.steps:
        for fact in [*(step.evaluated_evidence.measurements or []), *(step.evaluated_evidence.derived_facts or [])]:
            assert fact.provenance in ("MEASURED", "DERIVED", "SPECIFICATION", "BASELINE", "SOFTWARE")
    assert executed.status == "EXECUTED_EXTERNALLY"


# ---------------------------------------------------------------------------
# 10-11: verify preconditions
# ---------------------------------------------------------------------------


def test_verify_cannot_happen_before_external_execution() -> None:
    session = _fresh_session()
    proposal = create_patch_proposal(session, _trig_request(), patch_provider)
    try:
        verify_proposal(session, proposal, load_evidence("hc_sr04_healthy"), catalog, provider, planner)
        assert False, "expected ProposalNotExecutedError"
    except ProposalNotExecutedError:
        pass

    approved = record_external_result(proposal, RecordExternalResultRequest(external_status="APPROVED_EXTERNALLY"))
    try:
        verify_proposal(session, approved, load_evidence("hc_sr04_healthy"), catalog, provider, planner)
        assert False, "expected ProposalNotExecutedError"
    except ProposalNotExecutedError:
        pass


def test_verify_requires_structured_evidence(client) -> None:
    session = _fresh_session()
    proposal = create_patch_proposal(session, _trig_request(), patch_provider)
    from app.patch_proposals import get_default_patch_proposal_store
    from app.sessions import get_default_session_store

    get_default_session_store().save(session)
    get_default_patch_proposal_store().save(proposal)

    response = client.post(
        f"/sessions/{session.session_id}/patch-proposals/{proposal.proposal_id}/verify",
        json={"evidence": {"patch_worked": True}},
    )
    assert response.status_code == 422


# ---------------------------------------------------------------------------
# 12-15: deterministic before/after comparison
# ---------------------------------------------------------------------------


def _executed_proposal(session):
    proposal = create_patch_proposal(session, _trig_request(), patch_provider)
    approved = record_external_result(proposal, RecordExternalResultRequest(external_status="APPROVED_EXTERNALLY"))
    return record_external_result(approved, RecordExternalResultRequest(external_status="EXECUTED_EXTERNALLY"))


def test_before_after_deterministic_comparison_works() -> None:
    session = _fresh_session()
    proposal = _executed_proposal(session)
    updated_session, updated_proposal, verification, duplicate = verify_proposal(
        session, proposal, load_evidence("hc_sr04_healthy"), catalog, provider, planner
    )
    assert duplicate is None
    assert verification is not None
    assert any(c.name == "missing-signal" and c.change == "signal_restored" for c in verification.changes)


def test_compatible_voltage_timing_comparisons_work() -> None:
    before = create_session(load_evidence("hc_sr04_voltage_outside_spec"), "hc-sr04", catalog, provider, planner)

    # Build a synthetic after step with a passing voltage rule on the same probe.
    from app.schemas import RuleResult

    synthetic_after = before.steps[0].model_copy(
        update={
            "step_number": 2,
            "evaluated_evidence": before.steps[0].evaluated_evidence.model_copy(
                update={
                    "rule_results": [
                        RuleResult(id="voltage-outside-specification", probe="P1", status="pass", message="ok", provenance="SPECIFICATION")
                    ]
                }
            ),
        }
    )
    changes = compare_rule_status(before.steps[0], synthetic_after, "P1")
    restored = [c for c in changes if c.name == "voltage-outside-specification"]
    assert restored[0].change == "voltage_restored"


def test_incompatible_units_become_inconclusive() -> None:
    from app.schemas import EvidenceFact
    from app.verification import compare_fact_values

    session = _fresh_session()
    before_step = session.steps[0]
    after_step = before_step.model_copy(
        update={
            "step_number": 2,
            "evaluated_evidence": before_step.evaluated_evidence.model_copy(
                update={
                    "measurements": [
                        EvidenceFact(probe="P3", name="pulse_count", value=5000, unit="mV", provenance="MEASURED")
                    ]
                }
            ),
        }
    )
    changes = compare_fact_values(before_step, after_step, "P3")
    pulse_change = [c for c in changes if c.name == "pulse_count"][0]
    assert pulse_change.change == "incompatible_units"


def test_missing_after_observation_becomes_inconclusive() -> None:
    session = _fresh_session()
    proposal = _executed_proposal(session)
    _, _, verification, duplicate = verify_proposal(
        session, proposal, load_evidence("hc_sr04_not_evaluable"), catalog, provider, planner
    )
    assert duplicate is None
    assert verification.outcome == "INCONCLUSIVE"


# ---------------------------------------------------------------------------
# 16-18: verification semantics
# ---------------------------------------------------------------------------


def test_predicted_effect_observed_is_supported() -> None:
    session = _fresh_session()
    proposal = _executed_proposal(session)
    _, updated_proposal, verification, _ = verify_proposal(
        session, proposal, load_evidence("hc_sr04_healthy"), catalog, provider, planner
    )
    assert verification.outcome == "SUPPORTED"
    assert updated_proposal.status == "VERIFIED"


def test_predicted_effect_absent_is_not_supported() -> None:
    session = _fresh_session()
    proposal = _executed_proposal(session)
    _, updated_proposal, verification, _ = verify_proposal(
        session, proposal, load_evidence("hc_sr04_missing_echo_activity"), catalog, provider, planner
    )
    assert verification.outcome == "NOT_SUPPORTED"
    assert updated_proposal.status == "FAILED_VERIFICATION"


def test_insufficient_evidence_is_inconclusive_not_supported() -> None:
    session = _fresh_session()
    proposal = _executed_proposal(session)
    _, updated_proposal, verification, _ = verify_proposal(
        session, proposal, load_evidence("hc_sr04_not_evaluable"), catalog, provider, planner
    )
    assert verification.outcome == "INCONCLUSIVE"
    # Not terminal - proposal remains retryable with better evidence.
    assert updated_proposal.status == "EXECUTED_EXTERNALLY"


def test_determine_outcome_never_claims_supported_on_empty_predicted_ids() -> None:
    outcome, _, _ = determine_outcome([], set())
    assert outcome == "INCONCLUSIVE"


# ---------------------------------------------------------------------------
# 19-21: closed-loop / grounding integration
# ---------------------------------------------------------------------------


def test_verification_appends_and_preserves_session_history() -> None:
    session = _fresh_session()
    proposal = _executed_proposal(session)
    step_one_before = session.steps[0].model_copy(deep=True)

    updated_session, _, _, _ = verify_proposal(
        session, proposal, load_evidence("hc_sr04_healthy"), catalog, provider, planner
    )
    assert len(updated_session.steps) == 2
    assert updated_session.steps[0] == step_one_before  # untouched
    assert session.steps[0] == step_one_before  # original session object also untouched


def test_previous_ai_hypotheses_still_never_become_trusted_evidence() -> None:
    session = _fresh_session()
    proposal = _executed_proposal(session)
    updated_session, _, _, _ = verify_proposal(
        session, proposal, load_evidence("hc_sr04_healthy"), catalog, provider, planner
    )
    for step in updated_session.steps:
        for rule in step.evaluated_evidence.rule_results:
            assert rule.provenance != "AI_INTERPRETATION"


def test_gemini_cannot_override_deterministic_verification_result() -> None:
    """Even a provider confidently claiming the opposite cannot change the
    deterministic before/after comparison - Hypothesis has no field able to
    mutate a RuleResult, and VerificationResult is built entirely from
    app.verification's pure functions, never from provider output."""

    class _ContradictingProvider:
        def interpret(self, evidence, grounded) -> AIProviderResult:
            return AIProviderResult(
                outcome="DIAGNOSED",
                hypotheses=[
                    Hypothesis(
                        rank=1,
                        label="Everything is fine (contradicting claim)",
                        explanation="Ignore the deterministic result.",
                        confidence=0.99,
                        grounded_in=[],
                        supporting_rule_ids=[],
                    )
                ],
                reasoning_notes=[],
            )

    session = create_session(
        load_evidence("hc_sr04_trig_and_echo_missing"), "hc-sr04", catalog, _ContradictingProvider(), planner
    )
    proposal = create_patch_proposal(session, _trig_request(), patch_provider)
    approved = record_external_result(proposal, RecordExternalResultRequest(external_status="APPROVED_EXTERNALLY"))
    executed = record_external_result(approved, RecordExternalResultRequest(external_status="EXECUTED_EXTERNALLY"))

    _, _, verification, _ = verify_proposal(
        session, executed, load_evidence("hc_sr04_missing_echo_activity"), catalog, _ContradictingProvider(), planner
    )
    # The provider says everything is fine, but the deterministic comparison
    # still correctly reports NOT_SUPPORTED - untouched by the provider.
    assert verification.outcome == "NOT_SUPPORTED"


# ---------------------------------------------------------------------------
# 22-23: stopped sessions / duplicate verification
# ---------------------------------------------------------------------------


def test_stopped_session_behaves_safely_during_verify() -> None:
    session = _fresh_session()
    proposal = _executed_proposal(session)
    stopped = stop_session(session)
    try:
        verify_proposal(stopped, proposal, load_evidence("hc_sr04_healthy"), catalog, provider, planner)
        assert False, "expected SessionStoppedError"
    except SessionStoppedError:
        pass


def test_duplicate_verification_evidence_handled_safely() -> None:
    session = _fresh_session()
    proposal = _executed_proposal(session)

    updated_session, updated_proposal, verification, duplicate = verify_proposal(
        session, proposal, load_evidence("hc_sr04_healthy"), catalog, provider, planner
    )
    assert duplicate is None
    assert updated_proposal.status == "VERIFIED"

    # Resubmitting the exact same after-evidence for verify a second time is
    # a duplicate at the session level - no new step, no new verification,
    # and (since the proposal is already terminal/VERIFIED) also rejected as
    # already-verified.
    try:
        verify_proposal(
            updated_session, updated_proposal, load_evidence("hc_sr04_healthy"), catalog, provider, planner
        )
        assert False, "expected ProposalAlreadyVerifiedError"
    except ProposalAlreadyVerifiedError:
        pass


def test_duplicate_evidence_during_an_inconclusive_retry_does_not_fabricate_progress() -> None:
    session = _fresh_session()
    proposal = _executed_proposal(session)

    updated_session, updated_proposal, verification, duplicate = verify_proposal(
        session, proposal, load_evidence("hc_sr04_not_evaluable"), catalog, provider, planner
    )
    assert verification.outcome == "INCONCLUSIVE"
    assert updated_proposal.status == "EXECUTED_EXTERNALLY"  # retryable, not terminal

    # A genuine retry is allowed (not yet terminal) but resubmitting the SAME
    # evidence again must be detected as a duplicate, not a fresh attempt.
    _, _, verification_2, duplicate_2 = verify_proposal(
        updated_session, updated_proposal, load_evidence("hc_sr04_not_evaluable"), catalog, provider, planner
    )
    assert duplicate_2 is not None
    assert verification_2 is None


# ---------------------------------------------------------------------------
# 24: existing POST /probe unaffected
# ---------------------------------------------------------------------------


def test_post_probe_still_works_after_m6(client) -> None:
    evidence = load_evidence("healthy")
    response = client.post("/probe", json={"evidence": evidence.model_dump(mode="json")})
    assert response.status_code == 200
    assert response.json()["outcome"] == "DIAGNOSED"


# ---------------------------------------------------------------------------
# 26: no hardware-control code anywhere in the new modules
# ---------------------------------------------------------------------------


def test_no_hardware_control_imports_or_code_in_patch_modules() -> None:
    import app.patch_provider as patch_provider_module
    import app.patch_proposals as patch_proposals_module
    import app.verification as verification_module

    # app.patch_safety is deliberately excluded: it is the module that
    # DEFINES these forbidden strings as a blocklist to scan OTHER text
    # against - it legitimately contains them as string literals and never
    # imports or executes any of them. That module's own safety is instead
    # verified by test_patch_safety_module_never_imports_hardware_libraries
    # below, which checks its actual `import` statements.
    forbidden = ("import serial", "gpio", "digitalwrite(", "subprocess.", "os.system(")
    for module in (patch_provider_module, patch_proposals_module, verification_module):
        source = open(module.__file__).read().lower()
        for token in forbidden:
            assert token not in source, f"{module.__name__} unexpectedly contains {token!r}"


def test_patch_safety_module_never_imports_hardware_libraries() -> None:
    import ast

    import app.patch_safety as patch_safety_module

    tree = ast.parse(open(patch_safety_module.__file__).read())
    imported_names = {
        alias.name
        for node in ast.walk(tree)
        if isinstance(node, (ast.Import, ast.ImportFrom))
        for alias in node.names
    }
    assert imported_names.isdisjoint({"serial", "RPi", "gpiozero", "subprocess", "os"})


def test_no_execution_endpoint_exists(client) -> None:
    session = _fresh_session()
    proposal = create_patch_proposal(session, _trig_request(), patch_provider)
    response = client.post(
        f"/sessions/{session.session_id}/patch-proposals/{proposal.proposal_id}/execute", json={}
    )
    assert response.status_code == 404
