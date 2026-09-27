// Practice-only lifecycle model. It cannot import/call a hardware API and never
// produces a MeasurementWindow or a Known Good record.
export const patchPracticeSteps = ["PROPOSED", "VALIDATED", "AWAITING_APPROVAL", "ARMED", "ACTIVE", "DISABLED", "RE_MEASURE", "VERIFY"] as const;
export function patchPracticeNext(state: string, approved: boolean): string {
  const index = patchPracticeSteps.indexOf(state as typeof patchPracticeSteps[number]);
  if (index < 0 || state === "VERIFY" || (state === "AWAITING_APPROVAL" && !approved)) return state;
  return patchPracticeSteps[index + 1];
}
