"use client";

import { useEffect, useState } from "react";
import { RefreshCw, ShieldCheck } from "lucide-react";
import { systemApi, type SystemStatus } from "@/lib/api";

export function SettingsStatusView() {
  const [status,setStatus]=useState<SystemStatus|null>(null);
  const [error,setError]=useState("");
  const [busy,setBusy]=useState(false);
  async function refresh() {
    setBusy(true);
    try { setStatus(await systemApi.status()); setError(""); }
    catch (cause) { setStatus(null); setError(cause instanceof Error ? cause.message : "Backend unavailable."); }
    finally { setBusy(false); }
  }
  useEffect(() => { void refresh(); }, []);
  const fields: Array<[string,string]> = status ? [
    ["Backend",status.api],["Database",status.database],["Telemetry",`${status.telemetry_mode} · ${status.telemetry}`],["ESP32",status.esp32 ? "Serial telemetry available" : "Not connected"],["Gemini Vision",status.gemini.replaceAll("_"," ")],["PATCH",status.patch],["Git sync",status.git_sync_enabled ? "Enabled" : "Disabled"],["Git repository",status.git_repository_configured ? "Configured" : "Not configured"],["Computer agent",status.computer_agent.replaceAll("_"," ")],["Report retention",status.report_retention.replaceAll("_"," ")],["Raw measurement limit",`${status.measurement_window_limit.toLocaleString()} windows`],
  ] : [];
  return <section className="settings-status"><div className="page-heading"><div><span className="eyebrow">LOCAL INTEGRATIONS</span><h1>Settings &amp; status</h1><p>Read-only status. Configure services on the backend; secret values are never shown here.</p></div><button className="secondary" disabled={busy} onClick={refresh}><RefreshCw size={15} /> Refresh</button></div>
    {error && <div className="form-error page-error">{error}. The browser demo remains available, but persisted diagnostics require the API.</div>}
    <div className="panel status-list">{fields.length ? fields.map(([label,value]) => <div key={label}><span>{label}</span><strong>{value}</strong></div>) : <p>{busy ? "Checking backend…" : "No backend status available."}</p>}</div>
    <div className="security-note"><ShieldCheck size={18} /><div><strong>Safety boundaries</strong><span>PATCH is locked. Real computer collection and Git sync are off by default. Gemini is optional; its key remains server-side.</span></div></div>
  </section>;
}
