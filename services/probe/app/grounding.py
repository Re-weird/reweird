import re

from app.schemas import EvidenceFact, GroundedItem, RuleResult, StructuredEvidence

_CATEGORY_PREFIX: dict[str, str] = {
    "measurement": "measurement",
    "derived_fact": "derived",
    "specification_result": "spec",
    "baseline_comparison": "baseline",
    "rule_result": "rule",
}

_SLUG_RE = re.compile(r"[^a-z0-9_]+")


def _slug(text: str) -> str:
    return _SLUG_RE.sub("-", text.lower()).strip("-") or "na"


def _fact_summary(entry: EvidenceFact) -> str:
    probe = entry.probe or "unassigned probe"
    unit = f" {entry.unit}" if entry.unit else ""
    return f"{probe}: {entry.name} = {entry.value}{unit} ({entry.provenance})"


def _rule_summary(entry: RuleResult) -> str:
    probe = entry.probe or "unassigned probe"
    return f"{probe}: rule '{entry.id}' {entry.status} - {entry.message}"


def ground_evidence(evidence: StructuredEvidence) -> list[GroundedItem]:
    items: list[GroundedItem] = []
    seen: dict[str, int] = {}

    def add_facts(category: str, entries: list[EvidenceFact] | None) -> None:
        for entry in entries or []:
            base = f"{_CATEGORY_PREFIX[category]}:{_slug(entry.probe or 'na')}:{_slug(entry.name)}"
            seen[base] = seen.get(base, 0) + 1
            ref_id = base if seen[base] == 1 else f"{base}:{seen[base]}"
            items.append(
                GroundedItem(
                    ref_id=ref_id,
                    category=category,  # type: ignore[arg-type]
                    probe=entry.probe,
                    name=entry.name,
                    status=None,
                    provenance=entry.provenance,
                    summary=_fact_summary(entry),
                )
            )

    def add_rules(entries: list[RuleResult]) -> None:
        for entry in entries:
            base = f"{_CATEGORY_PREFIX['rule_result']}:{_slug(entry.probe or 'na')}:{_slug(entry.id)}"
            seen[base] = seen.get(base, 0) + 1
            ref_id = base if seen[base] == 1 else f"{base}:{seen[base]}"
            items.append(
                GroundedItem(
                    ref_id=ref_id,
                    category="rule_result",
                    probe=entry.probe,
                    name=entry.id,
                    status=entry.status,
                    provenance=entry.provenance,
                    summary=_rule_summary(entry),
                )
            )

    add_facts("measurement", evidence.measurements)
    add_facts("derived_fact", evidence.derived_facts)
    add_facts("specification_result", evidence.specification_results)
    add_facts("baseline_comparison", evidence.baseline_comparison)
    add_rules(evidence.rule_results)
    return items


def index_by_ref_id(items: list[GroundedItem]) -> dict[str, GroundedItem]:
    return {item.ref_id: item for item in items}


def refs_for(
    items: list[GroundedItem], *, probe: str | None = None, rule_id: str | None = None
) -> list[str]:
    result: list[str] = []
    for item in items:
        if probe is not None and item.probe != probe:
            continue
        if rule_id is not None and not (item.category == "rule_result" and item.name == rule_id):
            continue
        result.append(item.ref_id)
    return result
