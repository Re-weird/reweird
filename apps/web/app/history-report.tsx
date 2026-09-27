"use client";

import { useEffect, useState } from "react";
import { motion } from "framer-motion";
import { Download, FileText, Printer, RefreshCw, ShieldCheck } from "lucide-react";
import type { DetailedReport, HistoryDetail, HistoryStatus, HistorySummary } from "@reweird/shared-types";
import { historyApi } from "@/lib/api";
import { GitSyncPanel } from "./git-sync";

const statuses: HistoryStatus[] = ["OPEN", "TESTING", "WAITING_FOR_USER", "VERIFYING", "RESOLVED", "IMPROVED", "UNRESOLVED", "CANCELLED", "INCONCLUSIVE"];
const date = (value?: number) => value ? new Date(value).toLocaleString() : "—";

// With `projectID`, the view is locked to that project: the project picker is
// hidden and only its sessions/reports can be listed or opened.
export function HistoryReportView({ mode, projectID: scopedProjectID }: { mode: "history" | "reports"; projectID?: string }) {
  const [items, setItems] = useState<HistorySummary[]>([]);
  const [projects, setProjects] = useState<Array<{ id: string; name: string }>>([]);
  const [projectID, setProjectID] = useState(scopedProjectID ?? "");
  const [status, setStatus] = useState<HistoryStatus | "">("");
  const [sort, setSort] = useState<"newest" | "oldest">("newest");
  const [selectedID, setSelectedID] = useState("");
  const [detail, setDetail] = useState<HistoryDetail | null>(null);
  const [report, setReport] = useState<DetailedReport | null>(null);
  const [error, setError] = useState("");
  const [loading, setLoading] = useState(false);

  useEffect(() => { if (scopedProjectID !== undefined) setProjectID(scopedProjectID); }, [scopedProjectID]);
  useEffect(() => {
    if (scopedProjectID !== undefined) return;
    historyApi.list().then((response) => {
      setProjects(Array.from(new Map(response.items.map((item) => [item.project_id, { id: item.project_id, name: item.project_name }])).values()));
    }).catch(() => undefined);
  }, [scopedProjectID]);
  useEffect(() => {
    let live = true;
    setLoading(true);
    historyApi.list({ projectID, status, sort }).then((response) => {
      if (!live) return;
      setItems(response.items);
      setSelectedID((old) => response.items.some((item) => item.id === old) ? old : (response.items[0]?.id ?? ""));
      setError("");
    }).catch((cause) => { if (live) setError(cause instanceof Error ? cause.message : "History is unavailable."); }).finally(() => { if (live) setLoading(false); });
    return () => { live = false; };
  }, [projectID, status, sort]);
  useEffect(() => {
    if (!selectedID) { setDetail(null); setReport(null); return; }
    let live = true;
    Promise.all([historyApi.detail(selectedID), historyApi.report(selectedID)]).then(([nextDetail, nextReport]) => {
      if (live) { setDetail(nextDetail); setReport(nextReport); setError(""); }
    }).catch((cause) => { if (live) setError(cause instanceof Error ? cause.message : "Session details are unavailable."); });
    return () => { live = false; };
  }, [selectedID]);

  return <div className="history-page">
    <section className="page-heading"><div><p className="kicker">Persisted diagnostic evidence</p><h1>{mode === "history" ? "Diagnostic history" : "Diagnostic reports"}</h1><p>Browse stored guided tests and deterministic reports. Measured facts, derived facts, user actions, and VERIFY remain distinct.</p></div><div className="live-badge"><span /> {loading ? "Loading" : `${items.length} sessions`}</div></section>
    {error && <div className="form-error page-error">{error}</div>}
    <section className="panel history-controls">
      {scopedProjectID === undefined && <label>Project<select value={projectID} onChange={(event) => setProjectID(event.target.value)}><option value="">All projects</option>{projects.map((item) => <option key={item.id} value={item.id}>{item.name}</option>)}</select></label>}
      <label>Status<select value={status} onChange={(event) => setStatus(event.target.value as HistoryStatus | "")}><option value="">All statuses</option>{statuses.map((item) => <option key={item} value={item}>{item.replaceAll("_", " ")}</option>)}</select></label>
      <label>Order<select value={sort} onChange={(event) => setSort(event.target.value as "newest" | "oldest")}><option value="newest">Newest first</option><option value="oldest">Oldest first</option></select></label>
    </section>
    <section className="history-layout">
      <motion.div className="panel history-list" initial="hidden" animate="show" variants={{ hidden: {}, show: { transition: { staggerChildren: 0.04 } } }}>{items.length === 0 ? <div className="empty-state"><FileText size={24} /><h2>No diagnostic sessions</h2><p>Run a guided test to create a real, persisted history entry. Demo cards do not become evidence.</p></div> : items.map((item) => <motion.button variants={{ hidden: { opacity: 0, x: -8 }, show: { opacity: 1, x: 0, transition: { duration: 0.25, ease: [0.16, 1, 0.3, 1] } } }} key={item.id} className={`history-item ${selectedID === item.id ? "selected" : ""}`} onClick={() => setSelectedID(item.id)}><span><strong>{item.project_name}</strong><small>{date(item.started_at_ms)} · Profile revision {item.profile_version}</small><em>{item.original_problem}</em></span><b className={`history-status status-${item.status.toLowerCase()}`}>{item.status.replaceAll("_", " ")}</b></motion.button>)}</motion.div>
      <div className="history-main">{detail && mode === "history" && <section className="panel history-detail"><span className="eyebrow">Session {detail.summary.id}</span><h2>{detail.summary.project_name}</h2><div className="spec-chips"><span>Profile {detail.summary.profile_id} · revision {detail.summary.profile_version}</span><span>{detail.summary.telemetry_source || "No capture"}</span><span>{detail.summary.status}</span></div><p>{detail.summary.original_problem}</p><h3>Timeline</h3><div className="history-timeline">{detail.timeline.map((event) => <div key={event.id}><time>{date(event.timestamp_ms)}</time><span><strong>{event.kind.replaceAll("_", " ")}</strong><small>{event.description}</small><em>{event.provenance}{event.window_id ? ` · window #${event.window_id}` : ""}</em></span></div>)}</div>{detail.workflow.verification && <div className="history-verify"><strong>VERIFY: {detail.workflow.verification.status}</strong><p>{detail.workflow.verification.summary}</p><small>Before #{detail.workflow.verification.before_window_id} → after #{detail.workflow.verification.after_window_id}</small></div>}</section>}
      {report && <section className="panel report-preview"><div className="report-toolbar"><div><span className="eyebrow">Deterministic report</span><h2>{report.report_id}</h2></div><div><a className="secondary" href={historyApi.downloadURL(selectedID, "json")}><Download size={15} /> JSON</a><a className="secondary" href={historyApi.downloadURL(selectedID, "md")}><Download size={15} /> Markdown</a><button className="secondary" onClick={() => window.print()}><Printer size={15} /> Print</button></div></div><p>{report.summary}</p><div className="report-meta"><span>Project {report.project_name}</span><span>Controller {report.controller || "Not specified"}</span><span>Profile revision {report.profile_revision}</span><span>Status {report.final_status}</span></div><h3>Observed behavior</h3><p>{report.observed_behavior}</p><h3>Expected behavior</h3><p>{report.expected_behavior || "Not specified in the stored profile."}</p><h3>Measured evidence</h3><div className="report-facts">{report.measured_evidence.map((fact, index) => <div key={`${fact.window_id}-${fact.probe}-${fact.metric}-${index}`}><strong>{fact.probe || "Capture"} {fact.metric.replaceAll("_", " ")}</strong><span>{String(fact.value)} {fact.unit}</span><small>Window #{fact.window_id} · {fact.provenance}</small></div>)}</div><h3>Derived evidence</h3><div className="report-facts">{report.derived_evidence.map((fact, index) => <div key={`${fact.window_id}-${fact.probe}-${fact.metric}-${index}`}><strong>{fact.probe} {fact.metric.replaceAll("_", " ")}</strong><span>{String(fact.value)} {fact.unit}</span><small>Window #{fact.window_id} · {fact.provenance}</small></div>)}</div><h3>Tests and actions</h3>{report.test_results.map((result) => <p key={result.test_id}>{result.test_type.replaceAll("_", " ")}: {result.result} — {result.interpretation}</p>)}{(report.user_actions ?? []).map((action) => <p key={action.id}>User reported action · {action.description}</p>)}<h3>Before / after · VERIFY</h3><p>Window #{report.before_window_id ?? "—"} → #{report.after_window_id ?? "—"}. {report.verify_result ? `${report.verify_result.status}: ${report.verify_result.summary}` : "VERIFY not completed."}</p><h3>Unresolved items</h3>{report.unresolved_items.map((item) => <p key={item}>{item}</p>)}<div className="security-note"><ShieldCheck size={18} /><div><strong>Provenance and export security</strong><span>{report.provenance.join(" · ")}. {report.security_redaction_count ? `${report.security_redaction_count} potential secrets redacted.` : "No known secret pattern found in this report."} Uploaded source code is omitted.</span></div></div></section>}</div>
    </section>
    {report && <GitSyncPanel workflowID={selectedID} />}
  </div>;
}
