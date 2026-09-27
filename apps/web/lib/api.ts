import type {
  AnalyzeProjectResponse,
  ConfirmProfileResponse,
  DemoSession,
  DiagnosticWorkflow,
  DetailedReport,
  DevicePassport,
  HistoryDetail,
  HistoryStatus,
  HistorySummary,
  KnownGoodBaseline,
  MeasurementWindow,
  PhysicalCommit,
  ProbePlan,
  Project,
  ProjectProfile,
  SimulatorScenarioList,
  TestRecommendation,
} from "@reweird/shared-types";
import { getAuthToken } from "./auth-token";

const API_URL = process.env.NEXT_PUBLIC_API_URL ?? "";

export class ApiError extends Error {
  constructor(
    message: string,
    public readonly code: string,
    public readonly status: number,
  ) {
    super(message);
    this.name = "ApiError";
  }
}

async function requestJSON<T>(path: string, init?: RequestInit, timeoutMS = 8_000): Promise<T> {
  const controller = new AbortController();
  const timer = setTimeout(() => controller.abort(), timeoutMS);
  try {
    const headers = new Headers(init?.headers);
    if (!(init?.body instanceof FormData) && init?.body && !headers.has("Content-Type")) {
      headers.set("Content-Type", "application/json");
    }
    const token = await getAuthToken();
    if (token) headers.set("Authorization", `Bearer ${token}`);
    const response = await fetch(`${API_URL}${path}`, {
      ...init,
      headers,
      signal: controller.signal,
      cache: "no-store",
    });
    const payload = (await response.json().catch(() => ({}))) as { error?: string; detail?: string } & T;
    if (!response.ok) {
      throw new ApiError(payload.detail ?? "The ReWeird API rejected the request.", payload.error ?? "API_ERROR", response.status);
    }
    return payload as T;
  } catch (error) {
    if (error instanceof ApiError) throw error;
    if (error instanceof DOMException && error.name === "AbortError") {
      throw new ApiError("The ReWeird API did not respond in time.", "API_TIMEOUT", 0);
    }
    throw new ApiError("The ReWeird API is unavailable. Start the Go backend and try again.", "API_UNAVAILABLE", 0);
  } finally {
    clearTimeout(timer);
  }
}

async function demoRequest(path: string, init?: RequestInit): Promise<DemoSession | null> {
  try {
    return await requestJSON<DemoSession>(path, init, 5_000);
  } catch {
    return null;
  }
}

export const demoApi = {
  load: () => demoRequest("/api/v1/session"),
  wiggle: () => demoRequest("/api/v1/demo/wiggle", { method: "POST" }),
  repair: () => demoRequest("/api/v1/demo/repair", { method: "POST" }),
  reset: () => demoRequest("/api/v1/demo/reset", { method: "POST" }),
  profile: () => requestJSON<ProjectProfile>("/api/v1/profiles/ultrasonic-demo"),
  probePlan: () => requestJSON<ProbePlan>("/api/v1/demo/probe-plan"),
  scenarios: () => requestJSON<SimulatorScenarioList>("/api/v1/simulator/scenarios"),
  selectScenario: (scenarioID: string) => requestJSON<DemoSession>("/api/v1/simulator/scenario", {
    method: "POST",
    body: JSON.stringify({ scenario_id: scenarioID }),
  }),
  measurements: (profileID = "ultrasonic-demo", limit = 20) =>
    requestJSON<MeasurementWindow[]>(`/api/v1/measurements?profile_id=${encodeURIComponent(profileID)}&limit=${limit}`),
  telemetryWebSocketURL: (): string | null => {
    if (typeof window === "undefined") return null;
    const base = API_URL || window.location.origin;
    const url = new URL("/api/v1/ws/telemetry", base);
    url.protocol = url.protocol === "https:" ? "wss:" : "ws:";
    return url.toString();
  },
};

export const passportApi = {
  get: (profileID: string) => requestJSON<DevicePassport>(`/api/v1/profiles/${encodeURIComponent(profileID)}/passport`),
  saveKnownGood: (profileID: string, measurementID: number, note: string) => requestJSON<KnownGoodBaseline>(
    `/api/v1/profiles/${encodeURIComponent(profileID)}/known-good`,
    { method: "POST", body: JSON.stringify({ measurement_id: measurementID, confirm_healthy: true, note }) },
  ),
};

export interface CreateProjectInput {
  name: string;
  description?: string;
  controller: string;
  logic_voltage: number;
}

function uploadFile(path: string, file: File): Promise<Project> {
  const form = new FormData();
  form.append("file", file, file.name);
  return requestJSON<Project>(path, { method: "POST", body: form });
}

export const projectApi = {
  createProject: (input: CreateProjectInput) =>
    requestJSON<Project>("/api/v1/projects", { method: "POST", body: JSON.stringify(input) }),
  listProjects: () => requestJSON<Project[]>("/api/v1/projects"),
  uploadProjectImage: (projectID: string, file: File) => uploadFile(`/api/v1/projects/${projectID}/media`, file),
  uploadProjectCode: (projectID: string, file: File) => uploadFile(`/api/v1/projects/${projectID}/code`, file),
  submitPastedCode: (projectID: string, codeText: string, filename = "pasted-code.ino") =>
    requestJSON<Project>(`/api/v1/projects/${projectID}/code`, {
      method: "POST",
      body: JSON.stringify({ code_text: codeText, filename }),
    }),
  analyzeProject: (projectID: string) =>
    requestJSON<AnalyzeProjectResponse>(`/api/v1/projects/${projectID}/analyze`, { method: "POST" }, 35_000),
  getProject: (projectID: string) => requestJSON<Project>(`/api/v1/projects/${projectID}`),
  setVisibility: (projectID: string, visibility: Project["visibility"]) =>
    requestJSON<Project>(`/api/v1/projects/${projectID}/visibility`, { method: "PUT", body: JSON.stringify({ visibility }) }),
  getDraftProfile: (projectID: string) => requestJSON<ProjectProfile>(`/api/v1/projects/${projectID}/profile`),
  saveProfileCorrections: (projectID: string, profile: ProjectProfile) =>
    requestJSON<ProjectProfile>(`/api/v1/projects/${projectID}/profile`, { method: "PUT", body: JSON.stringify(profile) }),
  confirmProfile: (projectID: string) =>
    requestJSON<ConfirmProfileResponse>(`/api/v1/projects/${projectID}/profile/confirm`, { method: "POST" }),
  getProbePlan: (projectID: string) => requestJSON<ProbePlan>(`/api/v1/projects/${projectID}/probe-plan`),
  confirmProbeConnections: (projectID: string) =>
    requestJSON<ProbePlan>(`/api/v1/projects/${projectID}/probe-plan/confirm`, { method: "POST" }),
};

export const physicalGitApi = {
  create: (projectID: string, input: { note?: string; file?: File }) => {
    if (input.file) {
      const form = new FormData();
      if (input.note) form.append("note", input.note);
      form.append("file", input.file, input.file.name);
      return requestJSON<PhysicalCommit>(`/api/v1/projects/${projectID}/physical-commits`, { method: "POST", body: form });
    }
    return requestJSON<PhysicalCommit>(`/api/v1/projects/${projectID}/physical-commits`, {
      method: "POST",
      body: JSON.stringify({ note: input.note ?? "" }),
    });
  },
  list: (projectID: string) =>
    requestJSON<{ items: PhysicalCommit[]; count: number }>(`/api/v1/projects/${projectID}/physical-commits`),
  get: (projectID: string, commitID: string) =>
    requestJSON<PhysicalCommit>(`/api/v1/projects/${projectID}/physical-commits/${encodeURIComponent(commitID)}`),
};

export const testApi = {
  recommendation: () => requestJSON<TestRecommendation>("/api/v1/tests/recommendation"),
  current: () => requestJSON<DiagnosticWorkflow>("/api/v1/tests/current"),
  create: (recommendation: TestRecommendation) => requestJSON<DiagnosticWorkflow>("/api/v1/tests", { method: "POST", body: JSON.stringify(recommendation) }),
  start: (id: string) => requestJSON<DiagnosticWorkflow>(`/api/v1/tests/${encodeURIComponent(id)}/start`, { method: "POST" }, 15_000),
  capture: (id: string) => requestJSON<DiagnosticWorkflow>(`/api/v1/tests/${encodeURIComponent(id)}/capture`, { method: "POST" }, 15_000),
  remeasure: (id: string) => requestJSON<DiagnosticWorkflow>(`/api/v1/tests/${encodeURIComponent(id)}/remeasure`, { method: "POST" }, 15_000),
  cancel: (id: string) => requestJSON<DiagnosticWorkflow>(`/api/v1/tests/${encodeURIComponent(id)}/cancel`, { method: "POST" }),
  recordAction: (id: string, description: string) => requestJSON<DiagnosticWorkflow>(`/api/v1/tests/${encodeURIComponent(id)}/actions`, { method: "POST", body: JSON.stringify({ description }) }),
};

export const historyApi = {
  list: (filters: { projectID?: string; status?: HistoryStatus | ""; sort?: "newest" | "oldest" } = {}) => {
    const query = new URLSearchParams({ limit: "100", sort: filters.sort ?? "newest" });
    if (filters.projectID) query.set("project_id", filters.projectID);
    if (filters.status) query.set("status", filters.status);
    return requestJSON<{ items: HistorySummary[]; count: number }>(`/api/v1/history?${query}`);
  },
  detail: (id: string) => requestJSON<HistoryDetail>(`/api/v1/history/${encodeURIComponent(id)}`),
  report: (id: string) => requestJSON<DetailedReport>(`/api/v1/reports/${encodeURIComponent(id)}`),
  downloadURL: (id: string, format: "json" | "md") => `/api/v1/reports/${encodeURIComponent(id)}?format=${format}&download=1`,
};

export interface ComputerAnalysis {
  snapshot: { source: string; timestamp_ms: number; system: { os: string; cpu_percent: number; memory_total_mb: number; memory_used_mb: number }; disks: Array<{ name: string; total_mb: number; used_mb: number }>; processes: Array<{ name: string; pid: number; memory_mb: number }>; ports: Array<{ port: number; pid: number; process?: string }> };
  expectations: { exclusive_port?: number; expected_service?: string; expected_local_port?: number; required_dependency?: string; expected_version?: string };
  findings: Array<{ code: string; severity: string; summary: string; next_action: string; evidence: Array<{ name: string; value: unknown; provenance: string }> }>;
  evidence: { physical_evidence: unknown[]; computer_evidence: unknown[]; software_evidence: unknown[] };
}

export const computerApi = {
  status: () => requestJSON<{ real_collection_enabled: boolean; collector: string; simulator_available: boolean; active_operations_enabled: boolean }>("/api/v1/computer/status"),
  scenarios: () => requestJSON<{ scenarios: Array<{ id: string; name: string; description: string }> }>("/api/v1/computer/scenarios"),
  simulate: (scenarioID: string) => requestJSON<ComputerAnalysis>("/api/v1/computer/simulate", { method: "POST", body: JSON.stringify({ scenario_id: scenarioID }) }),
  collect: (expected: ComputerAnalysis["expectations"]) => requestJSON<ComputerAnalysis>("/api/v1/computer/collect", { method: "POST", body: JSON.stringify(expected) }, 30_000),
};

export interface GitPreview { enabled: boolean; repo_available: boolean; files: Array<{ path: string; bytes: number }>; secret_scan: "clear" | "blocked"; commit_allowed: boolean; push_configured: boolean; detail?: string }
export const gitApi = {
  preview: (id: string) => requestJSON<GitPreview>(`/api/v1/git/preview/${encodeURIComponent(id)}`),
  commit: (id: string, push = false) => requestJSON<{ commit: string; pushed: boolean }>(`/api/v1/git/commit/${encodeURIComponent(id)}`, { method: "POST", body: JSON.stringify({ approve: true, push }) }, 25_000),
};

export interface SystemStatus { api: string; database: string; telemetry_mode: string; telemetry: string; esp32: boolean; gemini: string; patch: string; git_sync_enabled: boolean; git_repository_configured: boolean; computer_agent: string; report_retention: string; measurement_window_limit: number }
export const systemApi = { status: () => requestJSON<SystemStatus>("/api/v1/status") };
