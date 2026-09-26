from app.code_analysis.extractor import analyze_files, detect_language
from tests.conftest import load_code_fixture


def test_detect_language_by_extension() -> None:
    assert detect_language("main.cpp") == "cpp"
    assert detect_language("probe_config.h") == "cpp"
    assert detect_language("sketch.ino") == "cpp"
    assert detect_language("script.py") == "unsupported"
    assert detect_language("no_extension") == "unsupported"


def test_healthy_ultrasonic_extracts_expected_facts() -> None:
    files = {"healthy_ultrasonic.cpp": load_code_fixture("healthy_ultrasonic.cpp")}
    result = analyze_files(files)

    assert result.language == "cpp"
    assert result.parse_errors == []

    kinds = {f.kind for f in result.facts}
    assert kinds == {
        "include_directive",
        "pin_constant",
        "hardware_api_call",
        "pin_mode_call",
        "symbol_reference",
    }

    pin_constants = {f.symbol: f.pin for f in result.facts if f.kind == "pin_constant"}
    assert pin_constants == {"TRIG_PIN": "25", "ECHO_PIN": "26", "POWER_PIN": "34"}

    pin_modes = {(f.symbol, f.mode) for f in result.facts if f.kind == "pin_mode_call"}
    assert pin_modes == {
        ("POWER_PIN", "INPUT"),
        ("TRIG_PIN", "OUTPUT"),
        ("ECHO_PIN", "INPUT"),
    }

    api_calls = {f.api_call for f in result.facts if f.kind == "hardware_api_call"}
    assert "pinMode" in api_calls
    assert "digitalWrite" in api_calls
    assert "pulseIn" in api_calls
    assert "analogReadMilliVolts" in api_calls
    assert "Serial.begin" in api_calls


def test_all_facts_have_stable_deterministic_ref_ids() -> None:
    files = {"healthy_ultrasonic.cpp": load_code_fixture("healthy_ultrasonic.cpp")}
    first = analyze_files(files)
    second = analyze_files(files)
    assert [f.ref_id for f in first.facts] == [f.ref_id for f in second.facts]
    # every ref_id is unique within one analysis
    ref_ids = [f.ref_id for f in first.facts]
    assert len(ref_ids) == len(set(ref_ids))


def test_missing_pin_assignment_still_extracts_bare_gpio_usage() -> None:
    files = {"missing_pin_assignment.cpp": load_code_fixture("missing_pin_assignment.cpp")}
    result = analyze_files(files)

    # GPIO 26 is used directly with no named constant - no symbol_reference
    # or pin_constant fact should claim it has a name.
    bare_pin_calls = [
        f for f in result.facts if f.kind == "hardware_api_call" and f.pin == "26"
    ]
    assert len(bare_pin_calls) >= 1
    assert all(f.symbol is None for f in bare_pin_calls)

    symbol_refs = {f.symbol for f in result.facts if f.kind == "symbol_reference"}
    assert "26" not in symbol_refs  # never fabricated a name for the bare pin
