"use client";
import { useEffect, useState } from "react";
import { patchPracticeNext } from "@/lib/patch-simulation";

export function SimulatedPatch() {
  const [states, setStates] = useState<string[]>([]);
  const [approved, setApproved] = useState(false);
  const state = states.at(-1) ?? "";
  useEffect(() => {
    const next = patchPracticeNext(state, approved);
    if (next === state) return;
    const timer = setTimeout(() => setStates(previous => [...previous, next]), state === "ACTIVE" ? 100 : 200);
    return () => clearTimeout(timer);
  }, [state, approved]);
  return <section className="panel guided-panel" aria-label="Simulated PATCH">
    <p className="kicker">SIMULATED PATCH · NO ELECTRICAL OUTPUT</p>
    <h2>Practice the active-test approval flow</h2>
    <p>Virtual isolated input · 3.3 V HIGH · 100 ms. This lifecycle rehearsal does not capture or verify physical measurements.</p>
    <button className="secondary" disabled={!!state && state !== "VERIFY" && state !== "CANCELLED"} onClick={() => { setApproved(false); setStates(["PROPOSED"]); }}>Propose simulated test</button>
    {state === "AWAITING_APPROVAL" && <button className="primary" onClick={() => setApproved(true)}>Approve SIMULATED test</button>}
    {!!state && state !== "VERIFY" && state !== "CANCELLED" && <button className="secondary" onClick={() => { setApproved(false); setStates(previous => [...previous, "CANCELLED"]); }}>Cancel simulation</button>}
    <p aria-live="polite">{states.map(s => `SIMULATED ${s}`).join(" → ")}</p>
    {state === "VERIFY" && <p>SIMULATED VERIFY: INCONCLUSIVE — no measurement evidence was supplied by this lifecycle rehearsal. No physical repair is claimed.</p>}
  </section>;
}
