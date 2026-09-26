# Diagnostic history

History reuses the persisted `test_workflows` records and their embedded before,
during, and after measurement windows. It does not create another diagnostic
session store. A workflow is pinned to the confirmed Project Profile revision
and now stores a profile snapshot so later edits cannot rewrite historical
expectations. Older workflow rows remain readable but may lack that snapshot.

`GET /api/v1/history` supports `project_id`, `status`, `sort=newest|oldest`, and
`limit` (1–100). `GET /api/v1/history/:id` returns the workflow and a timeline
assembled from persisted capture timestamps, test result, user actions, and
VERIFY result. The API refuses to silently truncate a history scan larger than
2000 workflows; an archival/pagination strategy is needed before that scale.

`POST /api/v1/tests/:id/actions` records a one-line, up-to-500-character user
note during a guided test or before re-measurement. The note is labeled `USER`,
not measured evidence. Known secret patterns are rejected. Results are mapped
to OPEN, TESTING, WAITING_FOR_USER, VERIFYING, RESOLVED, IMPROVED, UNRESOLVED,
CANCELLED, and INCONCLUSIVE. No status is inferred from Gemini.
