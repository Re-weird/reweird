from app.code_analysis.extractor import analyze_files
from tests.conftest import load_code_fixture


def test_malformed_syntax_records_parse_errors_but_still_extracts_partial_facts() -> None:
    files = {"malformed_syntax.cpp": load_code_fixture("malformed_syntax.cpp")}
    result = analyze_files(files)

    assert result.parse_errors != []
    assert any("malformed_syntax.cpp" in err for err in result.parse_errors)

    # The #define and the later, syntactically-valid digitalWrite call
    # should still be extracted from the parseable regions.
    kinds = {f.kind for f in result.facts}
    assert "pin_constant" in kinds
    assert "hardware_api_call" in kinds


def test_malformed_syntax_does_not_raise() -> None:
    files = {"malformed_syntax.cpp": load_code_fixture("malformed_syntax.cpp")}
    # Should complete without any exception - a parse failure is data, not a crash.
    result = analyze_files(files)
    assert result.language == "cpp"
