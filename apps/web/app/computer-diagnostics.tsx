"use client";

import { useEffect, useState } from "react";
import { AlertCircle, Cpu, Play, RefreshCw, ShieldCheck } from "lucide-react";
import { computerApi, type ComputerAnalysis } from "@/lib/api";

export function ComputerDiagnosticsView() {
  const [scenarios, setScenarios] = useState<Array<{ id: string; name: string; description: string }>>([]);
  const [selected, setSelected] = useState("PORT_CONFLICT");
  const [analysis, setAnalysis] = useState<ComputerAnalysis | null>(null);
  const [status, setStatus] = useState<{ real_collection_enabled: boolean; collector: string } | null>(null);
  const [error, setError] = useState("");
  const [busy, setBusy] = useState(false);
  const [exclusivePort, setExclusivePort] = useState("");
  useEffect(() => {
    computerApi.scenarios().then((result) => setScenarios(result.scenarios)).catch((cause) => setError(cause.message));
    computerApi.status().then(setStatus).catch((cause) => setError(cause.message));
  }, []);
  async function run(simulated: boolean) {
    setBusy(true); setError("");
    try { setAnalysis(simulated ? await computerApi.simulate(selected) : await computerApi.collect({ exclusive_port: exclusivePort ? Number(exclusivePort) : undefined })); }
    catch (cause) { setError(cause instanceof Error ? cause.message : "Computer diagnostics failed."); }
    finally { setBusy(false); }
  }
  return <section className="computer-page">
    <div className="page-heading"><div><span className="eyebrow">OPTIONAL · READ-ONLY</span><h1>Computer diagnostics</h1><p>System evidence is separate from probe measurements. Nothing is changed, stopped, or deleted.</p></div><Cpu size={34} /></div>
    <div className="computer-columns">
      <div className="panel computer-controls"><h2>Simulated faults</h2><p>Exercise the evidence pipeline without collecting data from this computer.</p>
        <label>Scenario<select value={selected} onChange={(event) => setSelected(event.target.value)}>{scenarios.map((scenario) => <option key={scenario.id} value={scenario.id}>{scenario.name}</option>)}</select></label>
        <p>{scenarios.find((scenario) => scenario.id === selected)?.description}</p>
        <button className="primary" disabled={busy || scenarios.length === 0} onClick={() => run(true)}><Play size={14} /> Run simulation</button>
        <hr /><h2>Local system snapshot</h2><p>{status?.real_collection_enabled ? `Opt-in collector: ${status.collector}. A snapshot is only collected when you click below.` : "Disabled by default. Set COMPUTER_DIAGNOSTICS_ENABLED=true on a trusted local backend to opt in."}</p>
        {status?.real_collection_enabled && <><label>Exclusive TCP port (optional)<input type="number" min="1" max="65535" value={exclusivePort} onChange={(event) => setExclusivePort(event.target.value)} placeholder="e.g. 3000" /></label><button className="secondary" disabled={busy} onClick={() => run(false)}><RefreshCw size={14} /> Collect read-only snapshot</button></>}
      </div>
      <div className="panel computer-result"><h2><ShieldCheck size={17} /> Structured evidence</h2>{error && <p className="computer-error"><AlertCircle size={15} /> {error}</p>}{!analysis && <p>Choose a simulated fault to see measured system facts and deterministic findings.</p>}
        {analysis && <><div className="report-meta"><span>Source: {analysis.snapshot.source}</span><span>{analysis.snapshot.system.os}</span><span>{new Date(analysis.snapshot.timestamp_ms).toLocaleString()}</span></div><h3>Findings</h3>{analysis.findings.length === 0 ? <p>No configured rule is violated in this snapshot.</p> : analysis.findings.map((finding) => <article className="computer-finding" key={finding.code}><strong>{finding.code.replaceAll("_", " ")}</strong><p>{finding.summary}</p><small>{finding.next_action}</small></article>)}<h3>Observed facts</h3><div className="report-facts">{analysis.evidence.computer_evidence.map((entry, index) => { const fact = entry as { name: string; value: unknown; unit?: string; provenance: string }; return <div key={index}><span>{fact.name.replaceAll("_", " ")}</span><strong>{String(fact.value)} {fact.unit ?? ""}</strong><small>{fact.provenance}</small></div>; })}</div><p>Physical evidence: {analysis.evidence.physical_evidence.length} · Software evidence: {analysis.evidence.software_evidence.length}</p></>}
      </div>
    </div>
  </section>;
}
