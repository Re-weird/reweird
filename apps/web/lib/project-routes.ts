export type ProjectTabID = "workbench" | "overview" | "probe-setup" | "health" | "simulator" | "diagnosis" | "next-test" | "verify" | "history" | "reports" | "computer";

export const projectTabs: { id: ProjectTabID; label: string; segment: string }[] = [
  { id: "workbench", label: "Workbench", segment: "" },
  { id: "overview", label: "Overview", segment: "overview" },
  { id: "probe-setup", label: "Probe setup", segment: "probe-setup" },
  { id: "health", label: "Device health", segment: "health" },
  { id: "simulator", label: "Simulator", segment: "simulator" },
  { id: "diagnosis", label: "Diagnosis", segment: "diagnosis" },
  { id: "next-test", label: "Next test", segment: "next-test" },
  { id: "verify", label: "Verify result", segment: "verify" },
  { id: "history", label: "History", segment: "history" },
  { id: "reports", label: "Reports", segment: "reports" },
  { id: "computer", label: "Computer checks", segment: "computer" },
];

export function projectPath(id: string, tab: ProjectTabID = "workbench"): string {
  const segment = projectTabs.find((t) => t.id === tab)?.segment ?? "";
  return segment ? `/projects/${id}/${segment}` : `/projects/${id}`;
}

// Placeholder project used for the built-in browser/simulator demo when no
// real project has been created yet - keeps demo mode reachable at a real URL
// (/projects/demo/...) instead of only living in in-memory state.
export const DEMO_PROJECT_ID = "demo";

// Demo-mode guided tests are recorded under the demo profile's project id.
export const DEMO_HISTORY_PROJECT_ID = "ultrasonic-demo";

// Components merged from main (circuit map) navigate with the
// old single-page view names; map them onto this app's project tabs. "live"
// lands on Workbench, which now hosts the live signal view.
export type LegacyView = "profile" | "connect" | "live" | "diagnosis" | "guided" | "history";
const legacyTabs: Record<LegacyView, ProjectTabID> = {
  profile: "overview",
  connect: "probe-setup",
  live: "workbench",
  diagnosis: "diagnosis",
  guided: "next-test",
  history: "history",
};
export function legacyViewPath(projectID: string, view: LegacyView): string {
  return projectPath(projectID, legacyTabs[view]);
}
