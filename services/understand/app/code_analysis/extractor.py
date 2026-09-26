from dataclasses import dataclass, field
from pathlib import Path

from tree_sitter import Language, Node, Parser, Query, QueryCursor

from app.code_analysis.cpp_queries import (
    DECLARED_CONSTANT_QUERY,
    FIELD_CALL_OBJECT_ALLOWLIST,
    FIELD_CALL_QUERY,
    HARDWARE_API_NAMES,
    INCLUDE_QUERY,
    MACRO_CONSTANT_QUERY,
    PIN_MODE_VALUES,
    PLAIN_CALL_QUERY,
)
from app.grounding import make_ref_id
from app.schemas import CodeAnalysisResult, CodeFact

_CPP_EXTENSIONS = {".cpp", ".cc", ".cxx", ".c", ".h", ".hpp", ".ino"}

_MAX_SNIPPET = 240


def detect_language(file_path: str) -> str:
    suffix = Path(file_path).suffix.lower()
    return "cpp" if suffix in _CPP_EXTENSIONS else "unsupported"


@dataclass
class _ParsedFile:
    path: str
    source: bytes
    lines: list[str]
    parse_errors: list[str] = field(default_factory=list)


def _snippet(lines: list[str], line_1indexed: int) -> str:
    if 1 <= line_1indexed <= len(lines):
        return lines[line_1indexed - 1].strip()[:_MAX_SNIPPET]
    return ""


def _collect_parse_errors(node: Node, path: str, out: list[str]) -> None:
    if node.type == "ERROR" or node.is_missing:
        out.append(f"{path}: syntax error near line {node.start_point.row + 1}")
        return  # don't descend further into an already-flagged error subtree
    for child in node.children:
        _collect_parse_errors(child, path, out)


_cpp_language: Language | None = None
_cpp_parser: Parser | None = None


def _get_cpp_parser() -> Parser:
    global _cpp_language, _cpp_parser
    if _cpp_parser is None:
        import tree_sitter_cpp

        _cpp_language = Language(tree_sitter_cpp.language())
        _cpp_parser = Parser(_cpp_language)
    return _cpp_parser


def _run_query(language: Language, root: Node, query_text: str):
    query = Query(language, query_text)
    cursor = QueryCursor(query)
    return cursor.matches(root)


def extract_facts_from_cpp(path: str, source: bytes) -> tuple[list[CodeFact], list[str]]:
    parser = _get_cpp_parser()
    assert _cpp_language is not None
    tree = parser.parse(source)
    lines = source.decode("utf-8", errors="replace").splitlines()

    parse_errors: list[str] = []
    _collect_parse_errors(tree.root_node, path, parse_errors)

    facts: list[CodeFact] = []
    seen: dict[str, int] = {}
    constants: dict[str, str] = {}  # symbol name -> literal pin/value, this file only

    def add(kind: str, line: int, *, symbol=None, pin=None, mode=None, api_call=None) -> None:
        ref_id = make_ref_id(path, line, kind, symbol, seen)
        facts.append(
            CodeFact(
                ref_id=ref_id,
                file=path,
                line=line,
                kind=kind,  # type: ignore[arg-type]
                symbol=symbol,
                pin=pin,
                mode=mode,  # type: ignore[arg-type]
                api_call=api_call,
                raw_snippet=_snippet(lines, line),
            )
        )

    for _, caps in _run_query(_cpp_language, tree.root_node, INCLUDE_QUERY):
        node = caps["path"][0]
        line = node.start_point.row + 1
        add("include_directive", line, symbol=node.text.decode("utf-8", errors="replace"))

    for _, caps in _run_query(_cpp_language, tree.root_node, MACRO_CONSTANT_QUERY):
        name_node = caps["name"][0]
        value_node = caps["value"][0]
        name = name_node.text.decode()
        value = value_node.text.decode().strip()
        if value.lstrip("-").isdigit():
            line = name_node.start_point.row + 1
            constants[name] = value
            add("pin_constant", line, symbol=name, pin=value)

    for _, caps in _run_query(_cpp_language, tree.root_node, DECLARED_CONSTANT_QUERY):
        name_node = caps["name"][0]
        value_node = caps["value"][0]
        name = name_node.text.decode()
        value = value_node.text.decode().strip()
        line = name_node.start_point.row + 1
        constants[name] = value
        add("pin_constant", line, symbol=name, pin=value)

    def handle_call(func_name: str, args_node: Node) -> None:
        line = args_node.start_point.row + 1
        arg_nodes = args_node.named_children
        first_arg_symbol = None
        first_arg_pin = None
        if arg_nodes:
            first = arg_nodes[0]
            if first.type == "identifier":
                first_arg_symbol = first.text.decode()
            elif first.type == "number_literal":
                first_arg_pin = first.text.decode()

        add(
            "hardware_api_call",
            line,
            symbol=first_arg_symbol,
            pin=first_arg_pin,
            api_call=func_name,
        )

        if func_name == "pinMode" and len(arg_nodes) >= 2:
            mode_node = arg_nodes[1]
            mode_text = mode_node.text.decode() if mode_node.type == "identifier" else None
            if mode_text in PIN_MODE_VALUES:
                add(
                    "pin_mode_call",
                    line,
                    symbol=first_arg_symbol,
                    pin=first_arg_pin,
                    mode=mode_text,
                    api_call=func_name,
                )

        if first_arg_symbol and first_arg_symbol in constants:
            add(
                "symbol_reference",
                line,
                symbol=first_arg_symbol,
                pin=constants[first_arg_symbol],
                api_call=func_name,
            )

    for _, caps in _run_query(_cpp_language, tree.root_node, PLAIN_CALL_QUERY):
        func_node = caps["func"][0]
        func_name = func_node.text.decode()
        if func_name in HARDWARE_API_NAMES:
            handle_call(func_name, caps["args"][0])

    for _, caps in _run_query(_cpp_language, tree.root_node, FIELD_CALL_QUERY):
        object_node = caps["object"][0]
        method_node = caps["method"][0]
        object_name = object_node.text.decode()
        if object_name in FIELD_CALL_OBJECT_ALLOWLIST:
            handle_call(f"{object_name}.{method_node.text.decode()}", caps["args"][0])

    return facts, parse_errors


def analyze_files(files: dict[str, bytes]) -> CodeAnalysisResult:
    """files: mapping of relative file path -> raw source bytes."""
    all_facts: list[CodeFact] = []
    all_errors: list[str] = []
    languages_seen: set[str] = set()

    for path, source in files.items():
        language = detect_language(path)
        languages_seen.add(language)
        if language != "cpp":
            continue
        facts, errors = extract_facts_from_cpp(path, source)
        all_facts.extend(facts)
        all_errors.extend(errors)

    overall_language = "cpp" if "cpp" in languages_seen else "unsupported"
    return CodeAnalysisResult(
        files_analyzed=list(files.keys()),
        language=overall_language,  # type: ignore[arg-type]
        facts=all_facts,
        parse_errors=all_errors,
    )
