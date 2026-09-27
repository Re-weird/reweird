"use client";

import { useCallback, useEffect, useState } from "react";
import { CircleAlert, Gauge, RefreshCw, Save, ShieldCheck } from "lucide-react";
import type { CalibrationState } from "@reweird/shared-types";
import { ApiError, passportApi } from "@/lib/api";
import { calibrationStatusLabel, calibrationSteps, canSaveKnownGood, expectedText, knownGoodText, observedText, probeTone } from "@/lib/calibration";

/**
 * Physical commissioning: observe REAL SERIAL windows, show EXPECTED /
 * OBSERVED / KNOWN GOOD side by side, and save a Known Good only after the
 * user explicitly confirms the circuit is operating correctly right now.
 */
export function CalibrationPanel({ profileID, compact = false, onSaved }: { profileID: string; compact?: boolean; onSaved?: () => void }) {
  const [state, setState] = useState<CalibrationState | null>(null);
  const [error, setError] = useState("");
  const [confirmed, setConfirmed] = useState(false);
  const [note, setNote] = useState("");
  const [saving, setSaving] = useState(false);

  const refresh = useCallback(async () => {
    try {
      setState(await passportApi.calibration(profileID));
      setError("");
    } catch (cause) {
      setError(cause instanceof Error ? cause.message : "Calibration state could not be loaded.");
    }
  }, [profileID]);

  useEffect(() => {
    void refresh();
    // Physical windows arrive about once per second; follow the observation.
    const timer = window.setInterval(() => { void refresh(); }, 3000);
    return () => window.clearInterval(timer);
  }, [refresh]);

  // A new candidate window means new evidence; the user must confirm again.
  const candidate = state?.candidate_measurement_id;
  const reviewable = state?.status === "REVIEW_REQUIRED";
  useEffect(() => { if (!reviewable) setConfirmed(false); }, [reviewable]);

  async function save() {
    if (!state || !canSaveKnownGood(state, confirmed) || !candidate) return;
    setSaving(true);
    setError("");
    try {
      await passportApi.saveKnownGood(profileID, candidate, note);
      setConfirmed(false);
      setNote("");
      await refresh();
      onSaved?.();
    } catch (cause) {
      setError(cause instanceof ApiError || cause instanceof Error ? cause.message : "Known Good could not be saved.");
    } finally {
      setSaving(false);
    }
  }

  if (!state) {
    return <section className="panel calibration-panel"><div className="calibration-head"><div><span className="eyebrow">Physical calibration</span><h2>Loading…</h2></div></div>{error && <p className="calibration-error" role="alert"><CircleAlert size={14} />{error}</p>}</section>;
  }

  const steps = calibrationSteps(state.status);
  const shownProbes = compact ? state.probes.filter((probe) => probe.role.toUpperCase() !== "UNASSIGNED" || probe.issues?.length) : state.probes;

  return <section className={`panel calibration-panel status-${state.status.toLowerCase()}`} aria-labelledby={`calibration-${profileID}`}>
    <div className="calibration-head">
      <div><span className="eyebrow">Physical calibration · {state.provenance === "REAL_SERIAL" ? "REAL SERIAL" : "no physical capture"}</span><h2 id={`calibration-${profileID}`}>{calibrationStatusLabel[state.status]}</h2><p>{state.detail}</p></div>
      <button className="secondary" onClick={() => void refresh()} aria-label="Refresh calibration"><RefreshCw size={14} /></button>
    </div>

    <ol className="calibration-steps" aria-label="Calibration progress">{steps.map((step, index) => <li key={step.id} className={step.state}><span>{String(index + 1).padStart(2, "0")}</span>{step.label}</li>)}</ol>

    <div className="calibration-meta">
      <span>Device {state.device_id ?? "—"}</span>
      <span>Profile {state.profile_id} · revision {state.profile_version}</span>
      <span>{state.windows_observed}/{state.windows_required} consecutive windows</span>
      {state.known_good && <span>Known Good #{state.known_good.first_measurement_id}–#{state.known_good.last_measurement_id} · {state.known_good.provenance ?? state.known_good.source} · rev {state.known_good.profile_version}</span>}
    </div>

    <div className="calibration-table" role="table" aria-label="Expected, observed and Known Good per probe">
      <div className="calibration-row calibration-row-head" role="row"><span role="columnheader">Probe</span><span role="columnheader">Expected <small>configured</small></span><span role="columnheader">Observed <small>real serial</small></span><span role="columnheader">Known Good <small>confirmed</small></span></div>
      {shownProbes.map((probe) => <div className={`calibration-row ${probeTone(probe)}`} role="row" key={probe.probe}>
        <span role="cell"><b>{probe.probe}</b><small>{probe.role} · {probe.mode}</small></span>
        <span role="cell" data-label="Expected">{expectedText(probe.role, probe.expected)}</span>
        <span role="cell" data-label="Observed">{observedText(probe.observed)}</span>
        <span role="cell" data-label="Known Good">{knownGoodText(probe.known_good)}</span>
        {probe.issues?.length ? <ul className="calibration-issues" role="cell">{probe.issues.map((issue) => <li key={issue}>{issue}</li>)}</ul> : null}
      </div>)}
    </div>

    {state.blockers?.length ? <div className="calibration-blockers" role="status"><CircleAlert size={15} /><div><strong>Cannot be saved as Known Good yet</strong>{state.blockers.map((blocker) => <p key={blocker}>{blocker}</p>)}</div></div> : null}

    {state.status === "REVIEW_REQUIRED" && <div className="calibration-confirm">
      <p><Gauge size={14} /> Review the expected and observed columns. ReWeird only records what it measured; it cannot know whether your circuit is doing the right thing.</p>
      <label className="passport-confirm"><input type="checkbox" checked={confirmed} onChange={(event) => setConfirmed(event.target.checked)} /><span>I have physically checked this circuit and it is operating correctly right now. Save captures #{state.first_measurement_id}–#{state.candidate_measurement_id} as its Known Good.</span></label>
      <label className="calibration-note">Note (optional)<input value={note} maxLength={500} onChange={(event) => setNote(event.target.value)} placeholder="What did you verify? e.g. rail 5.0 V on a meter, distance reads correctly" /></label>
      <button className="primary" disabled={!canSaveKnownGood(state, confirmed) || saving} onClick={save}>{saving ? <RefreshCw className="spin" size={15} /> : <Save size={15} />} Save as Known Good</button>
    </div>}

    {state.status === "CALIBRATED" && <p className="calibration-ok"><ShieldCheck size={15} /> New REAL SERIAL captures are compared against both the configured expectations and this Known Good.</p>}
    {error && <p className="calibration-error" role="alert"><CircleAlert size={14} />{error}</p>}
  </section>;
}
