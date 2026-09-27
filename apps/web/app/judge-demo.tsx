"use client";

import { useEffect, useReducer, useRef, useState } from "react";
import Link from "next/link";
import {
  ArrowLeft, ArrowRight, Check, CircleDot, CircleHelp, Cpu, Download, FlaskConical, Fingerprint, Lightbulb,
  LockKeyhole, Moon, RotateCcw, ShieldCheck, Sun, TriangleAlert, Wrench, Zap,
} from "lucide-react";
import {
  FAULTS, FAULT_ORDER, FIXES, FIX_ORDER, TESTS, TEST_ORDER,
  canGoBack, checks, current, failing, found, hint, incidentRecord, initialState, latestCapture, reduce, resolved, revealed, testHeadline, verifyHeadline,
  type Check as EvidenceCheck, type DemoState, type FaultID, type FixID, type Snapshot, type TestID, type TestRun,
} from "@/lib/judge-demo";

// Simulated capture delay: long enough to feel like a measurement, short enough that nobody waits.
const CAPTURE_MS = 900;

const JOURNEY = ["Start", "Make it weird", "Investigate", "Fix", "VERIFY", "Passport"] as const;
const STAGE_STEP: Record<DemoState["stage"], number> = { intro: 0, choose: 1, investigate: 2, fix: 3, verify: 4, passport: 5 };

type Theme = "light" | "dark";

function useTheme() {
  const [theme, setTheme] = useState<Theme>("dark");
  useEffect(() => {
    let stored: string | null = null;
    try { stored = window.localStorage.getItem("reweird-theme"); } catch { /* storage unavailable */ }
    const next: Theme = stored === "light" || stored === "dark" ? stored : window.matchMedia("(prefers-color-scheme: light)").matches ? "light" : "dark";
    setTheme(next);
    document.documentElement.dataset.theme = next;
  }, []);
  const toggle = () => {
    const next: Theme = theme === "dark" ? "light" : "dark";
    setTheme(next);
    document.documentElement.dataset.theme = next;
    try { window.localStorage.setItem("reweird-theme", next); } catch { /* keep in-session */ }
  };
  return { theme, toggle };
}

// ---------------------------------------------------------------------------
// Evidence pieces
// ---------------------------------------------------------------------------

function Trace({ values, danger }: { values: number[]; danger: boolean }) {
  const max = Math.max(...values, 1);
  const points = values.map((value, index) => `${(index / (values.length - 1)) * 120},${35 - (value / max) * 29}`).join(" ");
  return <svg className="mini-chart" viewBox="0 0 120 38" preserveAspectRatio="none" aria-hidden="true">
    <polyline className={danger ? "danger-line" : "signal-line"} points={points} fill="none" />
  </svg>;
}

function StatusPill({ status }: { status: EvidenceCheck["status"] }) {
  const Icon = status === "pass" ? Check : status === "warn" ? CircleDot : TriangleAlert;
  return <span className={`jd-pill ${status}`}><Icon size={11} />{status === "pass" ? "OK" : status === "warn" ? "Odd" : "Fail"}</span>;
}

function ProbeTiles({ snapshot, capturing = false }: { snapshot: Snapshot; capturing?: boolean }) {
  const results = checks(snapshot);
  const worst = (probe: string) => {
    const mine = results.filter((c) => c.probe === probe);
    return mine.find((c) => c.status === "fail") ?? mine.find((c) => c.status === "warn") ?? mine[0];
  };
  const tiles = [
    { probe: "P1", role: "POWER", what: "5 V supply", value: snapshot.power.min_v.toFixed(2), unit: "V lowest", samples: snapshot.power.samples },
    { probe: "P2", role: "TRIG", what: "Fires the sensor", value: String(snapshot.trig.width_us), unit: "µs pulse", samples: snapshot.trig.samples },
    { probe: "P3", role: "ECHO", what: "Sensor’s reply", value: snapshot.echo.rate_hz.toFixed(1), unit: "replies/s", samples: snapshot.echo.samples },
  ];
  return <div className={`jd-probes ${capturing ? "capturing" : ""}`} aria-busy={capturing}>
    {tiles.map((tile) => {
      const check = worst(tile.probe);
      return <article key={tile.probe} className={`probe-card jd-probe ${check.status}`}>
        <div className="jd-probe-top"><h3><span className="jd-probe-tag">{tile.probe}</span>{tile.role}</h3><StatusPill status={check.status} /></div>
        <small className="jd-probe-what">{tile.what}</small>
        <div className="probe-reading"><strong>{tile.value}</strong><span>{tile.unit}</span></div>
        <Trace values={tile.samples} danger={check.status === "fail"} />
        <p className="jd-probe-check">{check.message}</p>
      </article>;
    })}
  </div>;
}

function Sim() {
  return <span className="jd-sim-badge">SIMULATED</span>;
}

function testMetric(run: TestRun): { label: string; before: string; during: string } {
  switch (run.test) {
    case "wiggle": return { label: "ECHO dropouts / min", before: String(run.before.echo.dropouts_per_min), during: String(run.during.echo.dropouts_per_min) };
    case "rail-load": return { label: "Lowest rail voltage", before: `${run.before.power.min_v.toFixed(2)} V`, during: `${run.during.power.min_v.toFixed(2)} V` };
    case "continuity": return { label: "ECHO to GPIO18", before: "not checked", during: run.outcome === "FOUND" ? "open circuit" : "connected" };
    case "trigger-timing": return { label: "TRIG pulse width", before: "not measured", during: `${run.during.trig.width_us} µs (needs ≥ 10)` };
  }
}

const TEST_VERB: Record<TestID, string> = { wiggle: "Move the wire", "rail-load": "Stress the supply", continuity: "Check the wiring", "trigger-timing": "Time the pulse" };

// ---------------------------------------------------------------------------
// Stages
// ---------------------------------------------------------------------------

function Intro({ onStart }: { onStart: () => void }) {
  return <section className="bench-panel jd-hero" aria-labelledby="jd-stage-title">
    <span className="bench-label">Try it yourself · no hardware needed</span>
    <h1 id="jd-stage-title" tabIndex={-1}>Can you find the fault?</h1>
    <p className="jd-lead">This is a simulated project: an ESP32 reading an HC-SR04 distance sensor. You break it, then track down what went wrong from the evidence. None of it is real hardware; every reading is generated in your browser.</p>
    <ol className="jd-howto" aria-label="How it works">
      <li><span className="jd-howto-icon"><Zap size={15} /></span><b>Break it</b><small>Pick a fault, or a mystery</small></li>
      <li aria-hidden="true" className="jd-howto-arrow"><ArrowRight size={14} /></li>
      <li><span className="jd-howto-icon"><FlaskConical size={15} /></span><b>Test it</b><small>Watch the probes change</small></li>
      <li aria-hidden="true" className="jd-howto-arrow"><ArrowRight size={14} /></li>
      <li><span className="jd-howto-icon"><Wrench size={15} /></span><b>Fix it</b><small>VERIFY proves it worked</small></li>
    </ol>
    <div className="jd-actions"><button className="primary jd-big" onClick={onStart}>Let’s go <ArrowRight size={16} /></button><small>Takes about two minutes.</small></div>
  </section>;
}

function Choose({ onPick }: { onPick: (fault: FaultID | "mystery") => void }) {
  return <section className="bench-panel jd-panel jd-narrow" aria-labelledby="jd-stage-title">
    <span className="bench-label">Step 2 · simulated fault</span>
    <h1 id="jd-stage-title" tabIndex={-1}>MAKE IT WEIRD.</h1>
    <p className="jd-lead">What should go wrong? Pick the mystery for the real challenge.</p>
    <button className="jd-choice jd-choice-mystery" onClick={() => onPick("mystery")}>
      <CircleHelp size={22} /><span><b>Mystery fault</b><small>ReWeird secretly breaks something. You work out what.</small></span><ArrowRight size={16} />
    </button>
    <p className="jd-or">or pick one yourself</p>
    <div className="jd-choice-grid">
      {FAULT_ORDER.map((id) => <button key={id} className="jd-choice" onClick={() => onPick(id)}>
        <Zap size={16} /><span><b>{FAULTS[id].label}</b><small>{FAULTS[id].blurb}</small></span>
      </button>)}
    </div>
  </section>;
}

function Investigate({ state, running, onTest, onHint, onFix }: {
  state: DemoState; running: TestID | null; onTest: (test: TestID) => void; onHint: () => void; onFix: () => void;
}) {
  const fault = state.fault!;
  const capture = latestCapture(state);
  const bad = failing(current(state)).length;
  const latest = state.tests[state.tests.length - 1];
  const solved = found(state);
  const metric = latest ? testMetric(latest) : null;
  return <div className="jd-grid">
    <section className="bench-panel jd-panel" aria-labelledby="jd-stage-title">
      <span className="bench-label">Step 3 · the evidence <Sim /></span>
      <h1 id="jd-stage-title" tabIndex={-1}>SOMETHING’S WEIRD.</h1>
      <p className="jd-lead">{revealed(state)
        ? <>Fault: <b>{FAULTS[fault].label}</b>. {bad} of 4 checks are out of spec.</>
        : <><b>Mystery fault.</b> {bad} of 4 checks are out of spec. Which test explains it?</>}</p>
      <div className="jd-capture-label"><span className={`jd-live ${running ? "on" : ""}`} aria-hidden="true" />{running ? "Capturing a new window…" : capture.label}</div>
      <ProbeTiles snapshot={capture.snapshot} capturing={Boolean(running)} />
    </section>

    <section className="bench-panel jd-panel" aria-labelledby="jd-tests-title">
      <span className="bench-label">Step 3 · your move</span>
      <h2 id="jd-tests-title">{solved ? "Case cracked." : "Pick a test"}</h2>
      {!solved && <p className="jd-lead">A good test changes the evidence. A wrong guess leaves it the same.</p>}
      <div className="jd-test-grid">
        {TEST_ORDER.map((id) => {
          const done = state.tests.find((t) => t.test === id);
          return <button key={id} className={`jd-test ${done ? (done.outcome === "FOUND" ? "found" : "not-it") : ""} ${running === id ? "running" : ""}`}
            disabled={Boolean(done) || running !== null || solved} onClick={() => onTest(id)}>
            <small>{TEST_VERB[id]}</small>
            <strong>{TESTS[id].label}</strong>
            <span>{running === id ? "Capturing…" : done ? (done.outcome === "FOUND" ? "Found it" : "Ruled out") : `Watches ${TESTS[id].probe}`}</span>
          </button>;
        })}
      </div>

      <div aria-live="polite">
        {latest && metric && <article className={`weird-result-banner jd-result ${latest.outcome === "FOUND" ? "suspect" : "unresolved"}`}>
          <span className="bench-label">{TESTS[latest.test].label} <Sim /></span>
          <h3>{testHeadline(latest)}</h3>
          <p>{latest.finding}</p>
          <div className="jd-delta"><span>{metric.label}</span><b>{metric.before}</b><ArrowRight size={13} /><b>{metric.during}</b></div>
        </article>}
      </div>

      {solved ? <div className="jd-callout">
        <ShieldCheck size={18} />
        <div><strong>{state.mystery ? `The mystery was: ${FAULTS[fault].label.toLowerCase()}.` : "Evidence found."}</strong><p>{FAULTS[fault].reveal}</p></div>
        <button className="primary" onClick={onFix}>Fix it <ArrowRight size={15} /></button>
      </div> : <div className="jd-hints">
        {state.hints > 0 && <p><Lightbulb size={14} /> {hint(state, 1)}</p>}
        {state.hints > 1 && <p><Lightbulb size={14} /> {hint(state, 2)}</p>}
        {state.hints < 2 && <button className="text-button" onClick={onHint}><Lightbulb size={14} /> {state.hints === 0 ? "Need a hint?" : "Another hint"}</button>}
      </div>}
    </section>
  </div>;
}

function Fix({ state, onApply }: { state: DemoState; onApply: (fix: FixID) => void }) {
  const fault = state.fault!;
  const tried = new Set(state.fixes.map((f) => f.fix));
  const [applying, setApplying] = useState<FixID | null>(null);
  function apply(fix: FixID) {
    setApplying(fix);
    window.setTimeout(() => onApply(fix), CAPTURE_MS);
  }
  return <section className="bench-panel jd-panel jd-narrow" aria-labelledby="jd-stage-title">
    <span className="bench-label">Step 4 · simulated repair</span>
    <h1 id="jd-stage-title" tabIndex={-1}>Pick a fix.</h1>
    <p className="jd-lead">{FAULTS[fault].reveal} Which repair do you trust? ReWeird re-measures either way.</p>
    <div className="jd-choice-grid">
      {FIX_ORDER.map((id) => <button key={id} className={`jd-choice ${tried.has(id) ? "tried" : ""}`} disabled={applying !== null || tried.has(id)} onClick={() => apply(id)}>
        <Wrench size={16} />
        <span><b>{applying === id ? "Re-measuring…" : FIXES[id].label}</b><small>{tried.has(id) ? "Tried it. Still weird." : FIXES[id].detail}</small></span>
      </button>)}
    </div>
    <p className="jd-footnote"><LockKeyhole size={13} /> Simulated repair. Nothing is sent to any hardware.</p>
  </section>;
}

function Verify({ state, onRetry, onPassport }: { state: DemoState; onRetry: () => void; onPassport: () => void }) {
  const fault = state.fault!;
  const ok = resolved(state);
  const before = checks(FAULTS[fault].symptom);
  const after = checks(current(state));
  const lastFix = state.fixes[state.fixes.length - 1];
  return <section className="bench-panel jd-panel jd-narrow" aria-labelledby="jd-stage-title">
    <span className="bench-label">Step 5 · VERIFY <Sim /></span>
    <div className={`weird-result-banner jd-verdict ${ok ? "recovered" : "unresolved"}`}>
      <h1 id="jd-stage-title" tabIndex={-1}>{verifyHeadline(state)}</h1>
      <p>{ok
        ? "Every check passes again and ECHO is back to its healthy rate."
        : `“${FIXES[lastFix.fix].label}” didn’t change the evidence. The fault is still there.`}</p>
    </div>
    <div className="jd-table-wrap"><table className="jd-table">
      <thead><tr><th scope="col">Check (after fix)</th><th scope="col">Before</th><th scope="col">After</th></tr></thead>
      <tbody>{after.map((row, index) => <tr key={row.id}>
        <th scope="row"><span className="jd-probe-tag">{row.probe}</span>{row.message}</th>
        <td><StatusPill status={before[index].status} /></td>
        <td><StatusPill status={row.status} /></td>
      </tr>)}</tbody>
    </table></div>
    <div className="jd-actions">{ok
      ? <button className="primary jd-big" onClick={onPassport}><Fingerprint size={16} /> See the Device Passport <ArrowRight size={15} /></button>
      : <button className="primary jd-big" onClick={onRetry}><RotateCcw size={15} /> Try another fix</button>}</div>
  </section>;
}

function Passport({ state, onAgain }: { state: DemoState; onAgain: () => void }) {
  const fault = state.fault!;
  const record = incidentRecord(state)!;
  const seconds = state.startedAt && state.finishedAt ? Math.max(1, Math.round((state.finishedAt - state.startedAt) / 1000)) : null;
  const duration = seconds == null ? "—" : seconds < 60 ? `${seconds}s` : `${Math.floor(seconds / 60)}m ${seconds % 60}s`;
  function download() {
    const blob = new Blob([JSON.stringify(record, null, 2)], { type: "application/json" });
    const url = URL.createObjectURL(blob);
    const link = document.createElement("a");
    link.href = url; link.download = `reweird-simulated-incident-${fault}.json`;
    document.body.appendChild(link); link.click(); link.remove();
    window.setTimeout(() => URL.revokeObjectURL(url), 1000);
  }
  const steps = [
    { tone: "fail", title: state.mystery ? `Mystery fault: ${FAULTS[fault].label.toLowerCase()}` : `Made weird: ${FAULTS[fault].label.toLowerCase()}`, detail: `${failing(FAULTS[fault].symptom).length} of 4 checks out of spec.` },
    ...state.tests.map((t) => ({ tone: t.outcome === "FOUND" ? "fail" : "", title: `${TESTS[t.test].label}: ${t.outcome === "FOUND" ? "found it" : "ruled out"}`, detail: t.finding })),
    ...state.fixes.map((f) => ({ tone: f.resolved ? "pass" : "warn", title: `${FIXES[f.fix].label}: ${f.resolved ? "VERIFY passed" : "still weird"}`, detail: f.resolved ? "All 4 checks pass again." : "Evidence unchanged." })),
  ];
  return <section className="bench-panel jd-panel jd-narrow" aria-labelledby="jd-stage-title">
    <span className="bench-label">Step 6 · Device Passport <Sim /></span>
    <h1 id="jd-stage-title" tabIndex={-1}>NOT WEIRD ANYMORE.</h1>
    <div className="jd-passport-head">
      <span className="project-thumbnail"><Cpu size={24} strokeWidth={1.25} /></span>
      <div><strong>Ultrasonic Distance Sensor</strong><small>ESP32 + HC-SR04 · simulated project</small></div>
      <span className="jd-status">Simulated</span>
    </div>
    <div className="jd-stats">
      <div><strong>{duration}</strong><small>to a verified fix</small></div>
      <div><strong>{state.tests.length}</strong><small>{state.tests.length === 1 ? "test run" : "tests run"}</small></div>
      <div><strong>{state.fixes.length}</strong><small>{state.fixes.length === 1 ? "fix tried" : "fixes tried"}</small></div>
      <div><strong>{state.hints}</strong><small>{state.hints === 1 ? "hint used" : "hints used"}</small></div>
    </div>
    <ol className="jd-timeline">{steps.map((item, index) => <li key={index} className={item.tone}><span className="jd-timeline-dot" /><div><strong>{item.title}</strong><small>{item.detail}</small></div></li>)}</ol>
    <p className="jd-footnote"><LockKeyhole size={13} /> Simulated record. Not a physical baseline, and not saved to any server.</p>
    <div className="jd-actions">
      <button className="primary jd-big" onClick={onAgain}><Zap size={15} /> Play again</button>
      <button className="secondary" onClick={download}><Download size={15} /> Download record</button>
      <Link className="text-button" href="/demo">Back to demo choices</Link>
    </div>
  </section>;
}

// ---------------------------------------------------------------------------

export function JudgeDemo() {
  const [state, dispatch] = useReducer(reduce, initialState);
  const [running, setRunning] = useState<TestID | null>(null);
  const { theme, toggle } = useTheme();
  const timer = useRef<number | null>(null);
  const mainRef = useRef<HTMLDivElement>(null);
  const step = STAGE_STEP[state.stage];

  // Move focus to each new stage heading so keyboard and screen-reader users follow along.
  useEffect(() => {
    if (state.stage === "intro") return;
    mainRef.current?.querySelector<HTMLElement>("#jd-stage-title")?.focus({ preventScroll: true });
    window.scrollTo({ top: 0, behavior: "smooth" });
  }, [state.stage, state.fixes.length]);
  useEffect(() => () => { if (timer.current) window.clearTimeout(timer.current); }, []);
  // On narrow screens the result sits below the tests; bring it into view.
  useEffect(() => {
    if (state.tests.length) mainRef.current?.querySelector(".jd-result")?.scrollIntoView({ behavior: "smooth", block: "nearest" });
  }, [state.tests.length]);

  function runTest(test: TestID) {
    setRunning(test);
    timer.current = window.setTimeout(() => { dispatch({ type: "run-test", test }); setRunning(null); }, CAPTURE_MS);
  }

  function stopCapture() {
    if (timer.current) window.clearTimeout(timer.current);
    setRunning(null);
  }
  function startOver() { stopCapture(); dispatch({ type: "reset" }); }
  function back() { stopCapture(); dispatch({ type: "back" }); }

  return <div className="jd-page">
    <div className="jd-banner" role="note">
      <span className="jd-banner-tag">SIMULATED DEMO</span>
      <span>No hardware connected. Every reading here is generated in your browser.</span>
      <Link href="/demo" className="jd-banner-exit">Exit</Link>
    </div>
    <header className="jd-header">
      <Link href="/demo" className="jd-brand" aria-label="Back to demo choices"><img src="/images/reweird-logo-mark.png" alt="" width={52} height={26} /><span>ReWeird</span></Link>
      <ol className="jd-journey" aria-label="Progress">
        {JOURNEY.map((label, index) => <li key={label} className={index === step ? "current" : index < step ? "done" : ""} aria-current={index === step ? "step" : undefined}>
          <b>{index < step ? <Check size={10} /> : index + 1}</b><span>{label}</span>
        </li>)}
      </ol>
      <div className="jd-header-tools">
        {canGoBack(state) && <button className="jd-tool" onClick={back}><ArrowLeft size={14} /> Back</button>}
        {state.stage !== "intro" && <button className="jd-tool" onClick={startOver}><RotateCcw size={14} /> Start over</button>}
        <button className="jd-tool jd-tool-icon" onClick={toggle} aria-label={`Switch to ${theme === "dark" ? "light" : "dark"} mode`} title={theme === "dark" ? "Light mode" : "Dark mode"}>{theme === "dark" ? <Sun size={15} /> : <Moon size={15} />}</button>
      </div>
    </header>
    <main className="jd-main" ref={mainRef}>
      {state.stage === "intro" && <Intro onStart={() => dispatch({ type: "start", now: Date.now() })} />}
      {state.stage === "choose" && <Choose onPick={(fault) => dispatch({ type: "make-weird", fault })} />}
      {state.stage === "investigate" && <Investigate state={state} running={running} onTest={runTest} onHint={() => dispatch({ type: "hint" })} onFix={() => dispatch({ type: "go-fix" })} />}
      {state.stage === "fix" && <Fix key={state.fixes.length} state={state} onApply={(fix) => dispatch({ type: "apply-fix", fix })} />}
      {state.stage === "verify" && <Verify state={state} onRetry={() => dispatch({ type: "retry" })} onPassport={() => dispatch({ type: "passport", now: Date.now() })} />}
      {state.stage === "passport" && <Passport state={state} onAgain={() => dispatch({ type: "start", now: Date.now() })} />}
    </main>
  </div>;
}
