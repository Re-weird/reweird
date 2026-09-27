"use client";

import { useEffect, useState } from "react";
import { Activity, ArrowUpRight, RotateCcw, ShieldCheck } from "lucide-react";
import type { DemoSession } from "@reweird/shared-types";

type Capture = { id: number; failed: boolean; probe: string; role: string; failures: string[] };

function capture(session: DemoSession): Capture | null {
  if (!session.measurement_id || !session.raw_telemetry) return null;
  const failures = session.evidence.rule_results.filter((rule) => rule.status === "fail");
  return {
    id: session.measurement_id,
    failed: failures.length > 0,
    probe: session.evidence.probe,
    role: session.evidence.role,
    failures: failures.map((rule) => rule.message),
  };
}

export function JudgeCircuit({ session, onDiagnose, onTest, onVerify }: {
  session: DemoSession;
  onDiagnose: () => void;
  onTest: () => void;
  onVerify: () => void;
}) {
  const [baseline, setBaseline] = useState<Capture | null>(null);
  const [fault, setFault] = useState<Capture | null>(null);
  const [recovered, setRecovered] = useState<Capture | null>(null);
  const current = capture(session);

  // Keep the judge's physical experiment tied to one actual serial source/profile.
  // A project switch unmounts this card; a source switch resets its local steps.
  useEffect(() => {
    if (session.telemetry_mode !== "serial" || !session.hardware_connected) {
      setBaseline(null); setFault(null); setRecovered(null);
    }
  }, [session.telemetry_mode, session.hardware_connected]);

  useEffect(() => {
    if (!baseline || !current || current.id <= baseline.id) return;
    if (!fault && current.failed && /echo/i.test(current.role)) setFault(current);
    else if (fault && !recovered && current.id > fault.id && !current.failed) setRecovered(current);
  }, [baseline, fault, recovered, current]);

  const phase = recovered ? 3 : fault ? 2 : baseline ? 1 : 0;
  return <section className="judge-circuit" aria-labelledby="judge-circuit-title">
    <div className="judge-circuit-top"><span className="bench-label">Judge challenge · live serial measurements</span><span className="judge-circuit-count">{phase + 1} / 4</span></div>
    <h2 id="judge-circuit-title">BREAK OUR CIRCUIT.</h2>
    <p>Change the designated Echo connection on the prepared low voltage HC-SR04 demo. ReWeird watches the probe inputs; you make the physical change.</p>
    <ol className="judge-circuit-steps">
      <li className={phase === 0 ? "current" : ""}><strong>1 · Capture working</strong><span>{baseline ? `Capture #${baseline.id} recorded as the comparison point` : "Wait for a healthy serial capture, then begin."}</span></li>
      <li className={phase === 1 ? "current" : ""}><strong>2 · Break Echo</strong><span>{fault ? `Failed checks in capture #${fault.id}` : "Disconnect only the designated Echo path. Await a new measured window."}</span></li>
      <li className={phase === 2 ? "current" : ""}><strong>3 · Inspect and test</strong><span>{fault ? `${fault.probe} · ${fault.role}: ${fault.failures.join(" ")}` : "PROBE will show the evidence and a supported next test."}</span></li>
      <li className={phase === 3 ? "current" : ""}><strong>4 · Restore and verify</strong><span>{recovered ? `Capture #${recovered.id} has no failed checks; run VERIFY to confirm recovery.` : "Reconnect Echo and wait for another measured window."}</span></li>
    </ol>
    <div className="judge-circuit-actions">
      {phase === 0 && <button className="primary" disabled={!current || current.failed} onClick={() => setBaseline(current)}><Activity size={15} /> Start from healthy capture</button>}
      {phase === 1 && <span role="status">Watching for a new capture with failed checks…</span>}
      {phase >= 2 && <button className="primary" onClick={onDiagnose}>Inspect measured evidence <ArrowUpRight size={15} /></button>}
      {phase >= 2 && <button className="secondary" onClick={onTest}>Open guided test <ArrowUpRight size={15} /></button>}
      {phase === 3 && <button className="secondary" onClick={onVerify}>Open VERIFY <ArrowUpRight size={15} /></button>}
      {phase > 0 && <button className="text-button" onClick={() => { setBaseline(null); setFault(null); setRecovered(null); }}><RotateCcw size={14} /> Restart challenge</button>}
    </div>
    <small><ShieldCheck size={13} /> Readings come from serial telemetry. A clear window here is a candidate recovery; only the guided VERIFY workflow can confirm it. PATCH output stays locked.</small>
  </section>;
}
