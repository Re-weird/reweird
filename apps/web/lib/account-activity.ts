"use client";

import { useEffect, useState } from "react";
import type { GitHubStatus, HistorySummary, Project } from "@reweird/shared-types";
import { githubApi, historyApi, projectApi } from "./api";

// Everything the dashboard and profile rail show, from real API data only.
// Sessions are limited to the caller's own projects (the projects list is
// already owner-scoped), so demo sessions don't inflate the numbers.
export interface AccountActivity {
  projects: Project[];
  sessions: HistorySummary[];
  github: GitHubStatus | null;
}

export type AccountActivityState =
  | { status: "loading" }
  | { status: "error"; message: string }
  | { status: "ready"; data: AccountActivity };

async function loadActivity(): Promise<AccountActivity> {
  const [projects, github] = await Promise.all([projectApi.listProjects(), githubApi.status().catch(() => null)]);
  // One query per project: a single unfiltered query returns the newest 100
  // sessions across every project, which demo sessions could crowd out.
  const histories = await Promise.all(projects.map((project) => historyApi.list({ projectID: project.id })));
  const sessions = histories.flatMap((history) => history.items).sort((a, b) => b.started_at_ms - a.started_at_ms);
  return { projects, sessions, github };
}

// The rail and the dashboard mount together; share one request between them
// and reuse it briefly so navigating back and forth doesn't refetch.
let cached: { at: number; promise: Promise<AccountActivity> } | null = null;

const CHANGED = "reweird:account-activity-changed";

/** Call after anything the dashboard or rail summarizes changes (GitHub connection, projects). */
export function refreshAccountActivity() {
  cached = null;
  if (typeof window !== "undefined") window.dispatchEvent(new Event(CHANGED));
}

export function useAccountActivity(): AccountActivityState & { reload: () => void } {
  const [state, setState] = useState<AccountActivityState>({ status: "loading" });
  const [attempt, setAttempt] = useState(0);
  useEffect(() => {
    let live = true;
    if (!cached || Date.now() - cached.at > 15_000 || attempt > 0) cached = { at: Date.now(), promise: loadActivity() };
    const promise = cached.promise;
    promise
      .then((data) => { if (live) setState({ status: "ready", data }); })
      .catch((cause) => {
        if (cached?.promise === promise) cached = null;
        if (live) setState({ status: "error", message: cause instanceof Error ? cause.message : "Couldn't load your activity." });
      });
    return () => { live = false; };
  }, [attempt]);
  useEffect(() => {
    const onChange = () => setAttempt((count) => count + 1);
    window.addEventListener(CHANGED, onChange);
    return () => window.removeEventListener(CHANGED, onChange);
  }, []);
  return { ...state, reload: () => { setState({ status: "loading" }); setAttempt((count) => count + 1); } };
}

/** Median time from start to end for resolved sessions, or null when there are none. */
export function medianResolveMS(sessions: HistorySummary[]): number | null {
  const durations = sessions
    .filter((item) => item.status === "RESOLVED" && item.ended_at_ms && item.ended_at_ms > item.started_at_ms)
    .map((item) => item.ended_at_ms! - item.started_at_ms)
    .sort((a, b) => a - b);
  if (!durations.length) return null;
  const middle = Math.floor(durations.length / 2);
  return durations.length % 2 ? durations[middle] : (durations[middle - 1] + durations[middle]) / 2;
}

export function formatDuration(ms: number) {
  const minutes = Math.round(ms / 60_000);
  if (minutes < 60) return `${Math.max(minutes, 1)}m`;
  const hours = minutes / 60;
  return hours < 48 ? `${hours.toFixed(hours < 10 ? 1 : 0)}h` : `${Math.round(hours / 24)}d`;
}

export type PipelineStage = "needs-code" | "needs-review" | "needs-probes" | "ready";

/** Where a project is in setup, from the same signals the Workbench checklist uses. */
export function pipelineStage(project: Project): PipelineStage {
  if (project.analysis_status === "PENDING" || project.analysis_status === "PROCESSING" || project.analysis_status === "FAILED") return "needs-code";
  if (project.analysis_status === "DRAFT_READY") return "needs-review";
  if (!project.probe_plan?.connected) return "needs-probes";
  return "ready";
}
