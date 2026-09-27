"use client";

import { useEffect, useRef, useState } from "react";
import { ArrowRight, X, Zap } from "lucide-react";
import type { SimulatorScenario } from "@reweird/shared-types";
import { availableWeirdChoices, mysteryScenario } from "@/lib/weird-demo";

export function MakeItWeird({ scenarios, busy, onRun, onBrowserDemo }: {
  scenarios: SimulatorScenario[];
  busy: boolean;
  onRun: (scenarioID: string, mystery: boolean) => Promise<void>;
  onBrowserDemo: () => Promise<void>;
}) {
  const [open, setOpen] = useState(false);
  const [choice, setChoice] = useState<string | null>(null);
  const dialog = useRef<HTMLDialogElement>(null);
  const opener = useRef<HTMLButtonElement>(null);
  const choices = availableWeirdChoices(scenarios);

  useEffect(() => {
    const node = dialog.current;
    if (open && node && !node.open) node.showModal();
    if (!open && node?.open) node.close();
  }, [open]);

  function close() {
    setOpen(false);
    opener.current?.focus();
  }

  async function run() {
    if (choices.length === 0 && choice === "intermittent-connection") {
      close();
      await onBrowserDemo();
      return;
    }
    const mystery = choice === "mystery";
    const scenario = mystery ? mysteryScenario(scenarios) : choice;
    if (!scenario) return;
    close();
    await onRun(scenario, mystery);
  }

  return <>
    <section className="weird-invite" aria-labelledby="weird-invite-title">
      <div><span className="bench-label">Judge demo · simulated fault</span><h2 id="weird-invite-title">When hardware gets weird, ReWeird it.</h2><p>Choose a fault and follow it from raw samples to evidence, guided test, and VERIFY.</p></div>
      <button ref={opener} className="primary weird-open" onClick={() => { setChoice(null); setOpen(true); }}><Zap size={17} /> MAKE IT WEIRD <ArrowRight size={16} /></button>
      <small>Demo simulation — no physical output is generated.</small>
    </section>
    <dialog ref={dialog} className="weird-dialog" aria-labelledby="weird-dialog-title" aria-describedby="weird-dialog-description" onClose={() => { setOpen(false); opener.current?.focus(); }} onCancel={() => setOpen(false)}>
      <div className="weird-dialog-head"><div><span className="bench-label">Demo simulation · no physical output</span><h2 id="weird-dialog-title">MAKE IT WEIRD.</h2></div><button className="icon-button" aria-label="Close fault selector" onClick={close}><X size={18} /></button></div>
      <p id="weird-dialog-description">Pick a simulated fault. ReWeird will show what changed and why, without controlling hardware.</p>
      <fieldset className="weird-choices"><legend>Choose a simulated fault</legend>
        {choices.length === 0 && <label className={choice === "intermittent-connection" ? "chosen" : ""}><input type="radio" name="weird-choice" checked={choice === "intermittent-connection"} onChange={() => setChoice("intermittent-connection")} /><span>Loose connection<small>Illustrative browser demo</small></span></label>}
        {choices.map((item) => <label key={item.scenario} className={choice === item.scenario ? "chosen" : ""}><input type="radio" name="weird-choice" checked={choice === item.scenario} onChange={() => setChoice(item.scenario)} /><span>{item.label}</span></label>)}
        {choices.length > 0 && <label className={choice === "mystery" ? "chosen" : ""}><input type="radio" name="weird-choice" checked={choice === "mystery"} onChange={() => setChoice("mystery")} /><span>Mystery fault<small>Revealed only after analysis</small></span></label>}
      </fieldset>
      {choices.length === 0 && <p role="status">The Go API is not running. Loose connection works as an illustrative browser demo; other faults need the API.</p>}
      <div className="weird-dialog-actions"><button className="secondary" onClick={close}>Cancel</button><button className="primary" disabled={!choice || busy} onClick={run}>MAKE IT WEIRD <ArrowRight size={15} /></button></div>
    </dialog>
  </>;
}
