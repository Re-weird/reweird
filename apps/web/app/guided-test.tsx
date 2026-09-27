"use client";

import { useState } from "react";
import { Activity, CheckCircle2, RefreshCw, ShieldCheck, TriangleAlert } from "lucide-react";
import type { DiagnosticWorkflow, TestRecommendation } from "@reweird/shared-types";
import { testHeadline, verifyHeadline } from "@/lib/weird-demo";

type Props = {
  workflow: DiagnosticWorkflow | null;
  recommendation: TestRecommendation | null;
  busy: boolean;
  error: string | null;
  onPlan: () => void;
  onStart: () => void;
  onCapture: () => void;
  onRemeasure: () => void;
  onCancel: () => void;
  onRecordAction: (description: string) => Promise<void>;
  onHealth?: () => void;
};

function value(value: unknown) {
  if (typeof value === "number") return Number.isInteger(value) ? String(value) : value.toFixed(2);
  if (Array.isArray(value)) return value.join(", ");
  return value == null ? "—" : String(value);
}

export function GuidedTestView({ workflow, recommendation, busy, error, onPlan, onStart, onCapture, onRemeasure, onCancel, onRecordAction, onHealth }: Props) {
  const [actionNote, setActionNote] = useState("");
  const plan = workflow?.plan;
  const status = workflow?.status;
  const result = workflow?.result;
  const verification = workflow?.verification;
  const active = status === "PLANNED" || status === "READY" || status === "WAITING_FOR_USER";
  return <>
    <div className={`weird-result-banner ${verification ? verification.status === "RESOLVED" && status === "RESOLVED" ? "recovered" : "unresolved" : ""}`} aria-live="polite"><span className="eyebrow">Guided investigation · {workflow?.baseline?.source ?? "no capture yet"}</span><h2>{verifyHeadline(workflow) ?? testHeadline(workflow) ?? (status === "WAITING_FOR_USER" ? "TEST IN PROGRESS." : "I HAVE A THEORY. LET’S TEST IT.")}</h2><p>{verification ? verification.summary : result ? result.interpretation : status === "WAITING_FOR_USER" ? `ReWeird is watching ${plan?.recommendation.target_probes.join(", ") ?? "the selected probes"}. Follow only the plan’s safe instructions, then capture the test window.` : plan?.recommendation.reason ?? recommendation?.reason ?? "Ask the test planner for the next useful measurement."}</p></div>
    <section className="page-heading"><div><p className="kicker">Guided test · Passive sensing</p><h1>{plan?.title ?? "Choose the next useful measurement"}</h1><p>{plan?.recommendation.reason ?? recommendation?.reason ?? "Ask the deterministic test planner for a recommendation based on the current structured evidence."}</p></div><div className="live-badge"><span /> {status ?? "Not planned"}</div></section>
    {error && <div className="form-error page-error"><TriangleAlert size={16} />{error}</div>}
    {!workflow && <section className="panel guided-panel"><h2>Current recommendation</h2><p>{recommendation ? `${recommendation.test_type.replaceAll("_", " ")} · ${recommendation.target_probes.join(", ")}` : "No recommendation is available yet. Connect to the API and analyze a matching profile."}</p><button className="primary" disabled={busy || !recommendation} onClick={onPlan}>Create guided plan</button></section>}
    {workflow && <>
      <section className="guided-grid">
        <div className="panel guided-panel"><span className="eyebrow">Plan</span><h2>{plan?.title}</h2><p>{plan?.criteria}</p><div className="spec-chips">{plan?.monitoring.map((item) => <span key={item}>{item}</span>)}</div><h3>Instructions</h3><ol>{plan?.instructions.map((instruction, index) => <li key={index}>{instruction}</li>)}</ol></div>
        <div className="panel guided-panel"><span className="eyebrow">Capture</span><h2>{status?.replaceAll("_", " ")}</h2><p>Planned window: {plan?.window_ms ? `${plan.window_ms / 1000} seconds` : "—"}. Actual captured window durations are shown below.</p><div className="guided-window-list">{([["Before", workflow.baseline], ["During", workflow.during], ["After", workflow.after]] as const).map(([label, window]) => <div key={label}><strong>{label}</strong><span>{window ? `#${window.id} · ${window.analysis.window_ms / 1000}s · ${window.source}` : "Not captured"}</span></div>)}</div><p>Metrics: {plan?.metrics.join(", ")}</p>{plan?.unavailable && <div className="form-error"><TriangleAlert size={16} />{plan.unavailable}</div>}</div>
      </section>
      <section className="guided-actions">
        {status === "PLANNED" && <button className="primary" disabled={busy} onClick={onStart}><Activity size={16} /> Capture before window</button>}
        {(status === "WAITING_FOR_USER" || status === "READY") && <button className="primary" disabled={busy} onClick={onCapture}><Activity size={16} /> {status === "WAITING_FOR_USER" ? "I performed the guided action — capture during" : "Capture test window"}</button>}
        {(status === "COMPLETED" || status === "INCONCLUSIVE" || status === "UNRESOLVED") && workflow.during && <button className="primary" disabled={busy} onClick={onRemeasure}><RefreshCw size={16} /> {workflow.scenario_id ? "Simulate correction & VERIFY" : "Capture after window & VERIFY"}</button>}
        {active && <button className="secondary" disabled={busy} onClick={onCancel}>Cancel test</button>}
        {(status === "RESOLVED" || status === "UNRESOLVED" || status === "CANCELLED" || status === "LOCKED" || status === "FAILED") && <button className="secondary" disabled={busy || !recommendation} onClick={onPlan}>Plan another test</button>}
      </section>
      {(status === "WAITING_FOR_USER" || status === "COMPLETED" || status === "UNRESOLVED") && <section className="panel guided-panel"><span className="eyebrow">User-reported action</span><h2>Record what you did</h2><p>This is your report, not sensor-verified execution. Do not include credentials.</p><div className="guided-action-note"><input value={actionNote} maxLength={500} onChange={(event) => setActionNote(event.target.value)} placeholder="e.g. Reseated the connector before re-measurement" aria-label="User action note" /><button className="secondary" disabled={busy || !actionNote.trim()} onClick={async () => { await onRecordAction(actionNote.trim()); setActionNote(""); }}>Save action</button></div>{workflow.user_actions?.map((action) => <p key={action.id}>User reported action · {action.description}</p>)}</section>}
      {result && <section className="panel guided-panel"><span className="eyebrow">Test result · {result.test_type.replaceAll("_", " ")}</span><p>{result.derived_metrics.evaluation_phase === "BEFORE_AFTER_RECOVERY"
        ? `Recovery criteria · Before #${value(result.derived_metrics.before_window_id)} → After #${value(result.derived_metrics.after_window_id)} · During #${value(result.derived_metrics.during_window_id)} is intermediate evidence · Profile revision ${value(result.derived_metrics.criteria_profile_version)} · ${value(result.derived_metrics.capture_source)}`
        : `Guided-test criteria · Before #${workflow.baseline?.id ?? "—"} → During #${workflow.during?.id ?? "—"}. VERIFY separately evaluates recovery; a correlation/activity test is not a repair verdict.`}</p><h2>{result.result.replaceAll("_", " ")}</h2><p>{result.interpretation}</p><div className="guided-observations">{result.observations.map((observation, index) => <div key={`${observation.probe}-${observation.metric}-${index}`}><strong>{observation.probe} {observation.metric.replaceAll("_", " ")}</strong><span>{value(observation.value)} {observation.unit}</span><small>{observation.provenance}</small></div>)}</div><small>Evidence: {result.evidence_provenance.join(" · ")} · Confidence {Math.round(result.confidence * 100)}%</small></section>}
      {verification && <section className="panel guided-panel"><span className="eyebrow">Generic VERIFY · windows #{verification.before_window_id} → #{verification.after_window_id}</span><h2><CheckCircle2 size={19} /> {verification.status}</h2><p>{verification.summary}</p><div className="guided-observations">{verification.changes.map((change, index) => <div key={`${change.probe}-${change.metric}-${index}`}><strong>{change.probe} {change.metric.replaceAll("_", " ")}</strong><span>{value(change.before)} → {value(change.after)} {change.unit}</span></div>)}</div>{verification.remaining_issues.length > 0 && <><h3>Remaining issues</h3><ul>{verification.remaining_issues.map((issue) => <li key={issue}>{issue}</li>)}</ul></>}{verifyHeadline(workflow) === "NOT WEIRD ANYMORE." && onHealth && <button className="primary" onClick={onHealth}>View device health</button>}</section>}
      <section className="security-note"><ShieldCheck size={20} /><div><strong>PATCH remains locked</strong><span>This workflow only reads passive probe measurements. A recommendation requiring active output is shown as unavailable.</span></div></section>
    </>}
  </>;
}
