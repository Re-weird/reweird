from app.code_analysis.extractor import analyze_files
from tests.conftest import load_code_fixture


def test_unsupported_language_yields_zero_facts_no_crash() -> None:
    files = {"unsupported_language.py": load_code_fixture("unsupported_language.py")}
    result = analyze_files(files)

    assert result.language == "unsupported"
    assert result.facts == []
    assert result.parse_errors == []


def test_mixed_supported_and_unsupported_files_only_analyzes_supported_ones() -> None:
    files = {
        "healthy_ultrasonic.cpp": load_code_fixture("healthy_ultrasonic.cpp"),
        "unsupported_language.py": load_code_fixture("unsupported_language.py"),
    }
    result = analyze_files(files)

    assert result.language == "cpp"  # at least one supported file present
    assert len(result.files_analyzed) == 2
    assert all(f.file == "healthy_ultrasonic.cpp" for f in result.facts)
