"use client";

import { useEffect, useReducer, useRef, useState, type KeyboardEvent } from "react";
import Link from "next/link";
import {
  ArrowLeft, ArrowRight, Check, CircleDot, CircleHelp, Clock, Download, FlaskConical, Lightbulb,
  LockKeyhole, Moon, RotateCcw, Search, Star, Sun, TriangleAlert, Trophy, Wrench, X, Zap,
} from "lucide-react";
import {
  FAULTS, FAULT_ORDER, FIXES, FIX_ORDER, SCORE, TESTS, TEST_ORDER,
  canGoBack, checks, current, hint, incidentRecord, initialState, probeChecks, rank, reduce, revealed, testHeadline, waveform,
  type Check as EvidenceCheck, type DemoState, type FaultID, type FixID, type ProbeID, type Snapshot, type TestID,
} from "@/lib/judge-demo";

// A simulated capture takes this long: long enough to feel like a measurement.
const CAPTURE_MS = 1100;
const STEPS = ["Brief", "Break", "Investigate", "Call it", "Fix", "Solved"] as const;
const STAGE_STEP: Record<DemoState["stage"], number> = { intro: 0, choose: 1, investigate: 2, call: 3, fix: 4, result: 5 };
const PROBES: { id: ProbeID; role: string; what: string }[] = [
  { id: "P1", role: "POWER", what: "5 V supply rail" },
  { id: "P2", role: "TRIG", what: "ESP32 fires the sensor" },
  { id: "P3", role: "ECHO", what: "Sensor’s reply" },
];
const TEST_WIRE: Record<TestID, ProbeID> = { wiggle: "P3", "rail-load": "P1", continuity: "P3", "trigger-timing": "P2" };

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

// Per-browser case file: which faults this visitor has solved, and their best score.
type CaseFile = { solved: FaultID[]; best: number };
const CASE_KEY = "reweird-judge-cases";
function useCaseFile() {
  const [file, setFile] = useState<CaseFile>({ solved: [], best: 0 });
  useEffect(() => {
    try {
      const raw = window.localStorage.getItem(CASE_KEY);
      if (raw) { const parsed = JSON.parse(raw) as CaseFile; if (Array.isArray(parsed.solved)) setFile({ solved: parsed.solved, best: Number(parsed.best) || 0 }); }
    } catch { /* storage unavailable */ }
  }, []);
  const record = (fault: FaultID, score: number) => setFile((prev) => {
    const next = { solved: prev.solved.includes(fault) ? prev.solved : [...prev.solved, fault], best: Math.max(prev.best, score) };
    try { window.localStorage.setItem(CASE_KEY, JSON.stringify(next)); } catch { /* keep in-session */ }
    return next;
  });
  return { file, record };
}

function useReducedMotion() {
  const [reduced, setReduced] = useState(false);
  useEffect(() => {
    const query = window.matchMedia("(prefers-reduced-motion: reduce)");
    setReduced(query.matches);
    const listen = () => setReduced(query.matches);
    query.addEventListener("change", listen);
    return () => query.removeEventListener("change", listen);
  }, []);
  return reduced;
}

// ---------------------------------------------------------------------------
// Instruments
// ---------------------------------------------------------------------------

/** Three-channel oscilloscope. Traces scroll; each fault leaves a readable shape. */
function Scope({ snapshot, selected, onSelect, capturing, compact = false }: {
  snapshot: Snapshot; selected?: ProbeID | null; onSelect?: (probe: ProbeID) => void; capturing?: boolean; compact?: boolean;
}) {
  const reduced = useReducedMotion();
  const [phase, setPhase] = useState(0);
  useEffect(() => {
    if (reduced) return;
    let frame = 0;
    let last = performance.now();
    const tick = (now: number) => {
      const dt = Math.max(0, now - last) / (capturing ? 2600 : 7000);
      setPhase((p) => (p + dt) % 1);
      last = now;
      frame = requestAnimationFrame(tick);
    };
    frame = requestAnimationFrame(tick);
    return () => cancelAnimationFrame(frame);
  }, [reduced, capturing]);
  const W = 600, rowH = compact ? 46 : 64, N = 180;
  return <div className={`jd-scope ${capturing ? "capturing" : ""}`}>
    <svg viewBox={`0 0 ${W} ${rowH * 3}`} preserveAspectRatio="none" role="img" aria-label="Simulated oscilloscope: P1 power, P2 trigger, P3 echo">
      {Array.from({ length: 13 }, (_, i) => <line key={`v${i}`} className="jd-grid-line" x1={(i * W) / 12} x2={(i * W) / 12} y1={0} y2={rowH * 3} />)}
      {[1, 2].map((i) => <line key={`h${i}`} className="jd-grid-line strong" x1={0} x2={W} y1={i * rowH} y2={i * rowH} />)}
      {PROBES.map((probe, row) => {
        const values = waveform(snapshot, probe.id, N, phase);
        const points = values.map((v, i) => `${(i / (N - 1)) * W},${row * rowH + rowH - v * rowH}`).join(" ");
        return <polyline key={probe.id} className={`jd-trace ${probe.id.toLowerCase()} ${selected && selected !== probe.id ? "dim" : ""}`} points={points} fill="none" />;
      })}
    </svg>
    <div className="jd-scope-labels" style={{ gridTemplateRows: `repeat(3, ${rowH}px)` }}>
      {PROBES.map((probe) => onSelect
        ? <button key={probe.id} className={`jd-scope-label ${probe.id.toLowerCase()} ${selected === probe.id ? "on" : ""}`} onClick={() => onSelect(probe.id)} aria-pressed={selected === probe.id}>{probe.id} {probe.role}</button>
        : <span key={probe.id} className={`jd-scope-label ${probe.id.toLowerCase()}`}>{probe.id} {probe.role}</span>)}
    </div>
    {capturing && <div className="jd-scope-status" role="status"><span className="jd-rec" /> Capturing</div>}
  </div>;
}

/** Schematic bench: ESP32 wired to an HC-SR04, with clickable probe clips. */
function Bench({ selected, inspected, active, onProbe }: { selected: ProbeID | null; inspected: ProbeID[]; active: ProbeID | null; onProbe: (probe: ProbeID) => void }) {
  const wires: { id: ProbeID | "GND"; y: number; label: string }[] = [
    { id: "P1", y: 46, label: "5V" }, { id: "P2", y: 92, label: "TRIG" }, { id: "P3", y: 138, label: "ECHO" }, { id: "GND", y: 184, label: "GND" },
  ];
  return <svg className="jd-bench" viewBox="0 0 600 220" role="group" aria-label="Circuit with probe clips">
    <rect className="jd-board" x={18} y={20} width={150} height={184} rx={10} />
    <text className="jd-board-title" x={93} y={48} textAnchor="middle">ESP32</text>
    <rect className="jd-chip" x={58} y={82} width={70} height={70} rx={6} />
    <rect className="jd-board" x={432} y={20} width={150} height={184} rx={10} />
    <text className="jd-board-title" x={507} y={48} textAnchor="middle">HC-SR04</text>
    <circle className="jd-eye" cx={478} cy={120} r={24} /><circle className="jd-eye" cx={536} cy={120} r={24} />
    {wires.map((wire) => {
      const probe = wire.id === "GND" ? null : (wire.id as ProbeID);
      const cls = `jd-wire ${wire.id.toLowerCase()} ${probe && active === probe ? "active" : ""} ${probe && selected === probe ? "selected" : ""}`;
      return <g key={wire.id}>
        <path className={cls} d={`M168 ${wire.y} C 250 ${wire.y}, 350 ${wire.y}, 432 ${wire.y}`} />
        <text className="jd-wire-label" x={176} y={wire.y - 7}>{wire.label}</text>
        {probe && <g className={`jd-clip ${selected === probe ? "on" : ""} ${inspected.includes(probe) ? "seen" : ""}`} role="button" tabIndex={0} aria-label={`Inspect ${probe}`}
          onClick={() => onProbe(probe)} onKeyDown={(e: KeyboardEvent<SVGGElement>) => { if (e.key === "Enter" || e.key === " ") { e.preventDefault(); onProbe(probe); } }}>
          <circle cx={300} cy={wire.y} r={15} />
          <text x={300} y={wire.y + 4} textAnchor="middle">{probe}</text>
        </g>}
      </g>;
    })}
  </svg>;
}

function StatusPill({ status }: { status: EvidenceCheck["status"] }) {
  const Icon = status === "pass" ? Check : status === "warn" ? CircleDot : TriangleAlert;
  return <span className={`jd-pill ${status}`}><Icon size={11} />{status === "pass" ? "OK" : status === "warn" ? "Odd" : "Fail"}</span>;
}

function Sim() { return <span className="jd-sim-badge">SIMULATED</span>; }

function Stars({ count }: { count: number }) {
  return <span className="jd-stars" aria-label={`${count} of 3 stars`}>{[1, 2, 3].map((i) => <Star key={i} size={22} className={i <= count ? "on" : ""} />)}</span>;
}

// ---------------------------------------------------------------------------
// Stages
// ---------------------------------------------------------------------------

function Intro({ onStart, file }: { onStart: () => void; file: CaseFile }) {
  return <section className="jd-hero" aria-labelledby="jd-stage-title">
    <div className="jd-hero-copy">
      <span className="bench-label">Try it yourself · no hardware needed</span>
      <h1 id="jd-stage-title" tabIndex={-1}>Find the fault.</h1>
      <p className="jd-catchphrase">When hardware gets weird, <em>ReWeird it.</em></p>
      <p className="jd-lead">Something inside this ultrasonic sensor project is broken. Read the scope, run tests, name the cause, and prove your fix before your points run out.</p>
      <ul className="jd-rules" aria-label="Scoring">
        <li><Search size={15} /><span>Inspect a probe</span><b className="free">free</b></li>
        <li><FlaskConical size={15} /><span>Run a test</span><b>−10 to −15</b></li>
        <li><Lightbulb size={15} /><span>Hint</span><b>−{SCORE.hint}</b></li>
        <li><X size={15} /><span>Wrong call / wrong fix</span><b>−{SCORE.wrongCall} / −{SCORE.wrongFix}</b></li>
      </ul>
      <div className="jd-actions"><button className="primary jd-big" onClick={onStart}>Take a case <ArrowRight size={16} /></button><small>You start with {SCORE.start} points. About two minutes.</small></div>
      <CaseStrip file={file} />
    </div>
    <div className="jd-hero-scope" aria-hidden="true"><Scope snapshot={FAULTS["loose-echo"].symptom} compact /></div>
  </section>;
}

function CaseStrip({ file }: { file: CaseFile }) {
  return <div className="jd-casefile" aria-label="Your case file">
    <span className="bench-label">Case file {file.solved.length}/{FAULT_ORDER.length}</span>
    <div>{FAULT_ORDER.map((id) => {
      const done = file.solved.includes(id);
      return <span key={id} className={`jd-case-chip ${done ? "done" : ""}`}>{done ? <Check size={11} /> : <CircleHelp size={11} />}{done ? FAULTS[id].label : "Unsolved"}</span>;
    })}</div>
    {file.best > 0 && <span className="jd-best"><Trophy size={13} /> Best {file.best}</span>}
  </div>;
}

function Choose({ onPick, file }: { onPick: (fault: FaultID | "mystery") => void; file: CaseFile }) {
  const left = FAULT_ORDER.length - file.solved.length;
  return <section className="bench-panel jd-panel jd-narrow" aria-labelledby="jd-stage-title">
    <span className="bench-label">Step 2 · simulated fault</span>
    <h1 id="jd-stage-title" tabIndex={-1}>MAKE IT WEIRD.</h1>
    <p className="jd-lead">ReWeird secretly breaks something. Your job is to work out what.</p>
    <button className="jd-choice jd-choice-mystery" onClick={() => onPick("mystery")}>
      <CircleHelp size={26} /><span><b>Mystery case</b><small>{left > 0 ? `${left} unsolved ${left === 1 ? "case" : "cases"} left in your file.` : "You’ve solved them all. Go for a better score."}</small></span><ArrowRight size={18} />
    </button>
    <details className="jd-practice">
      <summary>Or practice a known fault</summary>
      <div className="jd-choice-grid">
        {FAULT_ORDER.map((id) => <button key={id} className="jd-choice" onClick={() => onPick(id)}>
          <Zap size={16} /><span><b>{FAULTS[id].label}</b><small>{FAULTS[id].blurb}</small></span>
        </button>)}
      </div>
    </details>
  </section>;
}

function Investigate({ state, running, onInspect, onTest, onHint, onCall }: {
  state: DemoState; running: TestID | null; onInspect: (probe: ProbeID) => void; onTest: (test: TestID) => void; onHint: () => void; onCall: () => void;
}) {
  const [selected, setSelected] = useState<ProbeID | null>(null);
  const latest = state.tests[state.tests.length - 1];
  const snapshot = latest ? latest.during : current(state);
  const pick = (probe: ProbeID) => { setSelected(probe); onInspect(probe); };
  const readout = selected ? PROBES.find((p) => p.id === selected)! : null;
  const reading = (probe: ProbeID) => probe === "P1" ? `${snapshot.power.min_v.toFixed(2)} V lowest`
    : probe === "P2" ? `${snapshot.trig.width_us} µs pulses`
    : snapshot.echo.rate_hz === 0 ? "no replies" : `${snapshot.echo.rate_hz.toFixed(1)} replies/s`;
  return <div className="jd-grid">
    <section className="bench-panel jd-panel" aria-labelledby="jd-stage-title">
      <div className="jd-panel-top"><span className="bench-label">Step 3 · the bench <Sim /></span><span className="jd-case-tag">{revealed(state) ? FAULTS[state.fault!].label : "Mystery case"}</span></div>
      <h1 id="jd-stage-title" tabIndex={-1}>SOMETHING’S WEIRD.</h1>
      <p className="jd-catchphrase">Time to <em>ReWeird it.</em></p>
      <p className="jd-lead">The distance readings are unreliable. Click a probe to inspect it (free), or read the scope yourself.</p>
      <Bench selected={selected} inspected={state.inspected} active={running ? TEST_WIRE[running] : null} onProbe={pick} />
      <Scope snapshot={snapshot} selected={selected} onSelect={pick} capturing={Boolean(running)} />
      <div className="jd-scope-caption">{running ? `Running: ${TESTS[running].action.toLowerCase()}…` : latest ? `Showing: during the ${TESTS[latest.test].label.toLowerCase()}` : "Showing: first capture"}</div>
      <div className="jd-readout" aria-live="polite">
        {readout ? <>
          <div className="jd-readout-head"><b className={readout.id.toLowerCase()}>{readout.id} {readout.role}</b><span>{readout.what}</span><strong>{reading(readout.id)}</strong></div>
          {probeChecks(snapshot, readout.id).map((c) => <div key={c.id} className={`jd-check ${c.status}`}><StatusPill status={c.status} /><span>{c.message}</span></div>)}
        </> : <p className="jd-muted"><Search size={14} /> Pick P1, P2 or P3 to see its reading and checks.</p>}
      </div>
    </section>

    <section className="bench-panel jd-panel jd-toolkit" aria-labelledby="jd-tests-title">
      <span className="bench-label">Step 3 · your toolkit</span>
      <h2 id="jd-tests-title">Run a test</h2>
      <div className="jd-test-grid">
        {TEST_ORDER.map((id) => {
          const done = state.tests.find((t) => t.test === id);
          return <button key={id} className={`jd-test ${done ? (done.outcome === "FOUND" ? "found" : "not-it") : ""} ${running === id ? "running" : ""}`}
            disabled={Boolean(done) || running !== null} onClick={() => onTest(id)}>
            <span className="jd-cost">{done ? (done.outcome === "FOUND" ? "Found it" : "Ruled out") : running === id ? "Capturing…" : `−${SCORE.test[id]}`}</span>
            <strong>{TESTS[id].action}</strong>
            <small>{TESTS[id].label} · {TESTS[id].probe}</small>
          </button>;
        })}
      </div>
      <div className="jd-notebook" aria-live="polite">
        <span className="bench-label">Evidence notebook</span>
        {state.tests.length === 0 && state.hints === 0 && <p className="jd-muted">Findings from your tests will show up here.</p>}
        {[...state.tests].reverse().map((run, i) => <article key={run.test} className={`jd-note ${run.outcome === "FOUND" ? "found" : ""} ${i === 0 ? "latest" : ""}`}>
          <b>{testHeadline(run)}</b><span>{TESTS[run.test].label}: {run.finding}</span>
        </article>)}
        {state.hints > 0 && <article className="jd-note hint"><b><Lightbulb size={13} /> Hint</b><span>{hint(state, 1)}</span></article>}
        {state.hints > 1 && <article className="jd-note hint"><b><Lightbulb size={13} /> Hint</b><span>{hint(state, 2)}</span></article>}
      </div>
      <div className="jd-toolkit-actions">
        {state.hints < 2 && <button className="text-button jd-hint-button" onClick={onHint} disabled={running !== null}><Lightbulb size={14} /> {state.hints === 0 ? "Hint" : "Another hint"} <span className="jd-cost-inline">−{SCORE.hint}</span></button>}
        <button className="primary jd-big" onClick={onCall} disabled={running !== null} aria-label="ReWeird it: name the root cause">ReWeird it <ArrowRight size={16} /></button>
      </div>
    </section>
  </div>;
}

function Call({ state, onCall }: { state: DemoState; onCall: (fault: FaultID) => void }) {
  const [shake, setShake] = useState<FaultID | null>(null);
  const pick = (fault: FaultID) => {
    if (fault !== state.fault) { setShake(fault); window.setTimeout(() => setShake(null), 500); }
    onCall(fault);
  };
  return <section className="bench-panel jd-panel jd-narrow" aria-labelledby="jd-stage-title">
    <span className="bench-label">Step 4 · call it</span>
    <h1 id="jd-stage-title" tabIndex={-1}>What’s the root cause?</h1>
    <p className="jd-lead">Choose carefully: a wrong call costs {SCORE.wrongCall} points. You can go back to the bench first.</p>
    <div className="jd-suspects">
      {FAULT_ORDER.map((id) => {
        const wrong = state.wrongCalls.includes(id);
        return <button key={id} className={`jd-suspect ${wrong ? "wrong" : ""} ${shake === id ? "shake" : ""}`} disabled={wrong} onClick={() => pick(id)}>
          <span className="jd-suspect-mark">{wrong ? <X size={18} /> : <CircleHelp size={18} />}</span>
          <b>{FAULTS[id].cause}</b>
          <small>{wrong ? `Not it. −${SCORE.wrongCall}` : FAULTS[id].blurb}</small>
        </button>;
      })}
    </div>
  </section>;
}

function Fix({ state, onApply }: { state: DemoState; onApply: (fix: FixID) => void }) {
  const fault = state.fault!;
  const tried = new Set(state.fixes.map((f) => f.fix));
  const [applying, setApplying] = useState<FixID | null>(null);
  const lastFailed = state.fixes.length > 0 && !state.fixes[state.fixes.length - 1].resolved;
  function apply(fix: FixID) {
    setApplying(fix);
    window.setTimeout(() => { onApply(fix); setApplying(null); }, CAPTURE_MS);
  }
  return <section className="bench-panel jd-panel jd-narrow" aria-labelledby="jd-stage-title">
    <span className="bench-label">Step 5 · fix and VERIFY <Sim /></span>
    <h1 id="jd-stage-title" tabIndex={-1}>Correct: {FAULTS[fault].cause.toLowerCase()}.</h1>
    <p className="jd-lead">{FAULTS[fault].reveal} Now fix it. ReWeird re-measures and only calls it fixed if every check passes.</p>
    {lastFailed && !applying && <div className="weird-result-banner unresolved jd-verdict-inline" role="status"><b>STILL WEIRD.</b> That repair didn’t change the evidence. −{SCORE.wrongFix}</div>}
    <Scope snapshot={current(state)} capturing={applying !== null} compact />
    <div className="jd-choice-grid jd-fixes">
      {FIX_ORDER.map((id) => <button key={id} className={`jd-choice ${tried.has(id) ? "tried" : ""}`} disabled={applying !== null || tried.has(id)} onClick={() => apply(id)}>
        <Wrench size={16} />
        <span><b>{applying === id ? "Re-measuring…" : FIXES[id].label}</b><small>{tried.has(id) ? "Tried. Still weird." : FIXES[id].detail}</small></span>
      </button>)}
    </div>
    <p className="jd-footnote"><LockKeyhole size={13} /> Simulated repair. Nothing is sent to any hardware.</p>
  </section>;
}

function Result({ state, file, onNext }: { state: DemoState; file: CaseFile; onNext: () => void }) {
  const fault = state.fault!;
  const { title, stars } = rank(state.score);
  const record = incidentRecord(state)!;
  const seconds = state.startedAt && state.finishedAt ? Math.max(1, Math.round((state.finishedAt - state.startedAt) / 1000)) : null;
  const duration = seconds == null ? "—" : seconds < 60 ? `${seconds}s` : `${Math.floor(seconds / 60)}m ${seconds % 60}s`;
  const [shown, setShown] = useState(0);
  useEffect(() => {
    let frame = 0; const start = performance.now();
    const tick = (now: number) => { const t = Math.min(1, (now - start) / 900); setShown(Math.round(state.score * (1 - Math.pow(1 - t, 3)))); if (t < 1) frame = requestAnimationFrame(tick); };
    frame = requestAnimationFrame(tick);
    // Hidden tabs freeze animation frames and timers; never leave the count stuck.
    const settle = () => setShown(state.score);
    if (document.hidden) settle();
    const timeout = window.setTimeout(settle, 1000);
    document.addEventListener("visibilitychange", settle);
    return () => { cancelAnimationFrame(frame); window.clearTimeout(timeout); document.removeEventListener("visibilitychange", settle); };
  }, [state.score]);
  function download() {
    const blob = new Blob([JSON.stringify(record, null, 2)], { type: "application/json" });
    const url = URL.createObjectURL(blob);
    const link = document.createElement("a");
    link.href = url; link.download = `reweird-simulated-case-${fault}.json`;
    document.body.appendChild(link); link.click(); link.remove();
    window.setTimeout(() => URL.revokeObjectURL(url), 1000);
  }
  const after = checks(current(state));
  const before = checks(FAULTS[fault].symptom);
  return <section className="bench-panel jd-panel jd-narrow jd-result-card" aria-labelledby="jd-stage-title">
    <span className="bench-label">Case closed · Device Passport <Sim /></span>
    <h1 id="jd-stage-title" tabIndex={-1}>NOT WEIRD ANYMORE.</h1>
    <p className="jd-catchphrase">You <em>ReWeirded it.</em></p>
    <div className="jd-scoreboard">
      <div className="jd-score-big"><strong>{shown}</strong><small>/ {SCORE.start} points</small></div>
      <div className="jd-rank"><Stars count={stars} /><b>{title}</b><small>{state.mystery ? "Mystery" : "Practice"} case: {FAULTS[fault].label.toLowerCase()}</small></div>
    </div>
    <div className="jd-stats">
      <div><strong>{duration}</strong><small>time</small></div>
      <div><strong>{state.tests.length}</strong><small>{state.tests.length === 1 ? "test" : "tests"}</small></div>
      <div><strong>{state.wrongCalls.length + state.fixes.filter((f) => !f.resolved).length}</strong><small>wrong answers</small></div>
      <div><strong>{state.hints}</strong><small>{state.hints === 1 ? "hint" : "hints"}</small></div>
    </div>
    <div className="jd-table-wrap"><table className="jd-table">
      <thead><tr><th scope="col">VERIFY</th><th scope="col">Before</th><th scope="col">After</th></tr></thead>
      <tbody>{after.map((row, i) => <tr key={row.id}><th scope="row"><span className="jd-probe-tag">{row.probe}</span>{row.message}</th><td><StatusPill status={before[i].status} /></td><td><StatusPill status={row.status} /></td></tr>)}</tbody>
    </table></div>
    <CaseStrip file={file} />
    <div className="jd-actions">
      <button className="primary jd-big" onClick={onNext}><Zap size={15} /> Next case</button>
      <button className="secondary" onClick={download}><Download size={15} /> Download record</button>
      <Link className="text-button" href="/demo">Back to demo choices</Link>
    </div>
    <p className="jd-footnote"><LockKeyhole size={13} /> Simulated record. Not a physical baseline, and not saved to any server.</p>
  </section>;
}

// ---------------------------------------------------------------------------

export function JudgeDemo() {
  const [state, dispatch] = useReducer(reduce, initialState);
  const [running, setRunning] = useState<TestID | null>(null);
  const [floater, setFloater] = useState<{ id: number; amount: number } | null>(null);
  const [now, setNow] = useState(() => Date.now());
  const { theme, toggle } = useTheme();
  const { file, record } = useCaseFile();
  const timer = useRef<number | null>(null);
  const prevScore = useRef(state.score);
  const mainRef = useRef<HTMLDivElement>(null);
  const step = STAGE_STEP[state.stage];
  const inCase = state.stage === "investigate" || state.stage === "call" || state.stage === "fix";

  useEffect(() => {
    if (state.stage === "intro") return;
    mainRef.current?.querySelector<HTMLElement>("#jd-stage-title")?.focus({ preventScroll: true });
    window.scrollTo({ top: 0, behavior: "smooth" });
  }, [state.stage]);
  useEffect(() => () => { if (timer.current) window.clearTimeout(timer.current); }, []);
  // Show a floating "−N" whenever points are spent.
  useEffect(() => {
    const diff = state.score - prevScore.current;
    prevScore.current = state.score;
    if (diff < 0) { setFloater({ id: Date.now(), amount: diff }); const t = window.setTimeout(() => setFloater(null), 1200); return () => window.clearTimeout(t); }
  }, [state.score]);
  // Case clock.
  useEffect(() => {
    if (!inCase) return;
    const t = window.setInterval(() => setNow(Date.now()), 1000);
    return () => window.clearInterval(t);
  }, [inCase]);
  // File the solved case once.
  useEffect(() => {
    if (state.stage === "result" && state.fault) record(state.fault, state.score);
    // eslint-disable-next-line react-hooks/exhaustive-deps
  }, [state.stage]);

  function runTest(test: TestID) {
    setRunning(test);
    timer.current = window.setTimeout(() => { dispatch({ type: "run-test", test }); setRunning(null); }, CAPTURE_MS);
  }
  function stopCapture() { if (timer.current) window.clearTimeout(timer.current); setRunning(null); }
  function startOver() { stopCapture(); dispatch({ type: "reset" }); }
  function back() { stopCapture(); dispatch({ type: "back" }); }
  function nextCase() {
    dispatch({ type: "start", now: Date.now() });
    dispatch({ type: "make-weird", fault: "mystery", exclude: state.fault ? [...file.solved, state.fault] : file.solved });
  }
  const elapsed = state.startedAt ? Math.max(0, Math.floor(((state.finishedAt ?? now) - state.startedAt) / 1000)) : 0;

  return <div className="jd-page">
    <div className="jd-banner" role="note">
      <span className="jd-banner-tag">SIMULATED DEMO</span>
      <span>No hardware connected. Every reading here is generated in your browser.</span>
      <Link href="/demo" className="jd-banner-exit">Exit</Link>
    </div>
    <header className="jd-header">
      <Link href="/demo" className="jd-brand" aria-label="Back to demo choices"><img src="/images/reweird-logo-mark.png" alt="" width={52} height={26} /><span>ReWeird</span></Link>
      <ol className="jd-journey" aria-label="Progress">
        {STEPS.map((label, index) => <li key={label} className={index === step ? "current" : index < step ? "done" : ""} aria-current={index === step ? "step" : undefined}>
          <b>{index < step ? <Check size={10} /> : index + 1}</b><span>{label}</span>
        </li>)}
      </ol>
      <div className="jd-header-tools">
        {state.stage !== "intro" && state.stage !== "choose" && <div className="jd-hud" aria-label="Score and time">
          <span className="jd-hud-score"><Star size={13} /> <b>{state.score}</b>{floater && <em key={floater.id} className="jd-floater">{floater.amount}</em>}</span>
          <span className="jd-hud-time"><Clock size={13} /> {Math.floor(elapsed / 60)}:{String(elapsed % 60).padStart(2, "0")}</span>
        </div>}
        {canGoBack(state) && <button className="jd-tool" onClick={back}><ArrowLeft size={14} /> Back</button>}
        {state.stage !== "intro" && <button className="jd-tool" onClick={startOver}><RotateCcw size={14} /> Start over</button>}
        <button className="jd-tool jd-tool-icon" onClick={toggle} aria-label={`Switch to ${theme === "dark" ? "light" : "dark"} mode`} title={theme === "dark" ? "Light mode" : "Dark mode"}>{theme === "dark" ? <Sun size={15} /> : <Moon size={15} />}</button>
      </div>
    </header>
    <main className="jd-main" ref={mainRef}>
      {state.stage === "intro" && <Intro file={file} onStart={() => dispatch({ type: "start", now: Date.now() })} />}
      {state.stage === "choose" && <Choose file={file} onPick={(fault) => dispatch({ type: "make-weird", fault, exclude: file.solved })} />}
      {state.stage === "investigate" && <Investigate state={state} running={running}
        onInspect={(probe) => dispatch({ type: "inspect", probe })} onTest={runTest} onHint={() => dispatch({ type: "hint" })} onCall={() => dispatch({ type: "go-call" })} />}
      {state.stage === "call" && <Call state={state} onCall={(fault) => dispatch({ type: "call", fault })} />}
      {state.stage === "fix" && <Fix state={state} onApply={(fix) => dispatch({ type: "apply-fix", fix, now: Date.now() })} />}
      {state.stage === "result" && <Result state={state} file={file} onNext={nextCase} />}
    </main>
  </div>;
}
