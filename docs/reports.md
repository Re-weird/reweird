# Deterministic diagnostic reports

`GET /api/v1/reports/:id` exports JSON from a persisted guided workflow.
`?format=md` exports Markdown; `?download=1` sets a download filename. The
History/Reports UI renders a printable preview. This path does not call Gemini.
The pre-existing `/api/v1/report` compatibility endpoint is unchanged.

Reports separate measured capture metadata, derived signal facts, Project
Profile expectations, trusted baseline facts, guided test results, user
actions, and VERIFY outcomes. They omit uploaded code and do not read `.env`
files or raw credentials. Every string is scanned for common API-key,
private-key, bearer-token, password, and credential-in-URL patterns before
export. Matches are replaced with a visible redaction marker and counted.
Generated reports are limited to 1 MiB; oversize reports fail rather than
being silently cut off. The later Git preview blocks synchronization if a
potential secret was detected before redaction.

The existing demo session report and this persisted evidence report are
different contracts. Only the latter forms diagnostic history.
