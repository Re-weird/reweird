import type { CalibrationProbe, CalibrationState, CalibrationStatus, ExpectedSignal, ObservedSummary, TrustedBaseline } from "@reweird/shared-types";

// Presentation helpers for physical calibration. They only format what the
// API returned: EXPECTED comes from the confirmed profile, OBSERVED from
// stored REAL SERIAL windows, KNOWN GOOD from a user-confirmed baseline.

export type StepState = "done" | "current" | "todo" | "blocked";
export interface CalibrationStep { id: string; label: string; state: StepState }

const stepIDs = ["not-calibrated", "observe", "review", "confirm", "calibrated"] as const;
const stepLabels: Record<(typeof stepIDs)[number], string> = {
  "not-calibrated": "Not calibrated",
  observe: "Observe hardware",
  review: "Review expected signals",
  confirm: "You confirm it is healthy",
  calibrated: "Calibrated",
};

const currentStep: Record<CalibrationStatus, number> = {
  NOT_CALIBRATED: 0,
  SIMULATED_SOURCE: 0,
  BASELINE_INCOMPATIBLE: 0,
  OBSERVING: 1,
  ATTENTION_REQUIRED: 2,
  REVIEW_REQUIRED: 3,
  CALIBRATED: 4,
};

export function calibrationSteps(status: CalibrationStatus): CalibrationStep[] {
  const current = currentStep[status];
  return stepIDs.map((id, index) => ({
    id,
    label: stepLabels[id],
    state: status === "CALIBRATED" ? "done"
      : index < current ? "done"
      : index === current ? (status === "ATTENTION_REQUIRED" || status === "SIMULATED_SOURCE" || status === "BASELINE_INCOMPATIBLE" ? "blocked" : "current")
      : "todo",
  }));
}

export const calibrationStatusLabel: Record<CalibrationStatus, string> = {
  NOT_CALIBRATED: "NOT CALIBRATED",
  OBSERVING: "OBSERVING",
  ATTENTION_REQUIRED: "ATTENTION REQUIRED",
  REVIEW_REQUIRED: "READY FOR YOUR CONFIRMATION",
  CALIBRATED: "CALIBRATED",
  BASELINE_INCOMPATIBLE: "BASELINE INCOMPATIBLE",
  SIMULATED_SOURCE: "SIMULATED SOURCE",
};

/** Save is possible only when the API offers it AND the user ticked the box. */
export function canSaveKnownGood(state: CalibrationState | null, userConfirmedHealthy: boolean): boolean {
  return Boolean(state && state.can_save_known_good && state.status === "REVIEW_REQUIRED" && state.provenance === "REAL_SERIAL" && state.candidate_measurement_id && userConfirmedHealthy);
}

export function formatVoltage(value: number): string {
  return `${value.toFixed(2)} V`;
}

export function formatFrequency(value: number): string {
  if (value >= 1000) return `${(value / 1000).toFixed(value >= 10_000 ? 1 : 2)} kHz`;
  if (value >= 100) return `${value.toFixed(0)} Hz`;
  if (value >= 10) return `${value.toFixed(1)} Hz`;
  return `${value.toFixed(2)} Hz`;
}

export function formatDuration(microseconds: number): string {
  if (microseconds >= 1000) return `${(microseconds / 1000).toFixed(microseconds >= 10_000 ? 1 : 2)} ms`;
  if (microseconds >= 100) return `${microseconds.toFixed(0)} µs`;
  return `${microseconds.toFixed(1)} µs`;
}

function range(low: number | undefined, high: number | undefined, format: (value: number) => string): string | null {
  if (low !== undefined && high !== undefined) return format(low) === format(high) ? format(low) : `${format(low)}–${format(high)}`;
  if (low !== undefined) return `≥ ${format(low)}`;
  if (high !== undefined) return `≤ ${format(high)}`;
  return null;
}

export function expectedText(role: string, expected: ExpectedSignal): string {
  if (role.toUpperCase() === "UNASSIGNED") return "Unassigned — no expectation";
  const parts = [
    range(expected.min_voltage, expected.max_voltage, formatVoltage) ?? (expected.nominal_voltage !== undefined ? `≈ ${formatVoltage(expected.nominal_voltage)}` : null),
    range(expected.min_frequency_hz, expected.max_frequency_hz, formatFrequency) ?? (expected.nominal_frequency_hz !== undefined ? `≈ ${formatFrequency(expected.nominal_frequency_hz)}` : null),
    (() => { const width = range(expected.min_pulse_width_us, expected.max_pulse_width_us, formatDuration); return width ? `${width} HIGH` : null; })(),
  ].filter(Boolean);
  const kind = expected.signal_type || "signal";
  if (!parts.length) return expected.required ? `${kind} · activity required` : `${kind} · observed only`;
  return `${parts.join(" · ")}${expected.required ? "" : " · optional"}`;
}

export function observedText(observed: ObservedSummary): string {
  if (!observed.windows) return "No physical window yet";
  if (observed.unreliable_windows > 0) return `Capture unreliable in ${observed.unreliable_windows}/${observed.windows} windows`;
  const parts = [
    range(observed.min_voltage, observed.max_voltage, formatVoltage),
    range(observed.min_frequency_hz, observed.max_frequency_hz, formatFrequency),
    (() => { const width = range(observed.min_pulse_width_us, observed.max_pulse_width_us, formatDuration); return width ? `${width} HIGH` : null; })(),
  ].filter(Boolean);
  if (!parts.length) return observed.active_windows ? `Active in ${observed.active_windows}/${observed.windows} windows` : `No activity in ${observed.windows} windows`;
  return `${parts.join(" · ")} · ${observed.stable_windows}/${observed.windows} stable`;
}

export function knownGoodText(knownGood: TrustedBaseline | undefined): string {
  if (!knownGood) return "Not calibrated";
  const parts = [
    range(knownGood.min_voltage, knownGood.max_voltage, formatVoltage),
    range(knownGood.min_frequency_hz, knownGood.max_frequency_hz, formatFrequency),
    knownGood.compare_pulse_width ? (() => { const width = range(knownGood.min_pulse_width_us, knownGood.max_pulse_width_us, formatDuration); return width ? `${width} HIGH` : null; })() : null,
  ].filter(Boolean);
  const windows = knownGood.window_count ? ` · ${knownGood.window_count} windows` : "";
  return parts.length ? `${parts.join(" · ")}${windows}` : `Confirmed healthy${windows}`;
}

export function probeTone(probe: CalibrationProbe): "ok" | "attention" | "idle" {
  if (probe.issues?.length) return "attention";
  if (probe.role.toUpperCase() === "UNASSIGNED" || !probe.observed.windows) return "idle";
  return "ok";
}

/** Label for the Diagnosis "Known Good" card instead of a bare UNKNOWN. */
export function baselineStatusLabel(status: unknown, learnedWindows: unknown): string {
  switch (status) {
    case "USER_CONFIRMED_HEALTHY":
      return typeof learnedWindows === "number" && learnedWindows > 0 ? `Known Good · ${learnedWindows} confirmed windows` : "Known Good · user confirmed";
    case "KNOWN_GOOD_CAPTURE":
      return "Known Good capture";
    case "MANUFACTURER_SPEC":
      return "Manufacturer specification";
    default:
      return "Not calibrated yet";
  }
}
