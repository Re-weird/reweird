// Software-only judge game engine ("Find the fault").
//
// Everything here runs in the browser with no hardware, no Go API, and no
// sign-in. Every reading comes from fixed, deterministic tables below and is
// always labelled SIMULATED in the UI. The same deterministic checks are
// applied to every snapshot, so the evidence changes for a reason.
//
// Scoring rewards reading the evidence: inspecting probes is free, tests and
// hints cost points, and wrong calls or fixes cost more.

export type FaultID = "loose-echo" | "unstable-power" | "missing-echo" | "timing-drift";
export type TestID = "wiggle" | "rail-load" | "continuity" | "trigger-timing";
export type FixID = "reseat-echo" | "stiffen-supply" | "reconnect-echo" | "restore-trigger";
export type ProbeID = "P1" | "P2" | "P3";
export type Stage = "intro" | "choose" | "investigate" | "call" | "fix" | "result";
export type CheckStatus = "pass" | "warn" | "fail";
export type TestOutcome = "FOUND" | "NOT_IT";

/** One simulated capture window across the three HC-SR04 probes. */
export interface Snapshot {
  power: { mean_v: number; min_v: number; samples: number[] };
  trig: { rate_hz: number; width_us: number; samples: number[] };
  echo: { rate_hz: number; dropouts_per_min: number; samples: number[] };
}

export interface Check {
  id: "rail" | "trigger-width" | "echo-activity" | "echo-dropouts";
  probe: ProbeID;
  status: CheckStatus;
  message: string;
}

export interface TestRun {
  test: TestID;
  outcome: TestOutcome;
  before: Snapshot;
  during: Snapshot;
  finding: string;
}

export interface FixAttempt {
  fix: FixID;
  after: Snapshot;
  resolved: boolean;
}

export interface DemoState {
  stage: Stage;
  fault: FaultID | null;
  mystery: boolean;
  inspected: ProbeID[];
  tests: TestRun[];
  wrongCalls: FaultID[];
  fixes: FixAttempt[];
  hints: number;
  score: number;
  startedAt: number | null;
  finishedAt: number | null;
}

// ---------------------------------------------------------------------------
// Specification (HC-SR04 on a 5 V rail, ESP32 firing ~28 measurements / s)
// ---------------------------------------------------------------------------

export const SPEC = {
  railMin: 4.75,
  railMax: 5.25,
  triggerMinWidthUs: 10,
  echoRateHz: 28.4,
  echoRateTolerance: 0.1,
} as const;

export const SCORE = {
  start: 100,
  test: { wiggle: 10, "rail-load": 10, continuity: 15, "trigger-timing": 10 } as Record<TestID, number>,
  hint: 10,
  wrongCall: 25,
  wrongFix: 15,
} as const;

const wave = (base: number, deltas: number[]) => deltas.map((d) => Number((base + d).toFixed(2)));
const jitter = [0, 0.01, -0.01, 0.02, 0, -0.01, 0.01, 0, 0.01, -0.02, 0, 0.01, 0, -0.01, 0.01, 0];

const healthyPower = { mean_v: 5.01, min_v: 4.99, samples: wave(5.01, jitter) };
const healthyTrig = { rate_hz: 28.4, width_us: 10, samples: wave(28.4, jitter.map((d) => d * 10)) };
const healthyEcho = { rate_hz: 28.4, dropouts_per_min: 0, samples: wave(28.4, jitter.map((d) => d * 20)) };

export const BASELINE: Snapshot = { power: healthyPower, trig: healthyTrig, echo: healthyEcho };

const snap = (patch: Partial<Snapshot>): Snapshot => ({ ...BASELINE, ...patch });

// ---------------------------------------------------------------------------
// Faults, tests, fixes
// ---------------------------------------------------------------------------

export const FAULTS: Record<FaultID, { label: string; cause: string; blurb: string; reveal: string; fix: FixID; symptom: Snapshot }> = {
  "loose-echo": {
    label: "Loose connection",
    cause: "Loose ECHO wire",
    blurb: "A jumper that only makes contact some of the time.",
    reveal: "The ECHO jumper was making intermittent contact.",
    fix: "reseat-echo",
    symptom: snap({ echo: { rate_hz: 19.7, dropouts_per_min: 12, samples: [28, 29, 0, 28, 29, 12, 0, 28, 29, 0, 27, 28, 0, 29, 28, 14] } }),
  },
  "unstable-power": {
    label: "Unstable power",
    cause: "Weak 5 V supply",
    blurb: "A supply that sags when the sensor fires.",
    reveal: "The 5 V rail sagged below spec every time the sensor fired.",
    fix: "stiffen-supply",
    symptom: snap({
      power: { mean_v: 4.81, min_v: 4.52, samples: [4.98, 4.61, 4.95, 4.52, 4.97, 4.66, 4.96, 4.55, 4.98, 4.62, 4.97, 4.58, 4.96, 4.7, 4.98, 4.6] },
      echo: { rate_hz: 21.6, dropouts_per_min: 8, samples: [28, 0, 28, 0, 29, 21, 28, 0, 28, 24, 29, 0, 28, 26, 28, 0] },
    }),
  },
  "missing-echo": {
    label: "Missing signal",
    cause: "ECHO not connected",
    blurb: "A signal that never arrives.",
    reveal: "The ECHO wire was not connected to GPIO18 at all.",
    fix: "reconnect-echo",
    symptom: snap({ echo: { rate_hz: 0, dropouts_per_min: 0, samples: new Array(16).fill(0) } }),
  },
  "timing-drift": {
    label: "Timing problem",
    cause: "TRIG pulse too short",
    blurb: "Firmware that fires the sensor wrong.",
    reveal: "Firmware shortened the TRIG pulse to 2 µs; the HC-SR04 needs at least 10 µs.",
    fix: "restore-trigger",
    symptom: snap({
      trig: { rate_hz: 28.4, width_us: 2, samples: wave(28.4, jitter.map((d) => d * 10)) },
      echo: { rate_hz: 9.1, dropouts_per_min: 19, samples: [0, 9, 0, 0, 11, 0, 8, 0, 0, 10, 0, 9, 0, 0, 12, 0] },
    }),
  },
};

export const FAULT_ORDER: FaultID[] = ["loose-echo", "unstable-power", "missing-echo", "timing-drift"];

export const TESTS: Record<TestID, { label: string; action: string; probe: string; instruction: string }> = {
  wiggle: { label: "Wiggle test", action: "Wiggle the ECHO wire", probe: "P3 ECHO", instruction: "Gently move the ECHO jumper while P3 is watched." },
  "rail-load": { label: "Rail load test", action: "Load the 5 V rail", probe: "P1 POWER", instruction: "Watch the 5 V rail while the sensor fires at full rate." },
  continuity: { label: "Continuity check", action: "Beep out the ECHO wire", probe: "ECHO → GPIO18", instruction: "Power off, then check the ECHO pin reaches GPIO18." },
  "trigger-timing": { label: "Trigger timing check", action: "Measure the TRIG pulse", probe: "P2 TRIG", instruction: "Compare the TRIG pulse width with the HC-SR04 spec (≥ 10 µs)." },
};

export const TEST_ORDER: TestID[] = ["wiggle", "rail-load", "continuity", "trigger-timing"];

export const FIXES: Record<FixID, { label: string; detail: string }> = {
  "reseat-echo": { label: "Replace the ECHO jumper", detail: "Swap in a fresh wire and seat it firmly." },
  "stiffen-supply": { label: "Stiffen the 5 V supply", detail: "Use a supply that can hold 5 V and add a 100 µF bulk capacitor." },
  "reconnect-echo": { label: "Reconnect ECHO to GPIO18", detail: "Wire ECHO back to GPIO18 through the 3.3 V divider." },
  "restore-trigger": { label: "Restore the 10 µs trigger", detail: "Set the TRIG pulse back to 10 µs in firmware and re-flash." },
};

export const FIX_ORDER: FixID[] = ["reseat-echo", "stiffen-supply", "reconnect-echo", "restore-trigger"];

// Which test exposes which fault.
const EXPOSES: Record<TestID, FaultID> = {
  wiggle: "loose-echo",
  "rail-load": "unstable-power",
  continuity: "missing-echo",
  "trigger-timing": "timing-drift",
};

function duringTest(fault: FaultID, test: TestID): { during: Snapshot; finding: string; outcome: TestOutcome } {
  const base = FAULTS[fault].symptom;
  const hit = EXPOSES[test] === fault;
  switch (test) {
    case "wiggle":
      return hit
        ? { outcome: "FOUND", during: { ...base, echo: { rate_hz: 11.2, dropouts_per_min: 27, samples: [28, 0, 14, 0, 0, 27, 0, 10, 0, 0, 21, 0, 0, 26, 0, 9] } },
            finding: "ECHO dropouts jumped from 12 to 27 per minute while the wire moved. POWER and TRIG did not change." }
        : { outcome: "NOT_IT", during: base, finding: "Moving the ECHO wire did not change anything." };
    case "rail-load":
      return hit
        ? { outcome: "FOUND", during: { ...base, power: { mean_v: 4.71, min_v: 4.31, samples: [4.96, 4.4, 4.93, 4.31, 4.95, 4.45, 4.94, 4.36, 4.96, 4.42, 4.95, 4.33, 4.94, 4.5, 4.96, 4.38] } },
            finding: "Under load the rail fell to 4.31 V, below the 4.75 V minimum, and the ECHO gaps lined up with every dip." }
        : { outcome: "NOT_IT", during: base, finding: `The rail held steady above ${base.power.min_v.toFixed(2)} V under load.` };
    case "continuity":
      return hit
        ? { outcome: "FOUND", during: base, finding: "No beep: there is no connection between the ECHO pin and GPIO18." }
        : { outcome: "NOT_IT", during: base, finding: "Beep: ECHO is connected to GPIO18 at rest." };
    case "trigger-timing":
      return hit
        ? { outcome: "FOUND", during: base, finding: "TRIG pulses measure 2 µs. The HC-SR04 needs at least 10 µs, so most pings never fire." }
        : { outcome: "NOT_IT", during: base, finding: `TRIG pulses measure ${base.trig.width_us} µs, which is within spec.` };
  }
}

function afterFix(fault: FaultID, fix: FixID): Snapshot {
  return FAULTS[fault].fix === fix ? BASELINE : FAULTS[fault].symptom;
}

// ---------------------------------------------------------------------------
// Deterministic checks — the same rules for every snapshot.
// ---------------------------------------------------------------------------

export function checks(s: Snapshot): Check[] {
  const railOk = s.power.min_v >= SPEC.railMin && s.power.mean_v <= SPEC.railMax;
  const low = Math.abs(s.echo.rate_hz - SPEC.echoRateHz) > SPEC.echoRateHz * SPEC.echoRateTolerance;
  return [
    { id: "rail", probe: "P1", status: railOk ? "pass" : "fail",
      message: railOk ? `5 V rail stable (min ${s.power.min_v.toFixed(2)} V)` : `5 V rail dipped to ${s.power.min_v.toFixed(2)} V (spec ≥ ${SPEC.railMin} V)` },
    { id: "trigger-width", probe: "P2", status: s.trig.width_us >= SPEC.triggerMinWidthUs ? "pass" : "fail",
      message: s.trig.width_us >= SPEC.triggerMinWidthUs ? `TRIG pulse ${s.trig.width_us} µs (spec ≥ ${SPEC.triggerMinWidthUs} µs)` : `TRIG pulse ${s.trig.width_us} µs is shorter than ${SPEC.triggerMinWidthUs} µs` },
    { id: "echo-activity", probe: "P3", status: s.echo.rate_hz === 0 ? "fail" : low ? "warn" : "pass",
      message: s.echo.rate_hz === 0 ? "No ECHO replies at all" : `ECHO ${s.echo.rate_hz} replies/s (healthy ${SPEC.echoRateHz})` },
    s.echo.rate_hz === 0
      ? { id: "echo-dropouts", probe: "P3", status: "warn", message: "Dropouts can't be measured without ECHO activity" }
      : { id: "echo-dropouts", probe: "P3", status: s.echo.dropouts_per_min > 0 ? "fail" : "pass",
          message: s.echo.dropouts_per_min > 0 ? `${s.echo.dropouts_per_min} ECHO dropouts per minute` : "No ECHO dropouts" },
  ];
}

export const failing = (s: Snapshot) => checks(s).filter((c) => c.status !== "pass");
export const probeChecks = (s: Snapshot, probe: ProbeID) => checks(s).filter((c) => c.probe === probe);

/** VERIFY: every check passes and ECHO is back within tolerance of the baseline. */
export function verifies(s: Snapshot): boolean {
  return failing(s).length === 0 && Math.abs(s.echo.rate_hz - BASELINE.echo.rate_hz) <= BASELINE.echo.rate_hz * SPEC.echoRateTolerance;
}

// ---------------------------------------------------------------------------
// State machine
// ---------------------------------------------------------------------------

export type Action =
  | { type: "start"; now: number }
  | { type: "make-weird"; fault: FaultID | "mystery"; random?: () => number; exclude?: FaultID[] }
  | { type: "inspect"; probe: ProbeID }
  | { type: "run-test"; test: TestID }
  | { type: "hint" }
  | { type: "go-call" }
  | { type: "call"; fault: FaultID }
  | { type: "apply-fix"; fix: FixID; now: number }
  | { type: "back" }
  | { type: "reset" };

export const initialState: DemoState = {
  stage: "intro", fault: null, mystery: false, inspected: [], tests: [], wrongCalls: [], fixes: [], hints: 0, score: SCORE.start, startedAt: null, finishedAt: null,
};

/** Picks a mystery fault, preferring ones not in `exclude` (e.g. already solved). */
export function pickMystery(random: () => number = Math.random, exclude: FaultID[] = []): FaultID {
  const pool = FAULT_ORDER.filter((f) => !exclude.includes(f));
  const choices = pool.length ? pool : FAULT_ORDER;
  return choices[Math.min(choices.length - 1, Math.floor(random() * choices.length))];
}

const last = <T,>(items: T[]): T | undefined => items[items.length - 1];
const spend = (score: number, cost: number) => Math.max(0, score - cost);

export function current(state: DemoState): Snapshot {
  if (!state.fault) return BASELINE;
  const lastFix = last(state.fixes);
  return lastFix ? lastFix.after : FAULTS[state.fault].symptom;
}

export const found = (state: DemoState) => state.tests.some((t) => t.outcome === "FOUND");
export const resolved = (state: DemoState) => last(state.fixes)?.resolved === true;
/** Called correctly: the stage moved past "call". */
export const solved = (state: DemoState) => state.stage === "fix" || state.stage === "result";
/** Mystery faults stay hidden until the player names the cause correctly. */
export const revealed = (state: DemoState) => !state.mystery || solved(state);

export function reduce(state: DemoState, action: Action): DemoState {
  switch (action.type) {
    case "start":
      return { ...initialState, stage: "choose", startedAt: action.now };
    case "make-weird": {
      if (state.stage !== "choose") return state;
      const mystery = action.fault === "mystery";
      const fault = mystery ? pickMystery(action.random, action.exclude) : (action.fault as FaultID);
      return { ...initialState, stage: "investigate", fault, mystery, startedAt: state.startedAt };
    }
    case "inspect":
      if (state.stage !== "investigate" || state.inspected.includes(action.probe)) return state;
      return { ...state, inspected: [...state.inspected, action.probe] };
    case "run-test": {
      if (state.stage !== "investigate" || !state.fault) return state;
      if (state.tests.some((t) => t.test === action.test)) return state;
      const before = current(state);
      const { during, finding, outcome } = duringTest(state.fault, action.test);
      return { ...state, score: spend(state.score, SCORE.test[action.test]), tests: [...state.tests, { test: action.test, outcome, before, during, finding }] };
    }
    case "hint":
      return state.stage === "investigate" && state.hints < 2 ? { ...state, hints: state.hints + 1, score: spend(state.score, SCORE.hint) } : state;
    case "go-call":
      return state.stage === "investigate" ? { ...state, stage: "call" } : state;
    case "call": {
      if (state.stage !== "call" || !state.fault || state.wrongCalls.includes(action.fault)) return state;
      if (action.fault === state.fault) return { ...state, stage: "fix" };
      return { ...state, wrongCalls: [...state.wrongCalls, action.fault], score: spend(state.score, SCORE.wrongCall) };
    }
    case "apply-fix": {
      if (state.stage !== "fix" || !state.fault || state.fixes.some((f) => f.fix === action.fix)) return state;
      const after = afterFix(state.fault, action.fix);
      const ok = verifies(after);
      const fixes = [...state.fixes, { fix: action.fix, after, resolved: ok }];
      return ok
        ? { ...state, fixes, stage: "result", finishedAt: action.now }
        : { ...state, fixes, score: spend(state.score, SCORE.wrongFix) };
    }
    case "back":
      switch (state.stage) {
        case "choose": return initialState;
        case "investigate": return { ...initialState, stage: "choose", startedAt: state.startedAt };
        case "call": return { ...state, stage: "investigate" };
        default: return state;
      }
    case "reset":
      return initialState;
  }
}

export const canGoBack = (state: DemoState) => state.stage === "choose" || state.stage === "investigate" || state.stage === "call";

export function rank(score: number): { title: string; stars: 1 | 2 | 3 } {
  if (score >= 85) return { title: "Hardware whisperer", stars: 3 };
  if (score >= 60) return { title: "Bench pro", stars: 2 };
  return { title: "Apprentice debugger", stars: 1 };
}

export const testHeadline = (run: TestRun) => (run.outcome === "FOUND" ? "WE FOUND SOMETHING." : "THAT WASN’T IT.");

/**
 * Hints come only from what the fault capture shows. Level 1 points at the
 * suspicious probe; level 2 names the kind of test that would examine it.
 */
export function hint(state: DemoState, level: 1 | 2): string | null {
  if (!state.fault) return null;
  const s = FAULTS[state.fault].symptom;
  const failed = new Set(failing(s).map((c) => c.id));
  if (failed.has("rail")) return level === 1
    ? "P1 POWER is failing too. The ECHO trouble might not start at ECHO."
    : "Try a test that stresses the power rail while the sensor is busy.";
  if (failed.has("trigger-width")) return level === 1
    ? "Look closely at P2: how wide are those TRIG pulses?"
    : "Try a test that measures the TRIG pulse against the sensor’s spec.";
  if (s.echo.rate_hz === 0) return level === 1
    ? "P3 ECHO is completely flat while TRIG keeps firing."
    : "Try a test that checks whether ECHO is physically connected at all.";
  return level === 1
    ? "ECHO comes and goes while POWER and TRIG look perfect."
    : "Try a test that disturbs the ECHO wire while P3 is watched.";
}

/** A machine-readable record of the run. Always marked simulated. */
export function incidentRecord(state: DemoState) {
  if (!state.fault) return null;
  const fault = FAULTS[state.fault];
  return {
    simulated: true,
    source: "BROWSER SIMULATOR",
    device: "Ultrasonic Distance Sensor · ESP32 + HC-SR04",
    fault: { id: state.fault, label: fault.label, cause: fault.reveal, chosen_as_mystery: state.mystery },
    symptom_checks: checks(fault.symptom),
    probes_inspected: state.inspected,
    tests: state.tests.map((t) => ({ test: t.test, outcome: t.outcome, finding: t.finding })),
    wrong_calls: state.wrongCalls,
    hints_used: state.hints,
    fix_attempts: state.fixes.map((f) => ({ fix: f.fix, resolved: f.resolved })),
    verification: resolved(state) ? "RESOLVED" : "UNRESOLVED",
    score: state.score,
    rank: rank(state.score).title,
    started_at: state.startedAt ? new Date(state.startedAt).toISOString() : null,
    finished_at: state.finishedAt ? new Date(state.finishedAt).toISOString() : null,
  };
}

// ---------------------------------------------------------------------------
// Oscilloscope waveforms (pure; the UI animates by shifting the phase).
// Each fault leaves a visible fingerprint an observant player can read.
// ---------------------------------------------------------------------------

/** 0..1 values for one channel over `n` points, starting at `phase` (0..1). */
export function waveform(s: Snapshot, probe: ProbeID, n: number, phase = 0): number[] {
  const out: number[] = [];
  const period = 16; // buckets per screen
  for (let i = 0; i < n; i++) {
    const t = (((i / n + phase) % 1) + 1) % 1; // tolerate any phase value
    const bucket = Math.floor(t * period) % period;
    const within = (t * period) % 1;
    if (probe === "P1") {
      // Rail: flat near the top; dips wherever the sample sags below 4.9 V.
      const v = s.power.samples[bucket];
      const dip = Math.max(0, 5.0 - v) / 0.8;
      const shape = within > 0.35 && within < 0.75 ? dip : dip * 0.25;
      out.push(0.82 - Math.min(0.6, shape));
    } else if (probe === "P2") {
      // TRIG: one pulse per bucket; width scales with the measured µs.
      const width = Math.min(0.5, s.trig.width_us / 30);
      out.push(within < width ? 0.85 : 0.12);
    } else {
      // ECHO: a return pulse per bucket, missing when that bucket dropped out.
      const alive = s.echo.samples[bucket] > 0;
      out.push(alive && within > 0.4 && within < 0.8 ? 0.85 : 0.12);
    }
  }
  return out;
}
