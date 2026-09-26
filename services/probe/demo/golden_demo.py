#!/usr/bin/env python3
"""ReWire Intelligence Layer - golden, offline, end-to-end demo.

Run from services/probe:

    uv run python demo/golden_demo.py

Requires NO API keys, NO cloud credentials, NO hardware, NO network, and NO
frontend. Every provider is the deterministic/fake default
(PROBE_AI_PROVIDER=fake, DIAGNOSTIC_REPOSITORY=memory, TELEMETRY_SINK=none,
ANALYTICS_SINK=none) - the exact same safe defaults a fresh `git clone` +
`uv sync` already has.

CORE RULE, demonstrated at every step below:
    THE LLM REASONS ABOUT EVIDENCE. IT DOES NOT CREATE EVIDENCE.

Flow:
    source code (+ optional image, omitted here)
        -> PROJECT UNDERSTANDING (services/understand, M3 - via its own
           environment/public boundary, invoked as a subprocess so this
           script never needs services/understand's dependencies installed
           in services/probe's own venv)
        -> ProjectProfileProposal (PROPOSED, unconfirmed)
        -> COMPONENT INTELLIGENCE (M4): catalog -> deterministic
           specification context
        -> StructuredEvidence -> PROBE (M1/M2) -> grounded hypotheses
        -> CLOSED-LOOP SESSION (M5): a second, REAL evidence submission
           narrows the diagnosis
        -> PATCH PROPOSAL (M6): grounded, proposal-only, no execution
        -> SIMULATED external execution record (never real hardware)
        -> new REAL after-evidence -> VERIFY (M6): deterministic
           before/after comparison
        -> persisted + reloaded through the M7 repository abstraction
        -> safe JSON export (M7)
"""

import json
import subprocess
import sys
from pathlib import Path

PROBE_DIR = Path(__file__).resolve().parents[1]
REPO_ROOT = PROBE_DIR.parents[1]
UNDERSTAND_DIR = REPO_ROOT / "services" / "understand"

sys.path.insert(0, str(PROBE_DIR))

from app.analytics_sink import InMemoryAnalyticsSink, build_session_summary  # noqa: E402
from app.catalog import get_default_catalog  # noqa: E402
from app.export import build_diagnostic_export  # noqa: E402
from app.patch_provider import FakePatchProposalProvider  # noqa: E402
from app.patch_proposals import create_patch_proposal, record_external_result, verify_proposal  # noqa: E402
from app.planner import TestPlanner  # noqa: E402
from app.providers.fake import FakeAIProvider  # noqa: E402
from app.repository import InMemoryDiagnosticRepository  # noqa: E402
from app.schemas import PatchProposalRequest, RecordExternalResultRequest, StructuredEvidence  # noqa: E402
from app.sessions import submit_evidence  # noqa: E402
from app.sessions import create_session as create_diagnostic_session  # noqa: E402
from app.telemetry_sink import InMemoryTelemetrySink, extract_telemetry_records  # noqa: E402

FIXTURES_DIR = PROBE_DIR / "fixtures"


def _section(title: str) -> None:
    print("\n" + "=" * 78)
    print(title)
    print("=" * 78)


def _load_evidence(name: str) -> StructuredEvidence:
    return StructuredEvidence.model_validate_json((FIXTURES_DIR / f"{name}.json").read_text())


# ---------------------------------------------------------------------------
# 1. PROJECT UNDERSTANDING (services/understand, through its own environment)
# ---------------------------------------------------------------------------

_UNDERSTAND_SNIPPET = r"""
import json
from app.catalog import get_default_catalog
from app.code_analysis.extractor import analyze_files
from app.code_analysis.provider import CodeInterpretationProvider
from app.config import UnderstandSettings
from app.proposal import assemble_proposal

source = open("fixtures/code/healthy_ultrasonic.cpp", "rb").read()
code_analysis = analyze_files({"main.cpp": source})
catalog = get_default_catalog()

trig_ref = next((f.ref_id for f in code_analysis.facts if f.kind == "pin_constant" and f.symbol == "TRIG_PIN"), None)
echo_ref = next((f.ref_id for f in code_analysis.facts if f.kind == "pin_constant" and f.symbol == "ECHO_PIN"), None)


class FakeClient:
    def __init__(self, text):
        self.text = text

    def generate(self, system_instruction, user_content):
        return self.text


gemini_text = json.dumps({
    "outcome": "INTERPRETED",
    "controller": "ESP32",
    "expected_behavior_summary": "Measures distance via an ultrasonic trigger/echo pulse pair.",
    "component_candidates": [{
        "catalog_id": "hc-sr04",
        "confidence": 0.86,
        "grounded_in": [r for r in (trig_ref, echo_ref) if r],
        "rationale": "TRIG output + ECHO input pulse pattern matches an HC-SR04.",
    }],
    "role_candidates": [
        {"target_pin": "25", "likely_role": "TRIG", "mode": "pulse", "is_power_rail": False, "confidence": 0.85, "grounded_in": [trig_ref] if trig_ref else []},
        {"target_pin": "26", "likely_role": "ECHO", "mode": "pulse", "is_power_rail": False, "confidence": 0.85, "grounded_in": [echo_ref] if echo_ref else []},
    ],
    "reasoning_notes": [],
})

settings = UnderstandSettings(ai_provider="gemini", gemini_api_key=None)
provider = CodeInterpretationProvider(settings, client=FakeClient(gemini_text))
interpretation = provider.interpret(code_analysis.facts, catalog)
proposal = assemble_proposal(code_analysis, interpretation, None, catalog)

print(json.dumps({
    "code_facts": [f.model_dump(mode="json") for f in code_analysis.facts],
    "proposal": proposal.model_dump(mode="json"),
}))
"""


def run_project_understanding() -> dict:
    _section("PROJECT UNDERSTANDING (services/understand, offline/fake)")
    result = subprocess.run(
        ["uv", "run", "python3", "-c", _UNDERSTAND_SNIPPET],
        cwd=UNDERSTAND_DIR,
        capture_output=True,
        text=True,
    )
    if result.returncode != 0:
        print(result.stderr, file=sys.stderr)
        raise RuntimeError("services/understand demo step failed - see stderr above")
    data = json.loads(result.stdout)

    print(f"CodeFacts extracted (deterministic, SOFTWARE provenance): {len(data['code_facts'])}")
    for fact in data["code_facts"]:
        if fact["kind"] == "pin_constant":
            print(f"  - {fact['symbol']} = GPIO{fact['pin']} (user's own board pin, file={fact['file']}:{fact['line']})")

    proposal = data["proposal"]
    print(f"\nProjectProfileProposal.status = {proposal['status']}  (unconfirmed - a human must review this)")
    if proposal["project"]:
        print(f"Component candidates: {[c['catalog_id'] for c in proposal['project']['components']]}")
        for role in proposal["project"]["roles"]:
            print(f"  role: target_pin={role['target_pin']!r} (user GPIO) role={role['role']!r} - NOTE: no 'probe' field exists here at all")
    print(
        "\nIMPORTANT: target_pin values above are the USER's OWN GPIO numbers "
        "(e.g. GPIO25). They are NOT ReWeird's P1-P6 physical probe channels - "
        "that mapping is a separate, later, human, physical step this proposal "
        "never performs."
    )
    return proposal


# ---------------------------------------------------------------------------
# 2. COMPONENT INTELLIGENCE (M4) - trusted, deterministic catalog knowledge
# ---------------------------------------------------------------------------


def run_component_intelligence():
    _section("COMPONENT INTELLIGENCE (M4 - trusted SPECIFICATION knowledge)")
    catalog = get_default_catalog()
    entry = catalog["hc-sr04"]
    print(f"Catalog entry: {entry.id} ({entry.name})")
    for role, spec in entry.specifications.items():
        print(f"  {role}: signal_type={spec.signal_type!r} required={spec.required}")
        if spec.voltage:
            print(f"    voltage: {spec.voltage.min}-{spec.voltage.max}{spec.voltage.unit} (source: {spec.source})")
        if spec.pulse_width:
            print(f"    pulse_width: {spec.pulse_width.min}-{spec.pulse_width.max}{spec.pulse_width.unit} (source: {spec.source})")
    print("\nNo Gemini call was made to produce any of the numbers above.")
    return catalog


# ---------------------------------------------------------------------------
# main
# ---------------------------------------------------------------------------


def main() -> None:
    project_understanding_proposal = run_project_understanding()
    catalog = run_component_intelligence()

    provider = FakeAIProvider()
    planner = TestPlanner()
    patch_provider = FakePatchProposalProvider()

    _section("DIAGNOSTIC STEP 1 (StructuredEvidence -> PROBE)")
    session = create_diagnostic_session(
        _load_evidence("hc_sr04_trig_and_echo_missing"), "hc-sr04", catalog, provider, planner
    )
    step1 = session.steps[0]
    print(f"session_id: {session.session_id}   status: {session.status}")
    print("Deterministic rules (grounded, before any hypothesis):")
    for rule in step1.evaluated_evidence.rule_results:
        print(f"  - {rule.id} [{rule.status}] probe={rule.probe} ({rule.provenance}): {rule.message}")
    print("Hypotheses:")
    for h in step1.result.hypotheses:
        assert h.grounded_in, "every hypothesis must cite grounded evidence"
        print(f"  - {h.label} (confidence={h.confidence}) grounded_in={h.grounded_in}")
    print(f"Recommended next test: {step1.result.recommended_test}")

    _section("DIAGNOSTIC STEP 2 (closed loop, M5 - REAL new evidence)")
    session, duplicate = submit_evidence(
        session, _load_evidence("hc_sr04_missing_echo_activity"), catalog, provider, planner
    )
    assert duplicate is None
    step2 = session.steps[1]
    print("New evidence submitted: pulse_count now measured at 0 (a real capture, not an assertion)")
    print("Updated rules:")
    for rule in step2.evaluated_evidence.rule_results:
        print(f"  - {rule.id} [{rule.status}] probe={rule.probe}: {rule.message}")
    print("Updated (narrowed) hypotheses:")
    for h in step2.result.hypotheses:
        print(f"  - {h.label} (confidence={h.confidence})")
    print(f"Next test: {step2.result.recommended_test}")

    _section("PATCH PROPOSAL (M6 - proposal only, no execution)")
    proposal = create_patch_proposal(
        session,
        PatchProposalRequest(
            source_step_number=1, target_probe="P2", target_role="TRIG",
            patch_type="TEMPORARY_SIGNAL_EMULATION",
        ),
        patch_provider,
    )
    print(f"proposal_id: {proposal.proposal_id}")
    print(f"patch_type: {proposal.patch_type}")
    print(f"target: {proposal.target_role} ({proposal.target_probe})")
    print(f"purpose: {proposal.purpose}")
    print(f"expected_effect: {proposal.expected_effect}")
    print(f"evidence_refs: {proposal.evidence_refs}")
    print(f"safety_requirements: {proposal.safety_requirements}")
    print(f"status: {proposal.status}")
    print("Execution: NOT PERFORMED BY PYTHON - services/probe contains no GPIO/serial/hardware code.")

    _section("EXTERNAL RESULT -- SIMULATED EXTERNAL EXECUTION FOR OFFLINE DEMO")
    approved = record_external_result(proposal, RecordExternalResultRequest(external_status="APPROVED_EXTERNALLY", reason="safety-checked (simulated)"))
    executed = record_external_result(approved, RecordExternalResultRequest(external_status="EXECUTED_EXTERNALLY", reason="SIMULATED EXTERNAL EXECUTION FOR OFFLINE DEMO"))
    print(f"proposal status: {executed.status}  (recorded metadata only - not electrical evidence)")

    _section("VERIFY (M6 - deterministic before/after comparison)")
    session, verified_proposal, verification, duplicate = verify_proposal(
        session, executed, _load_evidence("hc_sr04_healthy"), catalog, provider, planner
    )
    assert duplicate is None
    print("BEFORE (step 1) rule for the session's focus probe:")
    for r in step1.evaluated_evidence.rule_results:
        if r.probe == session.probe and r.id == "missing-signal":
            print(f"  missing-signal: {r.status}")
    print("AFTER (new step) rule for the same probe:")
    for r in session.steps[-1].evaluated_evidence.rule_results:
        if r.probe == session.probe and r.id == "missing-signal":
            print(f"  missing-signal: {r.status}")
    print("Deterministic changes:")
    for change in verification.changes:
        print(f"  - {change.kind} {change.name} (probe={change.probe}): {change.before} -> {change.after}  [{change.change}]")
    print(f"VERIFY outcome: {verification.outcome}")
    print(f"proposal status after verify: {verified_proposal.status}")
    assert verification.outcome == "SUPPORTED", "golden demo fixtures are curated to genuinely support this outcome"

    _section("Also demonstrating NOT_SUPPORTED and INCONCLUSIVE (separate sessions, not faked)")
    for label, after_fixture, expected in (
        ("NOT_SUPPORTED", "hc_sr04_missing_echo_activity", "NOT_SUPPORTED"),
        ("INCONCLUSIVE", "hc_sr04_not_evaluable", "INCONCLUSIVE"),
    ):
        alt_session = create_diagnostic_session(
            _load_evidence("hc_sr04_trig_and_echo_missing"), "hc-sr04", catalog, provider, planner
        )
        alt_proposal = create_patch_proposal(
            alt_session,
            PatchProposalRequest(source_step_number=1, target_probe="P2", target_role="TRIG", patch_type="TEMPORARY_SIGNAL_EMULATION"),
            patch_provider,
        )
        alt_approved = record_external_result(alt_proposal, RecordExternalResultRequest(external_status="APPROVED_EXTERNALLY"))
        alt_executed = record_external_result(alt_approved, RecordExternalResultRequest(external_status="EXECUTED_EXTERNALLY"))
        _, _, alt_verification, _ = verify_proposal(alt_session, alt_executed, _load_evidence(after_fixture), catalog, provider, planner)
        print(f"  {label}: outcome = {alt_verification.outcome}  (expected {expected})")
        assert alt_verification.outcome == expected

    _section("HISTORY (M7 - InMemoryDiagnosticRepository, save -> reload -> compare)")
    repo = InMemoryDiagnosticRepository()
    repo.create_session(session)
    repo.save_patch_proposal(verified_proposal)
    reloaded_session = repo.get_session(session.session_id)
    reloaded_proposal = repo.get_patch_proposal(verified_proposal.proposal_id)
    print(f"Round-trip equal (session): {reloaded_session == session}")
    print(f"Round-trip equal (proposal): {reloaded_proposal == verified_proposal}")
    assert reloaded_session == session
    assert reloaded_proposal == verified_proposal
    print(f"Steps: {[s.step_number for s in reloaded_session.steps]}")
    print(f"Patch proposals: [{reloaded_proposal.proposal_id}]  status={reloaded_proposal.status}")
    print(f"Verification result: {reloaded_proposal.verification.outcome}")
    print(f"Final status: {reloaded_session.status}")

    _section("EXPORT (M7 - safe JSON export)")
    export = build_diagnostic_export(reloaded_session, [reloaded_proposal])
    export_json = json.dumps(export, indent=2)
    secret_markers = ("GEMINI_API_KEY", "MONGODB_URI", "TIGER_DATABASE_URL", "SNOWFLAKE_PASSWORD", "Authorization: Bearer")
    found = [m for m in secret_markers if m in export_json]
    print(f"Export size: {len(export_json)} bytes; sections: {list(export.keys())}")
    print(f"Secrets present: {'YES - ' + str(found) if found else 'NO'}")
    assert not found

    _section("TELEMETRY + ANALYTICS (M7 - fake sinks, best-effort, never gate the diagnosis)")
    telemetry = InMemoryTelemetrySink()
    for step in reloaded_session.steps:
        for record in extract_telemetry_records(reloaded_session.session_id, step.evaluated_evidence, step.created_at_ms):
            telemetry.record_measurement(record)
    analytics = InMemoryAnalyticsSink()
    analytics.record_session_summary(build_session_summary(reloaded_session, [reloaded_proposal]))
    print(f"Telemetry records captured: {len(telemetry.records)}")
    print(f"Analytics summaries captured: {len(analytics.summaries)}")

    print("\n" + "=" * 78)
    print("GOLDEN DEMO COMPLETE - all assertions passed.")
    print("=" * 78)


if __name__ == "__main__":
    main()
