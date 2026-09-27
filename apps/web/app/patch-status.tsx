"use client";

import { useEffect, useState } from "react";
import { patchApi, type PatchAction, type PatchStatusResponse } from "@/lib/api";
import { useAppState } from "@/lib/app-state";

export function PatchStatus() {
  const { currentProjectID } = useAppState();
  const [status, setStatus] = useState<PatchStatusResponse | null>(null);
  const [actions, setActions] = useState<PatchAction[]>([]);
  const [pending, setPending] = useState<PatchAction | null>(null);
  const [busy, setBusy] = useState(false);
  const [error, setError] = useState("");
  const [confirm, setConfirm] = useState(false);
  const [level, setLevel] = useState("HIGH");
  const [duration, setDuration] = useState(10);
  useEffect(() => {
    let alive = true;
    const refresh = async () => {
      try {
        const next = await patchApi.status();
        if (alive) setStatus(next);
        const history = await patchApi.history(currentProjectID);
        if (alive) setActions(history);
      } catch { if (alive) setStatus(null); }
    };
    void refresh();
    const timer = setInterval(() => void refresh(), busy ? 150 : 1000);
    return () => { alive = false; clearInterval(timer); };
  }, [currentProjectID, busy]);
  useEffect(() => { setPending(null); setConfirm(false); }, [currentProjectID]);
  const qualified = status?.capability?.profile_id === currentProjectID && status.state !== "PATCH LOCKED";
  const run = async (task: () => Promise<unknown>) => {
    setBusy(true); setError("");
    try { await task(); } catch (e) { setError(e instanceof Error ? e.message : "PATCH failed; no execution is claimed."); }
    finally { setBusy(false); }
  };
  return <section className="panel guided-panel" aria-label="PATCH hardware safety">
    <p className="kicker">{status?.state ?? "PATCH LOCKED"} · Master enable {status?.master_enabled ? "ON" : "OFF"}</p>
    <h2>{status?.software_ready ? "PATCH SOFTWARE READY" : "PATCH CONTROLLER UNAVAILABLE"}</h2>
    <p>{status?.detail ?? "Controller unavailable; physical execution is not available."}</p>
    <p>P1–P6 remain measurement inputs. Physical output requires a verified dedicated protected interface.</p>
    {qualified && <>
      <button className="secondary" disabled={busy || !!pending} onClick={() => void run(() => patchApi.master(currentProjectID, !status?.master_enabled))}>{status?.master_enabled ? "Disable master output" : "Enable master for this verified hardware session"}</button>
      {!pending && <div>
        <label>Logic level <select value={level} onChange={e => setLevel(e.target.value)}><option>HIGH</option><option>LOW</option></select></label>
        <label>Duration (1–250 ms) <input type="number" min={1} max={250} value={duration} onChange={e => setDuration(Number(e.target.value))} /></label>
        <button className="secondary" disabled={busy || !status?.master_enabled} onClick={() => void run(async () => { setPending(await patchApi.prepare(currentProjectID, level, duration)); setConfirm(false); })}>Propose bounded test</button>
      </div>}
    </>}
    {pending && <div role="group" aria-label="Active hardware test approval">
      <h3>ACTIVE HARDWARE TEST</h3>
      <p>Target: {pending.parameters.target_node} · Dedicated PATCH GPIO {pending.parameters.patch_pin}</p>
      <p>{pending.parameters.mode} · {pending.parameters.logic_level} · {pending.parameters.max_voltage} V maximum · {pending.parameters.duration_ms} ms</p>
      <p>Device {pending.parameters.device_id} · Profile revision {pending.parameters.profile_revision}</p>
      <p>ReWeird will temporarily drive this node. Only connect a qualified, isolated input.</p>
      <label><input type="checkbox" checked={confirm} disabled={busy} onChange={e => setConfirm(e.target.checked)} /> I approve exactly this action and have verified target isolation.</label>
      <button className="secondary" onClick={() => void run(async () => { await patchApi.cancel(currentProjectID, pending); setPending(null); setConfirm(false); })}>Cancel / disable output</button>
      <button className="primary" disabled={!confirm || busy || !qualified || !status?.master_enabled || Date.now() >= pending.parameters.expires_at_ms} onClick={() => void run(async () => { await patchApi.approve(currentProjectID, pending); setPending(null); setConfirm(false); })}>Approve test</button>
    </div>}
    {error && <p role="alert">{error}</p>}
    <details><summary>PATCH audit / results ({actions.length})</summary>{actions.map(a => <article key={a.id}>
      <h3>{a.parameters.source} · {a.state}</h3><p>{a.parameters.target_node} · {a.parameters.duration_ms} ms · {a.result ?? "No verified result"}</p>
      {a.state === "AWAITING_APPROVAL" && a.parameters.source === "REAL_SERIAL" && !busy && !pending && <button className="secondary" onClick={() => { setPending(a); setConfirm(false); }}>Review proposed test</button>}
      {a.before && <p>Before #{a.before.measurement_id} / After {a.after ? `#${a.after.measurement_id}` : "pending"} · {a.before.source}</p>}
      <ol>{a.events.map((e, i) => <li key={`${a.id}-${i}`}>{e.state}: {e.detail}</li>)}</ol>
    </article>)}</details>
  </section>;
}
