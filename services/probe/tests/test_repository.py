from app.catalog import get_default_catalog
from app.events import new_event
from app.patch_provider import FakePatchProposalProvider
from app.patch_proposals import create_patch_proposal
from app.planner import TestPlanner
from app.providers.fake import FakeAIProvider
from app.repository import (
    InMemoryDiagnosticRepository,
    MongoDiagnosticRepository,
    RepositoryUnavailableError,
)
from app.schemas import PatchProposalRequest
from app.sessions import create_session, submit_evidence
from tests.conftest import load_evidence

catalog = get_default_catalog()
provider = FakeAIProvider()
planner = TestPlanner()
patch_provider = FakePatchProposalProvider()


# ---------------------------------------------------------------------------
# Fake pymongo-shaped collection/database - never touches a real network.
# ---------------------------------------------------------------------------


class _FakeCollection:
    def __init__(self) -> None:
        self._docs: dict[str, dict] = {}

    def insert_one(self, doc: dict) -> None:
        self._docs[doc["_id"]] = doc

    def replace_one(self, filter: dict, doc: dict, upsert: bool = False) -> None:
        self._docs[filter["_id"]] = doc

    def find_one(self, filter: dict) -> dict | None:
        return self._docs.get(filter["_id"])

    def find(self, filter: dict | None = None) -> list[dict]:
        filter = filter or {}
        return [
            doc for doc in self._docs.values() if all(doc.get(k) == v for k, v in filter.items())
        ]


class _FakeDatabase:
    def __init__(self) -> None:
        self.sessions = _FakeCollection()
        self.patch_proposals = _FakeCollection()
        self.events = _FakeCollection()


class _FailingCollection:
    def insert_one(self, doc):
        raise ConnectionError("simulated Mongo outage")

    def replace_one(self, *a, **k):
        raise ConnectionError("simulated Mongo outage")

    def find_one(self, *a, **k):
        raise ConnectionError("simulated Mongo outage")

    def find(self, *a, **k):
        raise ConnectionError("simulated Mongo outage")


class _FailingDatabase:
    sessions = _FailingCollection()
    patch_proposals = _FailingCollection()
    events = _FailingCollection()


def _sample_session():
    return create_session(load_evidence("hc_sr04_trig_and_echo_missing"), "hc-sr04", catalog, provider, planner)


# ---------------------------------------------------------------------------
# 1: InMemoryDiagnosticRepository preserves M5 behavior
# ---------------------------------------------------------------------------


def test_in_memory_repository_create_get_save_list() -> None:
    repo = InMemoryDiagnosticRepository()
    session = _sample_session()

    repo.create_session(session)
    assert repo.get_session(session.session_id) == session
    assert repo.get_session("does-not-exist") is None

    updated, _ = submit_evidence(session, load_evidence("hc_sr04_healthy"), catalog, provider, planner)
    repo.save_session(updated)
    assert repo.get_session(session.session_id).steps[-1].step_number == 2
    assert session in repo.list_sessions() or updated in repo.list_sessions()  # same session_id
    assert len(repo.list_sessions()) == 1


def test_in_memory_repository_patch_proposal_and_event_storage() -> None:
    repo = InMemoryDiagnosticRepository()
    session = _sample_session()
    repo.create_session(session)

    proposal = create_patch_proposal(
        session,
        PatchProposalRequest(target_probe="P2", target_role="TRIG", patch_type="TEMPORARY_SIGNAL_EMULATION"),
        patch_provider,
    )
    repo.save_patch_proposal(proposal)
    assert repo.get_patch_proposal(proposal.proposal_id) == proposal
    assert repo.list_patch_proposals(session.session_id) == [proposal]
    assert repo.list_patch_proposals("some-other-session") == []

    event = new_event(session.session_id, "PATCH_PROPOSED", {"proposal_id": proposal.proposal_id})
    repo.append_event(event)
    assert repo.list_events(session.session_id) == [event]
    assert repo.list_events("some-other-session") == []


# ---------------------------------------------------------------------------
# 3-6: round-trip fidelity (ordered steps, provenance, PATCH proposals,
# VerificationResults) through serialization - exercised via the Mongo
# adapter's model_dump/model_validate round trip, since that is the code
# path that actually serializes to and deserializes from a document store.
# ---------------------------------------------------------------------------


def test_ordered_diagnostic_steps_survive_round_trip() -> None:
    session = _sample_session()
    session, _ = submit_evidence(session, load_evidence("hc_sr04_healthy"), catalog, provider, planner)

    repo = MongoDiagnosticRepository(settings=None, database=_FakeDatabase())
    repo.save_session(session)
    restored = repo.get_session(session.session_id)

    assert [s.step_number for s in restored.steps] == [1, 2]
    assert restored.steps[0].submitted_evidence == session.steps[0].submitted_evidence
    assert restored == session


def test_provenance_survives_round_trip() -> None:
    session = _sample_session()
    repo = MongoDiagnosticRepository(settings=None, database=_FakeDatabase())
    repo.save_session(session)
    restored = repo.get_session(session.session_id)

    original_provenances = {r.provenance for r in session.steps[0].evaluated_evidence.rule_results}
    restored_provenances = {r.provenance for r in restored.steps[0].evaluated_evidence.rule_results}
    assert original_provenances == restored_provenances
    assert "SPECIFICATION" in restored_provenances


def test_patch_proposals_survive_round_trip() -> None:
    session = _sample_session()
    proposal = create_patch_proposal(
        session,
        PatchProposalRequest(target_probe="P2", target_role="TRIG", patch_type="TEMPORARY_SIGNAL_EMULATION"),
        patch_provider,
    )
    repo = MongoDiagnosticRepository(settings=None, database=_FakeDatabase())
    repo.save_patch_proposal(proposal)
    restored = repo.get_patch_proposal(proposal.proposal_id)
    assert restored == proposal
    assert repo.list_patch_proposals(session.session_id) == [proposal]


def test_verification_results_survive_round_trip() -> None:
    from app.patch_proposals import record_external_result, verify_proposal
    from app.schemas import RecordExternalResultRequest

    session = _sample_session()
    proposal = create_patch_proposal(
        session,
        PatchProposalRequest(target_probe="P2", target_role="TRIG", patch_type="TEMPORARY_SIGNAL_EMULATION"),
        patch_provider,
    )
    approved = record_external_result(proposal, RecordExternalResultRequest(external_status="APPROVED_EXTERNALLY"))
    executed = record_external_result(approved, RecordExternalResultRequest(external_status="EXECUTED_EXTERNALLY"))
    _, verified_proposal, verification, _ = verify_proposal(
        session, executed, load_evidence("hc_sr04_healthy"), catalog, provider, planner
    )
    assert verification is not None

    repo = MongoDiagnosticRepository(settings=None, database=_FakeDatabase())
    repo.save_patch_proposal(verified_proposal)
    restored = repo.get_patch_proposal(verified_proposal.proposal_id)
    assert restored.verification == verification
    assert restored.status == "VERIFIED"


# ---------------------------------------------------------------------------
# 7-8: Mongo adapter document structure + read reconstruction
# ---------------------------------------------------------------------------


def test_mongo_adapter_serializes_correct_document_structure() -> None:
    session = _sample_session()
    database = _FakeDatabase()
    repo = MongoDiagnosticRepository(settings=None, database=database)
    repo.save_session(session)

    stored_doc = database.sessions.find_one({"_id": session.session_id})
    assert stored_doc["_id"] == session.session_id
    assert stored_doc["probe"] == "P3"
    assert stored_doc["component_id"] == "hc-sr04"
    assert isinstance(stored_doc["steps"], list)
    assert stored_doc["steps"][0]["step_number"] == 1


def test_mongo_read_reconstructs_valid_session() -> None:
    session = _sample_session()
    repo = MongoDiagnosticRepository(settings=None, database=_FakeDatabase())
    repo.save_session(session)

    restored = repo.get_session(session.session_id)
    assert restored is not None
    assert restored.session_id == session.session_id
    assert restored.steps[0].result.outcome == session.steps[0].result.outcome


# ---------------------------------------------------------------------------
# 9-10: Mongo failure surfaced / missing config fails clearly
# ---------------------------------------------------------------------------


def test_mongo_failure_is_surfaced_not_swallowed() -> None:
    repo = MongoDiagnosticRepository(settings=None, database=_FailingDatabase())
    session = _sample_session()
    try:
        repo.save_session(session)
        assert False, "expected RepositoryUnavailableError"
    except RepositoryUnavailableError:
        pass

    try:
        repo.get_session("anything")
        assert False, "expected RepositoryUnavailableError"
    except RepositoryUnavailableError:
        pass


def test_mongo_repository_without_uri_fails_clearly() -> None:
    from app.config import ProbeSettings

    settings = ProbeSettings(diagnostic_repository="mongodb", mongodb_uri=None)
    try:
        MongoDiagnosticRepository(settings)
        assert False, "expected RuntimeError for missing MONGODB_URI"
    except RuntimeError as exc:
        assert "MONGODB_URI" in str(exc)


def test_get_repository_selects_mongo_and_fails_without_uri(monkeypatch) -> None:
    from app.config import get_settings
    from app.repository import get_repository

    get_settings.cache_clear()
    get_repository.cache_clear()
    monkeypatch.setenv("DIAGNOSTIC_REPOSITORY", "mongodb")
    monkeypatch.delenv("MONGODB_URI", raising=False)
    try:
        get_repository()
        assert False, "expected RuntimeError"
    except RuntimeError as exc:
        assert "MONGODB_URI" in str(exc)
    finally:
        get_settings.cache_clear()
        get_repository.cache_clear()


def test_get_repository_defaults_to_memory(monkeypatch) -> None:
    from app.config import get_settings
    from app.repository import get_repository

    get_settings.cache_clear()
    get_repository.cache_clear()
    monkeypatch.delenv("DIAGNOSTIC_REPOSITORY", raising=False)
    repo = get_repository()
    assert isinstance(repo, InMemoryDiagnosticRepository)
    get_settings.cache_clear()
    get_repository.cache_clear()
