import type {
  AnalyzeProjectResponse,
  ConfirmProfileResponse,
  DemoSession,
  MeasurementWindow,
  ProbePlan,
  Project,
  ProjectProfile,
  SimulatorScenarioList,
} from "@reweird/shared-types";

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
  scenarios: () => requestJSON<SimulatorScenarioList>("/api/v1/simulator/scenarios"),
  selectScenario: (scenarioID: string) => requestJSON<DemoSession>("/api/v1/simulator/scenario", {
    method: "POST",
    body: JSON.stringify({ scenario_id: scenarioID }),
  }),
  measurements: (profileID = "ultrasonic-demo", limit = 20) =>
    requestJSON<MeasurementWindow[]>(`/api/v1/measurements?profile_id=${encodeURIComponent(profileID)}&limit=${limit}`),
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
  getDraftProfile: (projectID: string) => requestJSON<ProjectProfile>(`/api/v1/projects/${projectID}/profile`),
  saveProfileCorrections: (projectID: string, profile: ProjectProfile) =>
    requestJSON<ProjectProfile>(`/api/v1/projects/${projectID}/profile`, { method: "PUT", body: JSON.stringify(profile) }),
  confirmProfile: (projectID: string) =>
    requestJSON<ConfirmProfileResponse>(`/api/v1/projects/${projectID}/profile/confirm`, { method: "POST" }),
  getProbePlan: (projectID: string) => requestJSON<ProbePlan>(`/api/v1/projects/${projectID}/probe-plan`),
  confirmProbeConnections: (projectID: string) =>
    requestJSON<ProbePlan>(`/api/v1/projects/${projectID}/probe-plan/confirm`, { method: "POST" }),
};
