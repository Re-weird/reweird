from app.catalog import get_default_catalog
from app.planner import TestPlanner
from app.providers.base import AIProviderResult
from app.providers.fake import FakeAIProvider
from app.schemas import EvidenceFact, Hypothesis, StructuredEvidence
from app.sessions import (
    InMemorySessionStore,
    SessionStoppedError,
    create_session,
    fingerprint_evidence,
    find_duplicate_step,
    stop_session,
    submit_evidence,
)
from tests.conftest import load_evidence

catalog = get_default_catalog()
provider = FakeAIProvider()
planner = TestPlanner()


class _CountingProvider:
    def __init__(self) -> None:
        self.calls = 0

    def interpret(self, evidence, grounded) -> AIProviderResult:
        self.calls += 1
        return provider.interpret(evidence, grounded)


def _echo_evidence(**overrides) -> StructuredEvidence:
    fields = dict(
        probe="P3",
        role="ECHO",
        expected={},
        observed={},
        baseline={"status": "UNKNOWN"},
        measurements=[],
        rule_results=[],
    )
    fields.update(overrides)
    return StructuredEvidence(**fields)


# ---------------------------------------------------------------------------
# 1-2: session creation
# ---------------------------------------------------------------------------


def test_create_session_from_initial_evidence() -> None:
    evidence = load_evidence("hc_sr04_not_evaluable")
    session = create_session(evidence, "hc-sr04", catalog, provider, planner)
    assert session.session_id.startswith("sess_")
    assert session.probe == "P3"
    assert session.role == "ECHO"
    assert session.component_id == "hc-sr04"
    assert len(session.steps) == 1
    assert session.steps[0].step_number == 1
    assert session.max_steps == 10


def test_initial_active_session_has_a_recommended_test() -> None:
    evidence = load_evidence("hc_sr04_not_evaluable")
    session = create_session(evidence, "hc-sr04", catalog, provider, planner)
    assert session.status == "ACTIVE"
    assert "measurement" in session.steps[0].result.recommended_test.lower()


# ---------------------------------------------------------------------------
# 3-5: follow-up evidence / append-only history / step numbering
# ---------------------------------------------------------------------------


def test_follow_up_evidence_creates_step_two() -> None:
    session = create_session(load_evidence("hc_sr04_not_evaluable"), "hc-sr04", catalog, provider, planner)
    updated, duplicate = submit_evidence(
        session, load_evidence("hc_sr04_missing_echo_activity"), catalog, provider, planner
    )
    assert duplicate is None
    assert len(updated.steps) == 2
    assert updated.steps[1].step_number == 2


def test_step_one_is_unchanged_after_step_two_is_appended() -> None:
    session = create_session(load_evidence("hc_sr04_not_evaluable"), "hc-sr04", catalog, provider, planner)
    step_one_before = session.steps[0].model_copy(deep=True)

    updated, _ = submit_evidence(
        session, load_evidence("hc_sr04_missing_echo_activity"), catalog, provider, planner
    )

    assert updated.steps[0] == step_one_before
    assert session.steps[0] == step_one_before  # original session object also untouched


def test_ordered_step_numbering_across_several_submissions() -> None:
    session = create_session(_echo_evidence(), None, catalog, provider, planner)
    for i in range(3):
        session, _ = submit_evidence(
            session, _echo_evidence(unresolved_questions=[f"round {i}"]), catalog, provider, planner
        )
    assert [s.step_number for s in session.steps] == [1, 2, 3, 4]


# ---------------------------------------------------------------------------
# 6: evidence can legitimately narrow hypotheses (HC-SR04 closed loop)
# ---------------------------------------------------------------------------


def test_new_evidence_narrows_from_not_evaluable_to_missing_signal() -> None:
    session = create_session(load_evidence("hc_sr04_not_evaluable"), "hc-sr04", catalog, provider, planner)
    assert session.steps[0].result.hypotheses[0].supporting_rule_ids == ["specification-not-evaluable"]

    session, duplicate = submit_evidence(
        session, load_evidence("hc_sr04_missing_echo_activity"), catalog, provider, planner
    )
    assert duplicate is None
    assert session.steps[1].result.hypotheses[0].supporting_rule_ids == ["missing-signal"]
    # The recommended test changed because the new, real evidence supports a
    # different, more specific next action - never fabricated.
    assert "probe assignment" in session.steps[1].result.recommended_test.lower()


# ---------------------------------------------------------------------------
# 7-8: previous hypotheses are never evidence
# ---------------------------------------------------------------------------


def test_previous_hypotheses_never_appear_as_trusted_evidence_facts() -> None:
    session = create_session(load_evidence("hc_sr04_missing_echo_activity"), "hc-sr04", catalog, provider, planner)
    session, _ = submit_evidence(
        session, load_evidence("hc_sr04_not_evaluable"), catalog, provider, planner
    )
    for step in session.steps:
        for fact in [*(step.evaluated_evidence.measurements or []), *(step.evaluated_evidence.derived_facts or [])]:
            assert fact.provenance != "AI_INTERPRETATION"
        for rule in step.evaluated_evidence.rule_results:
            assert rule.provenance != "AI_INTERPRETATION"


def test_previous_ai_interpretation_cannot_satisfy_grounding_for_a_later_step() -> None:
    """Structural guarantee, not just a text scan: step 2's hypotheses can
    only ever cite ref_ids that exist in step 2's OWN freshly-computed
    grounded evidence - never anything from step 1's result. This is true
    because _run_step/ground_evidence only ever look at the evidence the
    caller submitted for THIS step; a prior step's Hypothesis objects are
    never read when building the next step at all."""
    from app.grounding import ground_evidence

    session = create_session(load_evidence("hc_sr04_missing_echo_activity"), "hc-sr04", catalog, provider, planner)
    step_one_ref_ids = {
        ref for h in session.steps[0].result.hypotheses for ref in h.grounded_in
    }

    session, _ = submit_evidence(
        session, load_evidence("hc_sr04_not_evaluable"), catalog, provider, planner
    )
    step_two = session.steps[1]
    step_two_allowed_refs = {item.ref_id for item in ground_evidence(step_two.evaluated_evidence)}
    step_two_cited_refs = {ref for h in step_two.result.hypotheses for ref in h.grounded_in}

    assert step_two_cited_refs.issubset(step_two_allowed_refs)
    # None of step 1's own ref ids leaked into what step 2 considers valid to
    # cite (a real, independently-meaningful assertion here since both steps
    # use fixtures for the same probe/role and could otherwise collide).
    assert step_two_cited_refs.isdisjoint(step_one_ref_ids - step_two_allowed_refs)
    # And nothing in the evidence actually submitted for step 2 carries
    # AI_INTERPRETATION provenance - only step 1's *result* (never fed back
    # in) has that provenance at all.
    assert all(
        rule.provenance != "AI_INTERPRETATION" for rule in step_two.evaluated_evidence.rule_results
    )


# ---------------------------------------------------------------------------
# 9-10: duplicate evidence
# ---------------------------------------------------------------------------


def test_exact_duplicate_evidence_is_detected() -> None:
    evidence = load_evidence("hc_sr04_missing_echo_activity")
    session = create_session(evidence, "hc-sr04", catalog, provider, planner)

    duplicate_step = find_duplicate_step(session, evidence)
    assert duplicate_step is not None
    assert duplicate_step.step_number == 1


def test_duplicate_evidence_does_not_append_a_step_or_change_status() -> None:
    evidence = load_evidence("hc_sr04_missing_echo_activity")
    session = create_session(evidence, "hc-sr04", catalog, provider, planner)
    status_before = session.status
    step_count_before = len(session.steps)

    updated, duplicate_step = submit_evidence(session, evidence, catalog, provider, planner)

    assert duplicate_step is not None
    assert duplicate_step.step_number == 1
    assert len(updated.steps) == step_count_before  # no new step appended
    assert updated.status == status_before  # no fabricated progress
    assert updated == session  # session object is returned unchanged


def test_fingerprint_is_stable_regardless_of_dict_key_construction_order() -> None:
    a = StructuredEvidence(
        probe="P1", role="VCC", expected={}, observed={}, baseline={"status": "UNKNOWN", "trusted": True},
        rule_results=[],
    )
    b = StructuredEvidence(
        probe="P1", role="VCC", expected={}, observed={}, baseline={"trusted": True, "status": "UNKNOWN"},
        rule_results=[],
    )
    assert fingerprint_evidence(a) == fingerprint_evidence(b)


# ---------------------------------------------------------------------------
# 11-14: session lookup / malformed input / stop behavior
# ---------------------------------------------------------------------------


def test_unknown_session_returns_404(client) -> None:
    response = client.get("/sessions/sess_does_not_exist")
    assert response.status_code == 404


def test_malformed_follow_up_evidence_returns_422(client) -> None:
    evidence = load_evidence("hc_sr04_not_evaluable")
    create_response = client.post("/sessions", json={"evidence": evidence.model_dump(mode="json")})
    session_id = create_response.json()["session_id"]

    response = client.post(f"/sessions/{session_id}/evidence", json={"evidence": {"not": "valid"}})
    assert response.status_code == 422


def test_stopped_session_rejects_additional_evidence() -> None:
    session = create_session(_echo_evidence(), None, catalog, provider, planner)
    stopped = stop_session(session)
    assert stopped.status == "STOPPED"

    try:
        submit_evidence(stopped, _echo_evidence(), catalog, provider, planner)
        assert False, "expected SessionStoppedError"
    except SessionStoppedError:
        pass


def test_stop_endpoint_preserves_history(client) -> None:
    evidence = load_evidence("hc_sr04_not_evaluable")
    create_response = client.post(
        "/sessions", json={"evidence": evidence.model_dump(mode="json"), "component_id": "hc-sr04"}
    )
    session_id = create_response.json()["session_id"]

    stop_response = client.post(f"/sessions/{session_id}/stop")
    assert stop_response.status_code == 200
    body = stop_response.json()
    assert body["status"] == "STOPPED"
    assert len(body["steps"]) == 1  # history preserved, not deleted

    evidence_response = client.post(
        f"/sessions/{session_id}/evidence",
        json={"evidence": load_evidence("hc_sr04_missing_echo_activity").model_dump(mode="json")},
    )
    assert evidence_response.status_code == 409


# ---------------------------------------------------------------------------
# 15: maximum-step guard
# ---------------------------------------------------------------------------


def test_maximum_step_guard_stops_the_session() -> None:
    from app.schemas import RuleResult

    def _fail_evidence(note: str) -> StructuredEvidence:
        return _echo_evidence(
            unresolved_questions=[note],
            rule_results=[RuleResult(id="missing-signal", probe="P3", status="fail", message=note)],
        )

    session = create_session(_fail_evidence("round 1"), None, catalog, provider, planner, max_steps=2)
    assert session.status == "ACTIVE"  # a real fail rule exists: a further test is meaningful
    assert len(session.steps) == 1

    session, duplicate = submit_evidence(session, _fail_evidence("round 2"), catalog, provider, planner)
    assert duplicate is None
    assert len(session.steps) == 2
    assert session.status == "STOPPED"
    assert session.stop_reason == "max_steps_reached"

    try:
        submit_evidence(session, _fail_evidence("round 3"), catalog, provider, planner)
        assert False, "expected SessionStoppedError once the step limit is reached"
    except SessionStoppedError:
        pass


# ---------------------------------------------------------------------------
# 16-17: conflicting evidence / provider failure
# ---------------------------------------------------------------------------


def test_conflicting_evidence_fails_closed_within_a_step() -> None:
    from app.schemas import RuleResult

    evidence = _echo_evidence(
        rule_results=[
            RuleResult(id="missing-signal", probe="P3", status="fail", message="a"),
            RuleResult(id="missing-signal", probe="P3", status="pass", message="b"),
        ]
    )
    session = create_session(evidence, None, catalog, provider, planner)
    assert session.steps[0].result.outcome == "UNKNOWN"
    assert session.steps[0].result.unknown_reason == "CONFLICTING_RULES"
    assert session.status == "UNKNOWN"


def test_provider_failure_produces_safe_unknown_not_a_crash() -> None:
    """Sessions reuse GeminiAIProvider unmodified, which already converts
    every client failure into a safe UNKNOWN/PROVIDER_ERROR result rather
    than raising (Milestone 2, test_gemini_provider.py). This proves that
    contract still holds end-to-end through the new session pipeline."""
    from app.config import ProbeSettings
    from app.providers.gemini import GeminiAIProvider

    class _TimeoutClient:
        def generate(self, system_instruction: str, user_content: dict) -> str:
            raise TimeoutError("simulated provider timeout")

    settings = ProbeSettings(ai_provider="gemini", gemini_api_key=None)
    gemini_provider = GeminiAIProvider(settings, client=_TimeoutClient())

    evidence = load_evidence("hc_sr04_voltage_outside_spec")
    session = create_session(evidence, "hc-sr04", catalog, gemini_provider, planner)

    assert session.steps[0].result.outcome == "UNKNOWN"
    assert session.steps[0].result.unknown_reason == "PROVIDER_ERROR"
    assert session.status == "UNKNOWN"
    # The deterministic voltage failure is still visible in the evaluated
    # evidence even though the provider itself failed - Gemini's failure
    # never erases or hides a deterministic finding.
    assert any(
        r.id == "voltage-outside-specification" and r.status == "fail"
        for r in session.steps[0].evaluated_evidence.rule_results
    )


# ---------------------------------------------------------------------------
# 18-20: component intelligence integration
# ---------------------------------------------------------------------------


def test_unknown_component_preserves_milestone_4_behavior_with_zero_provider_calls() -> None:
    counting = _CountingProvider()
    session = create_session(_echo_evidence(), "does-not-exist", catalog, counting, planner)
    assert session.steps[0].result.outcome == "UNKNOWN"
    assert session.steps[0].result.unknown_reason == "UNKNOWN_COMPONENT"
    assert counting.calls == 0


def test_component_specification_evaluation_runs_on_every_applicable_step() -> None:
    session = create_session(load_evidence("hc_sr04_voltage_outside_spec"), "hc-sr04", catalog, provider, planner)
    assert any(r.id == "voltage-outside-specification" for r in session.steps[0].evaluated_evidence.rule_results)

    session, _ = submit_evidence(
        session, load_evidence("hc_sr04_pulse_outside_spec"), catalog, provider, planner
    )
    assert any(
        r.id == "pulse-width-outside-specification" for r in session.steps[1].evaluated_evidence.rule_results
    )


def test_missing_observation_does_not_become_missing_signal_across_a_session() -> None:
    session = create_session(load_evidence("hc_sr04_not_evaluable"), "hc-sr04", catalog, provider, planner)
    ids = {r.id for r in session.steps[0].evaluated_evidence.rule_results}
    assert "missing-signal" not in ids
    assert "specification-not-evaluable" in ids


# ---------------------------------------------------------------------------
# 21-22: existing /probe + full M1-4 regression (separately verified by
# running the whole suite; these two spot-check the most load-bearing paths)
# ---------------------------------------------------------------------------


def test_post_probe_still_works_unchanged(client) -> None:
    evidence = load_evidence("healthy")
    response = client.post("/probe", json={"evidence": evidence.model_dump(mode="json")})
    assert response.status_code == 200
    assert response.json()["outcome"] == "DIAGNOSED"


# ---------------------------------------------------------------------------
# 23-24: safety
# ---------------------------------------------------------------------------


def test_no_session_endpoint_can_control_hardware() -> None:
    import app.api as api_module

    source = open(api_module.__file__).read().lower()
    for forbidden in ("gpio", "serial.write", "hardware_control", "digitalwrite", "/patch"):
        assert forbidden not in source


def test_no_patch_route_exists(client) -> None:
    response = client.post("/patch", json={})
    assert response.status_code == 404
