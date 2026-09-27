"use client";

import { useEffect, useState } from "react";

type Reading = {probe: string; role: string; value?: number | null; unit?: string; status?: string};
type View = {connected: boolean; device_id: string; profile_id: string; last_capture_at_ms: number; sequence?: number; error?: string; probes?: Reading[]};

export default function LiveJudgePage() {
  const [view, setView] = useState<View | null>(null);
  const [error, setError] = useState("");
  useEffect(() => {
    const id = window.location.pathname.split("/").filter(Boolean).at(-1) ?? "";
    // Fragment never reaches server/access logs or referrers. Memory only.
    const token = window.location.hash.slice(1);
    let stopped = false;
    let active: AbortController | null = null;
    async function refresh() {
      if (active) return;
      if (token.length !== 64) { setError("Open the full judge link supplied by the project owner."); return; }
      const abort = new AbortController();
      active = abort;
      const timeout = setTimeout(() => abort.abort(), 5000);
      try {
        const res = await fetch(`/api/v1/bridge/${encodeURIComponent(id)}/view`, {headers: {"X-ReWeird-Share": token}, cache: "no-store", signal: abort.signal});
        const payload = await res.json();
        if (!res.ok) throw new Error(payload.detail ?? "Judge link expired or revoked.");
        if (!stopped) {setView(payload); setError("");}
      } catch (e) { if (!stopped) {setView(null); setError(e instanceof Error ? e.message : "Connection unavailable");} }
      finally { clearTimeout(timeout); active = null; }
    }
    void refresh();
    const interval = setInterval(() => void refresh(), 3000);
    return () => {stopped = true; active?.abort(); clearInterval(interval);};
  }, []);
  return <main style={{maxWidth: 1080, margin: "40px auto", padding: 24}}>
    <p className="eyebrow">ReWeird · read-only live demo</p>
    <h1>{view?.connected && !error ? "Live circuit measurements" : "Device offline / waiting for hardware"}</h1>
    <p>REAL SERIAL · USB → laptop → cloud · PATCH LOCKED</p>
    {error && <p role="alert">{error}</p>}
    {view && <><p>Device: {view.device_id} · Capture #{view.sequence ?? "—"}</p><p>Last capture: {view.last_capture_at_ms ? new Date(view.last_capture_at_ms).toLocaleString() : "None received"}</p>{view.error && <p role="status">{view.error}</p>}</>}
    {view?.connected && !error && <div style={{display: "grid", gridTemplateColumns: "repeat(auto-fit,minmax(230px,1fr))", gap: 16}}>{view.probes?.map((p, index) => <section className="panel" key={`${p.probe}-${index}`} style={{padding: 24}}><h2>{p.probe} {p.role}</h2><p style={{fontSize: 28}}>{typeof p.value === "number" ? p.value.toLocaleString(undefined, {maximumFractionDigits: 3}) : "—"} {p.unit}</p><p>{p.status}</p></section>)}</div>}
    <p>This link cannot change the profile, approve tests, drive outputs, or access uploaded code/photos. No simulated readings are substituted when hardware goes offline.</p>
  </main>;
}
