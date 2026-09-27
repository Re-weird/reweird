// Software-only judge demo engine.
//
// Everything here runs in the browser with no hardware, no Go API, and no
// sign-in. Every reading is generated from fixed, deterministic tables below
// and is always labelled SIMULATED in the UI. The same deterministic checks are
// applied to every snapshot, so a judge sees evidence change for a reason, not
// because a script jumped to the next slide.

export type FaultID = "loose-echo" | "unstable-power" | "missing-echo" | "timing-drift";
export type TestID = "wiggle" | "rail-load" | "continuity" | "trigger-timing";
export type FixID = "reseat-echo" | "stiffen-supply" | "reconnect-echo" | "restore-trigger";
export type Stage = "intro" | "choose" | "investigate" | "fix" | "verify" | "passport";
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
  probe: "P1" | "P2" | "P3";
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
  tests: TestRun[];
  fixes: FixAttempt[];
  hints: number;
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

export const FAULTS: Record<FaultID, { label: string; blurb: string; reveal: string; fix: FixID; symptom: Snapshot }> = {
  "loose-echo": {
    label: "Loose connection",
    blurb: "A jumper that only makes contact some of the time.",
    reveal: "The ECHO jumper was making intermittent contact.",
    fix: "reseat-echo",
    symptom: snap({ echo: { rate_hz: 19.7, dropouts_per_min: 12, samples: [28, 29, 0, 28, 29, 12, 0, 28, 29, 0, 27, 28, 0, 29, 28, 14] } }),
  },
  "unstable-power": {
    label: "Unstable power",
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
    blurb: "A signal that never arrives.",
    reveal: "The ECHO wire was not connected to GPIO18 at all.",
    fix: "reconnect-echo",
    symptom: snap({ echo: { rate_hz: 0, dropouts_per_min: 0, samples: new Array(16).fill(0) } }),
  },
  "timing-drift": {
    label: "Timing problem",
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

export const TESTS: Record<TestID, { label: string; probe: string; instruction: string }> = {
  wiggle: { label: "Wiggle test", probe: "P3 ECHO", instruction: "Gently move the ECHO jumper while P3 is watched." },
  "rail-load": { label: "Rail load test", probe: "P1 POWER", instruction: "Watch the 5 V rail while the sensor fires at full rate." },
  continuity: { label: "Continuity check", probe: "ECHO → GPIO18", instruction: "Power off, then check the ECHO pin reaches GPIO18." },
  "trigger-timing": { label: "Trigger timing check", probe: "P2 TRIG", instruction: "Compare the TRIG pulse width with the HC-SR04 spec (≥ 10 µs)." },
};

export const TEST_ORDER: TestID[] = ["wiggle", "rail-load", "continuity", "trigger-timing"];

export const FIXES: Record<FixID, { label: string; detail: string }> = {
  "reseat-echo": { label: "Replace the ECHO jumper", detail: "Swap in a fresh wire and seat it firmly." },
  "stiffen-supply": { label: "Stiffen the 5 V supply", detail: "Use a supply that can hold 5 V and add a 100 µF bulk capacitor." },
  "reconnect-echo": { label: "Reconnect ECHO to GPIO18", detail: "Wire ECHO back to GPIO18 through the 3.3 V divider." },
  "restore-trigger": { label: "Restore the 10 µs trigger", detail: "Set the TRIG pulse back to 10 µs in firmware and re-flash." },
};

export const FIX_ORDER: FixID[] = ["reseat-echo", "stiffen-supply", "reconnect-echo", "restore-trigger"];

// Which test exposes which fault, and what each test shows otherwise.
const EXPOSES: Record<TestID, FaultID> = {
  wiggle: "loose-echo",
  "rail-load": "unstable-power",
  continuity: "missing-echo",
  "trigger-timing": "timing-drift",
};

function duringTest(fault: FaultID, test: TestID): { during: Snapshot; finding: string; outcome: TestOutcome } {
  const base = FAULTS[fault].symptom;
  const found = EXPOSES[test] === fault;
  switch (test) {
    case "wiggle":
      return found
        ? { outcome: "FOUND", during: { ...base, echo: { rate_hz: 11.2, dropouts_per_min: 27, samples: [28, 0, 14, 0, 0, 27, 0, 10, 0, 0, 21, 0, 0, 26, 0, 9] } },
            finding: "ECHO dropouts jumped from 12 to 27 per minute while the jumper moved. POWER and TRIG did not change." }
        : { outcome: "NOT_IT", during: base, finding: "Moving the ECHO jumper did not change the dropout rate." };
    case "rail-load":
      return found
        ? { outcome: "FOUND", during: { ...base, power: { mean_v: 4.71, min_v: 4.31, samples: [4.96, 4.4, 4.93, 4.31, 4.95, 4.45, 4.94, 4.36, 4.96, 4.42, 4.95, 4.33, 4.94, 4.5, 4.96, 4.38] } },
            finding: "At full firing rate the rail fell to 4.31 V, below the 4.75 V minimum, and ECHO dropouts lined up with each dip." }
        : { outcome: "NOT_IT", during: base, finding: `The rail held between ${base.power.min_v.toFixed(2)} V and 5.02 V under load.` };
    case "continuity":
      return found
        ? { outcome: "FOUND", during: base, finding: "Open circuit: no continuity between the HC-SR04 ECHO pin and GPIO18." }
        : { outcome: "NOT_IT", during: base, finding: "ECHO has continuity to GPIO18 at rest. A static check cannot rule out an intermittent contact." };
    case "trigger-timing":
      return found
        ? { outcome: "FOUND", during: base, finding: "TRIG pulses measure 2 µs wide. The HC-SR04 needs at least 10 µs, so most pings never fire." }
        : { outcome: "NOT_IT", during: base, finding: `TRIG pulses measure ${base.trig.width_us} µs wide at ${base.trig.rate_hz} Hz, within spec.` };
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
      message: s.echo.rate_hz === 0 ? "No ECHO activity in the capture window" : `ECHO ${s.echo.rate_hz} pulses/s (baseline ${SPEC.echoRateHz})` },
    s.echo.rate_hz === 0
      ? { id: "echo-dropouts", probe: "P3", status: "warn", message: "Dropouts not evaluable without ECHO activity" }
      : { id: "echo-dropouts", probe: "P3", status: s.echo.dropouts_per_min > 0 ? "fail" : "pass",
          message: s.echo.dropouts_per_min > 0 ? `${s.echo.dropouts_per_min} unexpected ECHO dropouts per minute` : "No ECHO dropouts" },
  ];
}

export const failing = (s: Snapshot) => checks(s).filter((c) => c.status !== "pass");

/** VERIFY: every check passes and ECHO is back within tolerance of the baseline. */
export function verifies(s: Snapshot): boolean {
  return failing(s).length === 0 && Math.abs(s.echo.rate_hz - BASELINE.echo.rate_hz) <= BASELINE.echo.rate_hz * SPEC.echoRateTolerance;
}

// ---------------------------------------------------------------------------
// State machine
// ---------------------------------------------------------------------------

export type Action =
  | { type: "start"; now: number }
  | { type: "make-weird"; fault: FaultID | "mystery"; random?: () => number }
  | { type: "run-test"; test: TestID }
  | { type: "hint" }
  | { type: "go-fix" }
  | { type: "apply-fix"; fix: FixID }
  | { type: "retry" }
  | { type: "back" }
  | { type: "passport"; now: number }
  | { type: "reset" };

export const initialState: DemoState = { stage: "intro", fault: null, mystery: false, tests: [], fixes: [], hints: 0, startedAt: null, finishedAt: null };

export function pickMystery(random: () => number = Math.random): FaultID {
  return FAULT_ORDER[Math.min(FAULT_ORDER.length - 1, Math.floor(random() * FAULT_ORDER.length))];
}

const last = <T,>(items: T[]): T | undefined => items[items.length - 1];

export function current(state: DemoState): Snapshot {
  if (!state.fault) return BASELINE;
  const lastFix = last(state.fixes);
  return lastFix ? lastFix.after : FAULTS[state.fault].symptom;
}

export const found = (state: DemoState) => state.tests.some((t) => t.outcome === "FOUND");
export const resolved = (state: DemoState) => last(state.fixes)?.resolved === true;
/** Mystery faults stay hidden until a test finds evidence for them. */
export const revealed = (state: DemoState) => !state.mystery || found(state);

export function reduce(state: DemoState, action: Action): DemoState {
  switch (action.type) {
    case "start":
      return { ...initialState, stage: "choose", startedAt: action.now };
    case "make-weird": {
      if (state.stage !== "choose") return state;
      const mystery = action.fault === "mystery";
      const fault = mystery ? pickMystery(action.random) : (action.fault as FaultID);
      return { ...state, stage: "investigate", fault, mystery, tests: [], fixes: [], hints: 0 };
    }
    case "run-test": {
      if (state.stage !== "investigate" || !state.fault) return state;
      if (state.tests.some((t) => t.test === action.test)) return state;
      const before = current(state);
      const { during, finding, outcome } = duringTest(state.fault, action.test);
      return { ...state, tests: [...state.tests, { test: action.test, outcome, before, during, finding }] };
    }
    case "hint":
      return state.stage === "investigate" && !found(state) ? { ...state, hints: Math.min(state.hints + 1, 2) } : state;
    case "go-fix":
      return state.stage === "investigate" && found(state) ? { ...state, stage: "fix" } : state;
    case "apply-fix": {
      if (state.stage !== "fix" || !state.fault) return state;
      const after = afterFix(state.fault, action.fix);
      return { ...state, stage: "verify", fixes: [...state.fixes, { fix: action.fix, after, resolved: verifies(after) }] };
    }
    case "retry":
      return state.stage === "verify" && !resolved(state) ? { ...state, stage: "fix" } : state;
    case "passport":
      return state.stage === "verify" && resolved(state) ? { ...state, stage: "passport", finishedAt: action.now } : state;
    case "back":
      switch (state.stage) {
        case "choose": return initialState;
        case "investigate": return { ...state, stage: "choose", fault: null, mystery: false, tests: [], fixes: [], hints: 0 };
        case "fix": return { ...state, stage: "investigate" };
        case "verify": return resolved(state) ? state : { ...state, stage: "fix" };
        default: return state;
      }
    case "reset":
      return initialState;
  }
}

export function verifyHeadline(state: DemoState): "NOT WEIRD ANYMORE." | "STILL WEIRD." | null {
  if (!state.fixes.length) return null;
  return resolved(state) ? "NOT WEIRD ANYMORE." : "STILL WEIRD.";
}

export const testHeadline = (run: TestRun) => (run.outcome === "FOUND" ? "WE FOUND SOMETHING." : "THAT WASN’T IT.");

/** A machine-readable record of the run. Always marked simulated. */
export function incidentRecord(state: DemoState) {
  if (!state.fault) return null;
  const fault = FAULTS[state.fault];
  return {
    simulated: true,
    source: "BROWSER SIMULATOR",
    device: "Ultrasonic Distance Sensor · ESP32 + HC-SR04",
    fault: { id: state.fault, label: fault.label, chosen_as_mystery: state.mystery, cause: fault.reveal },
    symptom_checks: checks(fault.symptom),
    tests: state.tests.map((t) => ({ test: t.test, outcome: t.outcome, finding: t.finding })),
    hints_used: state.hints,
    fix_attempts: state.fixes.map((f) => ({ fix: f.fix, resolved: f.resolved })),
    verification: resolved(state) ? "RESOLVED" : "UNRESOLVED",
    started_at: state.startedAt ? new Date(state.startedAt).toISOString() : null,
    finished_at: state.finishedAt ? new Date(state.finishedAt).toISOString() : null,
  };
}

/** The capture currently on screen while investigating: the latest test window, or the fault capture. */
export function latestCapture(state: DemoState): { label: string; snapshot: Snapshot } {
  const run = last(state.tests);
  if (run) return { label: `During the ${TESTS[run.test].label.toLowerCase()}`, snapshot: run.during };
  return { label: "First capture", snapshot: current(state) };
}

/**
 * Hints come only from what the fault capture shows, never from the hidden
 * answer directly. Level 1 points at the suspicious probe; level 2 names the
 * kind of test that would examine it.
 */
export function hint(state: DemoState, level: 1 | 2): string | null {
  if (!state.fault) return null;
  const s = FAULTS[state.fault].symptom;
  const failed = new Set(failing(s).map((c) => c.id));
  if (failed.has("rail")) return level === 1
    ? "P1 POWER failed its check. The ECHO trouble might not start at ECHO."
    : "Try a test that watches the power rail while the sensor is busy.";
  if (failed.has("trigger-width")) return level === 1
    ? "P2 TRIG failed its check. Look at how the sensor is being fired."
    : "Try a test that measures the TRIG pulse against the sensor’s spec.";
  if (s.echo.rate_hz === 0) return level === 1
    ? "P3 ECHO is completely silent while TRIG keeps firing."
    : "Try a test that checks whether ECHO is physically connected at all.";
  return level === 1
    ? "ECHO comes and goes while POWER and TRIG look perfect."
    : "Try a test that disturbs the ECHO wire while P3 is watched.";
}

/** Whether the header Back button has somewhere sensible to go. */
export const canGoBack = (state: DemoState) =>
  state.stage === "choose" || state.stage === "investigate" || state.stage === "fix" || (state.stage === "verify" && !resolved(state));
