export type ProjectTabID = "workbench" | "overview" | "probe-setup" | "simulator" | "diagnosis" | "next-test" | "verify" | "history" | "reports" | "computer";

export const projectTabs: { id: ProjectTabID; label: string; segment: string }[] = [
  { id: "workbench", label: "Workbench", segment: "" },
  { id: "overview", label: "Overview", segment: "overview" },
  { id: "probe-setup", label: "Probe setup", segment: "probe-setup" },
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
