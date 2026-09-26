"use client";

import { useEffect, useState } from "react";
import { Activity, ArrowUpRight, CheckCircle2, CircleAlert, Cpu, Fingerprint, History, RefreshCw, Save, ShieldCheck } from "lucide-react";
import type { DemoSession, DevicePassport, HistoryDetail, PassportCapture, PassportStatus, ProbePlan, Project, ProjectProfile } from "@reweird/shared-types";
import { ApiError, historyApi, passportApi } from "@/lib/api";
import { CircuitMap } from "./circuit-map";

const statusLabels: Record<PassportStatus, string> = {
  NO_PHYSICAL_BASELINE: "No physical baseline yet",
  NEEDS_VERIFICATION: "Needs verification",
  HEALTHY: "Healthy — physical comparison",
  DEVIATION_DETECTED: "Deviation detected",
  SIMULATED_BASELINE: "Simulated baseline saved",
  SIMULATED_MATCH: "Simulated match",
  SIMULATED_DEVIATION: "Simulated deviation",
};

function dateLabel(timestamp: number): string {
  return new Date(timestamp).toLocaleString();
}

function metric(value: number | undefined, unit: string): string {
  return value == null ? "—" : `${Number(value.toFixed(2))} ${unit}`;
}

function captureLabel(capture: PassportCapture): string {
  return `#${capture.measurement_id} · ${capture.source === "PHYSICAL" ? "ESP32 serial" : "Simulator"} · ${dateLabel(capture.captured_at_ms || capture.ingested_at_ms)}`;
}

export function DevicePassportView({
  profile,
  project,
  plan,
  session,
  onNavigate,
}: {
  profile: ProjectProfile | null;
  project: Project | null;
  plan: ProbePlan | null;
  session: DemoSession;
  onNavigate: (destination: "profile" | "connect" | "live" | "diagnosis" | "guided" | "history") => void;
}) {
  const [passport, setPassport] = useState<DevicePassport | null>(null);
  const [loading, setLoading] = useState(false);
  const [saving, setSaving] = useState(false);
  const [error, setError] = useState("");
  const [selectedID, setSelectedID] = useState<number | null>(null);
  const [confirmed, setConfirmed] = useState(false);
  const [note, setNote] = useState("");
  const [story, setStory] = useState<HistoryDetail[]>([]);

  useEffect(() => {
    if (!profile?.confirmed) return;
    let cancelled = false;
    setLoading(true);
    passportApi.get(profile.id).then(async (result) => {
      if (!cancelled) { setPassport(result); setError(""); }
      const details = await Promise.allSettled(result.history.slice(0, 8).map((item) => historyApi.detail(item.id)));
      if (!cancelled) setStory(details.filter((entry): entry is PromiseFulfilledResult<HistoryDetail> => entry.status === "fulfilled").map((entry) => entry.value));
    }).catch((cause: unknown) => {
      if (!cancelled) setError(cause instanceof Error ? cause.message : "Passport could not be loaded.");
    }).finally(() => { if (!cancelled) setLoading(false); });
    return () => { cancelled = true; };
  }, [profile?.id, profile?.confirmed]);

  if (!profile?.confirmed) return <section className="empty-state panel"><Fingerprint size={30} /><h2>No confirmed Device Passport yet</h2><p>Confirm the Project Profile before collecting a Known Good capture.</p><button className="primary" onClick={() => onNavigate("profile")}>Review Project Profile</button></section>;

  const captures = passport?.recent_captures ?? [];
  const selected = captures.find((capture) => capture.measurement_id === selectedID) ?? captures[0];
  const alreadySaved = Boolean(selected && passport?.known_good.some((record) => record.measurement_id === selected.measurement_id));
  const physicalBaseline = passport?.physical_baseline;
  const simulatedBaseline = passport?.simulated_baseline;

  async function refresh() {
    if (!profile) return;
    setLoading(true); setError("");
    try { const updated = await passportApi.get(profile.id); setPassport(updated); const details = await Promise.allSettled(updated.history.slice(0, 8).map((item) => historyApi.detail(item.id))); setStory(details.filter((entry): entry is PromiseFulfilledResult<HistoryDetail> => entry.status === "fulfilled").map((entry) => entry.value)); }
    catch (cause) { setError(cause instanceof Error ? cause.message : "Passport could not be loaded."); }
    finally { setLoading(false); }
  }

  async function save() {
    if (!profile || !selected || !confirmed || alreadySaved) return;
    setSaving(true); setError("");
    try {
      await passportApi.saveKnownGood(profile.id, selected.measurement_id, note);
      setPassport(await passportApi.get(profile.id));
      setConfirmed(false);
      setNote("");
    } catch (cause) {
      setError(cause instanceof ApiError || cause instanceof Error ? cause.message : "Could not save Known Good.");
    } finally { setSaving(false); }
  }

  const events = story.flatMap((detail) => detail.timeline.map((event) => ({ ...event, source: detail.summary.telemetry_source || "Source not recorded" })))
    .concat((passport?.known_good ?? []).map((baseline) => ({ id: `baseline-${baseline.id}`, timestamp_ms: baseline.saved_at_ms, kind: "KNOWN_GOOD", description: `User marked capture #${baseline.measurement_id} as Known Good.`, provenance: "USER" as const, source: baseline.source })))
    .sort((a, b) => b.timestamp_ms - a.timestamp_ms).slice(0, 18);
  const supportedFindings = story.filter((detail) => detail.workflow.result && ["POSITIVE_CORRELATION", "SHARED_FAILURE_PATTERN", "RAIL_OUTSIDE_TOLERANCE", "EXPECTED_ACTIVITY_MISSING", "TIMING_OUTSIDE_SPECIFICATION", "BASELINE_DEVIATION"].includes(detail.workflow.result.result)).length;

  return <div className="passport-page">
    <section className="page-heading"><div><p className="kicker">Persistent device record</p><h1>Device Passport</h1><p>Project identity, intended circuit, user-marked Known Good captures, diagnostic history, and verified outcomes in one place.</p></div><button className="secondary" onClick={refresh} disabled={loading}><RefreshCw size={15} /> Refresh passport</button></section>
    {error && <div className="form-error page-error" role="alert"><CircleAlert size={15} />{error}</div>}
    {loading && !passport && <div className="panel empty-state"><RefreshCw size={24} /><p>Loading saved device record…</p></div>}
    {passport && <>
      <section className="passport-hero panel">
        <div className="passport-identity"><span className="passport-mark"><Fingerprint size={27} /></span><div><span className="eyebrow">Project identity</span><h2>{passport.profile.project_name}</h2><p>{passport.profile.controller} · {passport.profile.logic_voltage} V logic · Profile revision {passport.profile.version}</p><div className="spec-chips"><span>{passport.profile.components.length} components</span><span>{passport.profile.connections?.length ?? 0} intended connections</span><span>{passport.probe_plan?.connected ? "Probe placement user-confirmed" : "Probe placement not confirmed"}</span></div></div></div>
        <div className={`passport-status ${passport.status.toLowerCase()}`}><span className="eyebrow">Last observed status</span><strong>{statusLabels[passport.status]}</strong><p>{passport.status_detail}</p>{passport.recent_captures[0] && <small>Latest stored capture: {dateLabel(passport.recent_captures[0].captured_at_ms || passport.recent_captures[0].ingested_at_ms)}</small>}</div>
      </section>

      <section className="passport-grid">
        <div className="panel passport-baselines"><div className="panel-heading"><div><span className="eyebrow">Electrical fingerprint</span><h2>Known Good baselines</h2></div><ShieldCheck size={17} /></div>
          <div className="baseline-cards">
            <div className="baseline-card physical"><span>PHYSICAL · ESP32 SERIAL</span><strong>{physicalBaseline ? `Capture #${physicalBaseline.measurement_id}` : "No physical baseline yet"}</strong><small>{physicalBaseline ? `${physicalBaseline.device_id} · ${dateLabel(physicalBaseline.captured_at_ms || physicalBaseline.ingested_at_ms)}` : "Requires matching serial telemetry and your explicit healthy confirmation."}</small></div>
            <div className="baseline-card simulated"><span>SIMULATED · TEST HARNESS</span><strong>{simulatedBaseline ? `Capture #${simulatedBaseline.measurement_id}` : "No simulated baseline saved"}</strong><small>{simulatedBaseline ? `${simulatedBaseline.device_id} · ${dateLabel(simulatedBaseline.captured_at_ms || simulatedBaseline.ingested_at_ms)}` : "Simulator records never establish physical health."}</small></div>
          </div>
          {passport.known_good.length > 0 && <div className="passport-record-list">{passport.known_good.slice(0, 10).map((record) => <div key={record.id}><span className={`passport-source ${record.source.toLowerCase()}`}>{record.source}</span><div><strong>Capture #{record.measurement_id}</strong><small>{record.probes.length} probes · {dateLabel(record.saved_at_ms)} · User reported healthy</small>{record.note && <small>{record.note}</small>}</div></div>)}</div>}
        </div>

        <div className="panel passport-save"><div className="panel-heading"><div><span className="eyebrow">Explicit user confirmation</span><h2>Save as Known Good</h2></div><Save size={17} /></div><p>Choose a persisted, matching capture. ReWeird checks its configured probe behavior before saving your healthy report. This does not certify the hardware.</p>
          {captures.length ? <><label>Capture<select value={selected?.measurement_id ?? ""} onChange={(event) => { setSelectedID(Number(event.target.value)); setConfirmed(false); }}>{captures.map((capture) => <option key={capture.measurement_id} value={capture.measurement_id}>{captureLabel(capture)}</option>)}</select></label>
            {selected && <div className={`passport-source-note ${selected.source.toLowerCase()}`}>{selected.source === "SIMULATED" ? "SIMULATED: this baseline will only compare against simulator captures. It never becomes a trusted physical baseline." : "PHYSICAL: saved from stored ESP32 serial telemetry for this profile and device."}</div>}
            {selected && <div className="passport-metrics">{selected.probes.filter((probe) => probe.role !== "UNASSIGNED").map((probe) => <div key={probe.probe}><strong>{probe.probe} · {probe.role}</strong><span>{metric(probe.average_voltage, "V")} · {metric(probe.frequency_hz, "Hz")} · {probe.dropout_events} dropouts</span></div>)}</div>}
            <label>Note (optional)<input value={note} maxLength={500} onChange={(event) => setNote(event.target.value)} placeholder="What was working correctly?" /></label>
            <label className="passport-confirm"><input type="checkbox" checked={confirmed} onChange={(event) => setConfirmed(event.target.checked)} /><span>{selected?.source === "SIMULATED" ? "I confirm this simulated behavior is the expected reference." : "I report that this physical device was working correctly during this capture."}</span></label>
            <button className="primary" disabled={!confirmed || saving || alreadySaved || !selected} onClick={save}>{saving ? <RefreshCw size={15} /> : <Save size={15} />}{alreadySaved ? "Capture already saved" : "Save as Known Good"}</button>
          </> : <div className="inline-empty"><Activity size={19} /><p>No stored serial or simulator capture yet. Start a capture, then refresh this passport.</p><button className="secondary" onClick={() => onNavigate("live")}>Open live signals <ArrowUpRight size={14} /></button></div>}
        </div>
      </section>

      <CircuitMap profile={passport.profile} plan={passport.probe_plan ?? plan} session={session} demoMode={!project} onNavigate={onNavigate} />

      <section className="panel passport-story" aria-labelledby="passport-story-title"><div className="panel-heading"><div><span className="eyebrow">Persisted evidence · newest first</span><h2 id="passport-story-title">The device’s story</h2></div><History size={17} /></div>
        <p>{supportedFindings} evidence-supported {supportedFindings === 1 ? "finding" : "findings"} in the most recent {story.length} detailed {story.length === 1 ? "session" : "sessions"} · {passport.verified_repairs.length} {passport.verified_repairs.length === 1 ? "outcome" : "outcomes"} verified as recovered after a user-reported action.</p>
        {events.length ? <ol className="passport-story-list">{events.map((event) => <li key={event.id}><time dateTime={new Date(event.timestamp_ms).toISOString()}>{dateLabel(event.timestamp_ms)}</time><div><strong>{event.kind.replaceAll("_", " ")}</strong><span>{event.kind === "USER_ACTION" ? `User reported: ${event.description}` : event.description}</span><small>{event.source} · {event.provenance}</small></div></li>)}</ol> : <p>No persisted diagnostic events yet. Run a guided test to begin the record.</p>}
      </section>

      <section className="passport-grid passport-records">
        <div className="panel"><div className="panel-heading"><div><span className="eyebrow">Build</span><h2>Components</h2></div><Cpu size={17} /></div>{passport.profile.components.length ? passport.profile.components.map((component) => <div className="passport-list-row" key={component.id}><strong>{component.name}</strong><small>{component.interface_type ?? "Interface not specified"}</small></div>) : <p className="passport-empty">No components in the confirmed profile.</p>}</div>
        <div className="panel"><div className="panel-heading"><div><span className="eyebrow">Diagnostic record</span><h2>History & verified outcomes</h2></div><History size={17} /></div>{passport.history.length ? passport.history.map((item) => <div className="passport-list-row" key={item.id}><strong>{item.status.replaceAll("_", " ")} · {dateLabel(item.started_at_ms)}</strong><small>{item.original_problem}</small></div>) : <p className="passport-empty">No guided diagnostic sessions saved yet.</p>}
          <h3>Verified after user-reported action</h3>{passport.verified_repairs.length ? passport.verified_repairs.map((repair) => <div className="passport-list-row" key={repair.workflow_id}><strong>{dateLabel(repair.verified_at_ms)} · {repair.source === "PHYSICAL" ? "Physical serial comparison" : "Simulated comparison"}</strong><small>{repair.summary}</small><small>User reported: {repair.user_reported_actions.join("; ")}</small></div>) : <p className="passport-empty">No resolved VERIFY outcome following a user-reported action.</p>}
          <button className="text-button" onClick={() => onNavigate("history")}>Open full diagnostic history <ArrowUpRight size={14} /></button>
        </div>
      </section>
      <div className="security-note"><CheckCircle2 size={18} /><div><strong>No invented health score</strong><span>Physical and simulated baselines stay separate. “Healthy” means a later physical capture matched a user-reported physical baseline within the stated tolerances, not a certification.</span></div></div>
    </>}
  </div>;
}
