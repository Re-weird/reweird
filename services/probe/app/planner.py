from app.providers.base import AIProviderResult
from app.schemas import StructuredEvidence


class TestPlanner:
    """Deterministic recommender for the next diagnostic test/action."""

    def recommend(self, evidence: StructuredEvidence, result: AIProviderResult) -> str:
        role, probe = evidence.role, evidence.probe

        if result.outcome == "UNKNOWN":
            if result.unknown_reason == "CONFLICTING_RULES":
                return (
                    f"Re-run measurement capture for {role} on {probe} to resolve "
                    "conflicting rule results before recommending a further test."
                )
            return (
                f"Capture an initial measurement window for {probe} ({role}) to "
                "establish baseline evidence before further testing."
            )

        ids_by_status = {(rule.id, rule.status) for rule in evidence.rule_results}
        has_movement_rule = any(rule_id == "movement-correlation" for rule_id, _ in ids_by_status)

        if ("movement-correlation", "fail") in ids_by_status:
            return (
                f"Reseat or replace the {role} connection on {probe}, then re-run VERIFY "
                "to confirm the dropout rate returns to baseline."
            )
        if any(
            (rule_id, "fail") in ids_by_status
            for rule_id in ("power-rail-instability", "simultaneous-dropout")
        ):
            return (
                "Measure the configured power rail and shared ground reference during "
                "the failure window to rule out a shared electrical cause."
            )
        if ("voltage-outside-specification", "fail") in ids_by_status:
            return (
                f"Verify the measurement divider/scale for {probe} and compare the source "
                "rail under load against the trusted specification."
            )
        if ("pulse-width-outside-specification", "fail") in ids_by_status:
            return (
                f"Re-measure the {role} pulse timing on {probe} and compare it against the "
                "component's trigger/echo specification for signal integrity issues."
            )
        if ("missing-signal", "fail") in ids_by_status:
            return (
                f"Confirm the {probe} probe assignment and compare the physical signal "
                f"with the software-commanded state for {role}."
            )
        if ("unexpected-dropout", "fail") in ids_by_status and not has_movement_rule:
            return (
                f"Gently flex/wiggle the {role} connection on {probe} while monitoring "
                "dropout rate to test for an intermittent physical connection "
                "(movement correlation)."
            )
        if any(
            (rule_id, status) in ids_by_status
            for rule_id in ("baseline-deviation",)
            for status in ("warn", "fail")
        ):
            return (
                f"Re-capture a trusted baseline for {role} and compare deviation after "
                "eliminating other causes."
            )
        return "Continue monitoring during normal operation; no further test indicated."
