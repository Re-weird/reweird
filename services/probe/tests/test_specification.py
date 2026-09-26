from app.catalog import CatalogSpecEntry, RangeSpec, SpecificationEntry
from app.schemas import EvidenceFact, StructuredEvidence
from app.specification import evaluate_component_specification


def _entry(**specifications: SpecificationEntry) -> CatalogSpecEntry:
    return CatalogSpecEntry(id="test-widget", name="Test Widget", specifications=specifications)


def _evidence(probe: str, role: str, measurements: list[EvidenceFact]) -> StructuredEvidence:
    return StructuredEvidence(
        probe=probe,
        role=role,
        expected={},
        observed={},
        baseline={"status": "UNKNOWN"},
        measurements=measurements,
        rule_results=[],
    )


VOLTAGE_SPEC = SpecificationEntry(
    pin="VCC",
    signal_type="power_rail",
    is_power_rail=True,
    required=True,
    voltage=RangeSpec(min=4.5, max=5.5, nominal=5.0, unit="V"),
    source="test datasheet",
)

PULSE_SPEC = SpecificationEntry(
    pin="ECHO",
    signal_type="digital_pulse",
    mode="pulse",
    required=True,
    pulse_width=RangeSpec(min=100, max=1000, unit="us"),
    source="test datasheet",
)


def test_voltage_within_range_passes() -> None:
    entry = _entry(VCC=VOLTAGE_SPEC)
    evidence = _evidence(
        "P1", "VCC", [EvidenceFact(probe="P1", name="average_voltage", value=5.0, unit="V", provenance="MEASURED")]
    )
    facts, rules = evaluate_component_specification(entry, "P1", "VCC", evidence)
    assert [r.id for r in rules] == ["voltage-outside-specification"]
    assert rules[0].status == "pass"
    assert rules[0].provenance == "SPECIFICATION"
    assert facts[0].provenance == "SPECIFICATION"


def test_voltage_outside_range_fails() -> None:
    entry = _entry(VCC=VOLTAGE_SPEC)
    evidence = _evidence(
        "P1", "VCC", [EvidenceFact(probe="P1", name="average_voltage", value=4.0, unit="V", provenance="MEASURED")]
    )
    _, rules = evaluate_component_specification(entry, "P1", "VCC", evidence)
    assert len(rules) == 1
    assert rules[0].id == "voltage-outside-specification"
    assert rules[0].status == "fail"


def test_pulse_width_outside_range_fails() -> None:
    entry = _entry(ECHO=PULSE_SPEC)
    evidence = _evidence(
        "P3",
        "ECHO",
        [
            EvidenceFact(probe="P3", name="pulse_width", value=5000, unit="us", provenance="MEASURED"),
            EvidenceFact(probe="P3", name="pulse_count", value=10, provenance="MEASURED"),
        ],
    )
    _, rules = evaluate_component_specification(entry, "P3", "ECHO", evidence)
    ids_statuses = {(r.id, r.status) for r in rules}
    assert ("pulse-width-outside-specification", "fail") in ids_statuses


def test_missing_signal_only_emitted_when_activity_fact_present_and_zero() -> None:
    entry = _entry(ECHO=PULSE_SPEC)
    evidence = _evidence(
        "P3", "ECHO", [EvidenceFact(probe="P3", name="pulse_count", value=0, provenance="MEASURED")]
    )
    _, rules = evaluate_component_specification(entry, "P3", "ECHO", evidence)
    ids_statuses = {(r.id, r.status) for r in rules}
    assert ("missing-signal", "fail") in ids_statuses


def test_missing_observation_is_not_evaluable_never_missing_signal() -> None:
    """Critical distinction: lacking a measurement is NOT the same as having
    deterministically established the signal is absent. Absence of data must
    never be reported as evidence of absence."""
    entry = _entry(ECHO=PULSE_SPEC)
    evidence = _evidence("P3", "ECHO", [])  # no facts at all
    _, rules = evaluate_component_specification(entry, "P3", "ECHO", evidence)
    ids = {r.id for r in rules}
    assert "missing-signal" not in ids
    assert "specification-not-evaluable" in ids
    assert all(r.status == "warn" for r in rules if r.id == "specification-not-evaluable")


def test_unit_mismatch_is_not_evaluable_no_comparison_attempted() -> None:
    entry = _entry(VCC=VOLTAGE_SPEC)
    evidence = _evidence(
        "P1",
        "VCC",
        [EvidenceFact(probe="P1", name="average_voltage", value=5000, unit="mV", provenance="MEASURED")],
    )
    _, rules = evaluate_component_specification(entry, "P1", "VCC", evidence)
    assert len(rules) == 1
    assert rules[0].id == "specification-not-evaluable"
    assert rules[0].status == "warn"
    assert "voltage-outside-specification" not in {r.id for r in rules}


def test_missing_unit_entirely_is_not_evaluable() -> None:
    entry = _entry(VCC=VOLTAGE_SPEC)
    evidence = _evidence(
        "P1", "VCC", [EvidenceFact(probe="P1", name="average_voltage", value=5.0, unit=None, provenance="MEASURED")]
    )
    _, rules = evaluate_component_specification(entry, "P1", "VCC", evidence)
    assert rules[0].id == "specification-not-evaluable"


def test_no_spec_for_role_is_not_evaluable() -> None:
    entry = _entry(VCC=VOLTAGE_SPEC)
    evidence = _evidence("P3", "ECHO", [])
    facts, rules = evaluate_component_specification(entry, "P3", "ECHO", evidence)
    assert facts == []
    assert len(rules) == 1
    assert rules[0].id == "specification-not-evaluable"
    assert rules[0].status == "warn"


def test_power_rail_without_mode_never_emits_required_signal_rule() -> None:
    """A continuous power rail has no 'activity' concept; only its voltage
    range is checked. required=True with mode=None must not spuriously emit
    a 'not evaluable' warning for an activity check that doesn't apply."""
    entry = _entry(VCC=VOLTAGE_SPEC)
    evidence = _evidence(
        "P1", "VCC", [EvidenceFact(probe="P1", name="average_voltage", value=5.0, unit="V", provenance="MEASURED")]
    )
    _, rules = evaluate_component_specification(entry, "P1", "VCC", evidence)
    assert len(rules) == 1
    assert rules[0].id == "voltage-outside-specification"
