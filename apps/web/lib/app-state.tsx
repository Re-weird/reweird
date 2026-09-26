"use client";

import { createContext, useCallback, useContext, useEffect, useState } from "react";
import { useRouter } from "next/navigation";
import type { AnalyzeProjectResponse, DemoSession, DiagnosticWorkflow, ProbePlan, Project, ProjectProfile, SimulatorScenario, TestRecommendation } from "@reweird/shared-types";
import { ApiError, demoApi, projectApi, testApi } from "@/lib/api";
import { makeDemoProfile, makeDemoSession } from "@/lib/demo";
import { DEMO_PROJECT_ID, projectPath } from "@/lib/project-routes";

interface AppState {
  session: DemoSession;
  source: "api" | "browser";
  busy: boolean;
  toast: string | null;
  project: Project | null;
  profile: ProjectProfile | null;
  probePlan: ProbePlan | null;
  scenarios: SimulatorScenario[];
  selectedScenario: string;
  setSelectedScenario: (id: string) => void;
  recommendation: TestRecommendation | null;
  workflow: DiagnosticWorkflow | null;
  testError: string | null;
  legacyVerify: boolean;
  showNewProject: boolean;
  setShowNewProject: (value: boolean) => void;
  currentProjectID: string;
  /** False until the first API session request settles; before that, `session` is only the local fixture. */
  sessionReady: boolean;

  runTestAction: (action: "plan" | "start" | "capture" | "remeasure" | "cancel") => Promise<void>;
  runOriginalDemo: (action: "wiggle" | "repair" | "reset") => Promise<void>;
  recordUserAction: (description: string) => Promise<void>;
  runScenario: () => Promise<void>;
  loadProject: (id: string) => Promise<"ready" | "missing">;
  completeProjectAnalysis: (result: AnalyzeProjectResponse) => void;
  loadDemoProject: () => Promise<void>;
  saveProfile: (nextProfile: ProjectProfile) => Promise<ProjectProfile>;
  confirmProfile: (nextProfile: ProjectProfile) => Promise<void>;
  confirmConnections: () => Promise<void>;
}

const AppStateContext = createContext<AppState | null>(null);

export function useAppState(): AppState {
  const context = useContext(AppStateContext);
  if (!context) throw new Error("useAppState must be used within AppStateProvider");
  return context;
}

export function AppStateProvider({ children }: { children: React.ReactNode }) {
  const router = useRouter();
  const [session, setSession] = useState<DemoSession>(() => makeDemoSession());
  const [source, setSource] = useState<"api" | "browser">("browser");
  const [busy, setBusy] = useState(false);
  const [showNewProject, setShowNewProject] = useState(false);
  const [toast, setToast] = useState<string | null>(null);
  const [project, setProject] = useState<Project | null>(null);
  const [profile, setProfile] = useState<ProjectProfile | null>(() => makeDemoProfile());
  const [probePlan, setProbePlan] = useState<ProbePlan | null>(null);
  const [scenarios, setScenarios] = useState<SimulatorScenario[]>([]);
  const [selectedScenario, setSelectedScenario] = useState("intermittent-connection");
  const [recommendation, setRecommendation] = useState<TestRecommendation | null>(null);
  const [workflow, setWorkflow] = useState<DiagnosticWorkflow | null>(null);
  const [testError, setTestError] = useState<string | null>(null);
  const [legacyVerify, setLegacyVerify] = useState(false);
  const [sessionReady, setSessionReady] = useState(false);

  const currentProjectID = project?.id ?? DEMO_PROJECT_ID;

  useEffect(() => {
    demoApi.load().then((remote) => {
      if (remote) { setSession(remote); setSource("api"); }
      setSessionReady(true);
    });
    demoApi.profile().then(setProfile).catch(() => undefined);
    demoApi.scenarios().then((result) => {
      setScenarios(result.scenarios);
      setSelectedScenario(result.active);
    }).catch(() => undefined);
    testApi.current().then(setWorkflow).catch(() => undefined);
    testApi.recommendation().then(setRecommendation).catch(() => undefined);
  }, []);

  // From main: with no real project open, show the demo's probe plan (used by
  // the circuit map and device passport).
  useEffect(() => {
    if (project) return;
    let cancelled = false;
    demoApi.probePlan().then((plan) => { if (!cancelled) setProbePlan(plan); }).catch(() => undefined);
    return () => { cancelled = true; };
  }, [project]);

  useEffect(() => {
    if (!toast) return;
    const timer = setTimeout(() => setToast(null), 2800);
    return () => clearTimeout(timer);
  }, [toast]);

  useEffect(() => {
    if (source !== "api") return;
    const url = demoApi.telemetryWebSocketURL();
    if (!url) return;
    let cancelled = false;
    let socket: WebSocket | null = null;
    try {
      socket = new WebSocket(url);
    } catch {
      return;
    }
    socket.onmessage = (event) => {
      if (cancelled) return;
      try {
        const next = JSON.parse(event.data) as DemoSession;
        if (next && next.stage) setSession(next);
      } catch {
        // Ignore malformed frames; the next push will self-correct.
      }
    };
    return () => {
      cancelled = true;
      socket?.close();
    };
  }, [source]);

  const runTestAction = useCallback(async (action: "plan" | "start" | "capture" | "remeasure" | "cancel") => {
    setLegacyVerify(false);
    router.push(projectPath(currentProjectID, "next-test"));
    setTestError(null);
    if (source !== "api") { setTestError("Start the Go API to capture and persist a real guided workflow; browser demo data is not used."); return; }
    if (project && profile?.id !== session.profile_id) { setTestError("The active telemetry source does not match this project's confirmed profile."); return; }
    setBusy(true);
    try {
      let next: DiagnosticWorkflow;
      if (action === "plan") {
        const proposed = await testApi.recommendation();
        setRecommendation(proposed);
        next = await testApi.create(proposed);
      } else {
        if (!workflow) throw new Error("Create a test plan first.");
        next = await testApi[action](workflow.id);
      }
      setWorkflow(next);
      if (next.verification) router.push(projectPath(currentProjectID, "verify"));
      setToast(`Guided test: ${next.status.replaceAll("_", " ").toLowerCase()}`);
    } catch (error) {
      setTestError(error instanceof ApiError || error instanceof Error ? error.message : "The guided test could not continue.");
    } finally { setBusy(false); }
  }, [router, currentProjectID, source, project, profile, session.profile_id, workflow]);

  const runOriginalDemo = useCallback(async (action: "wiggle" | "repair" | "reset") => {
    setBusy(true);
    setProject(null);
    setProbePlan(null);
    setProfile(makeDemoProfile());
    const remote = await demoApi[action]();
    const next = remote ?? makeDemoSession(action === "wiggle" ? "test" : action === "repair" ? "verify" : "diagnose");
    setSession(next);
    setSource(remote ? "api" : "browser");
    setLegacyVerify(action === "repair");
    if (action === "repair") router.push(projectPath(DEMO_PROJECT_ID, "verify"));
    if (action === "reset") router.push(projectPath(DEMO_PROJECT_ID, "simulator"));
    setBusy(false);
  }, [router]);

  const recordUserAction = useCallback(async (description: string) => {
    if (!workflow) return;
    setBusy(true);
    setTestError(null);
    try { setWorkflow(await testApi.recordAction(workflow.id, description)); setToast("User action added to diagnostic history"); }
    catch (cause) { setTestError(cause instanceof Error ? cause.message : "The action could not be recorded."); }
    finally { setBusy(false); }
  }, [workflow]);

  const runScenario = useCallback(async () => {
    setBusy(true);
    try {
      const next = await demoApi.selectScenario(selectedScenario);
      setProject(null);
      setProbePlan(null);
      setProfile(makeDemoProfile());
      setSession(next);
      setSource("api");
      setWorkflow(null);
      setLegacyVerify(false);
      setRecommendation(await testApi.recommendation().catch(() => null));
      setToast(`${scenarios.find((scenario) => scenario.id === selectedScenario)?.name ?? "Scenario"} analyzed from raw telemetry`);
    } catch {
      setToast("The API simulator is unavailable; start the Go backend to run fault scenarios");
    } finally {
      setBusy(false);
    }
  }, [selectedScenario, scenarios]);

  const loadProject = useCallback(async (id: string): Promise<"ready" | "missing"> => {
    if (id === DEMO_PROJECT_ID) {
      if (project) {
        setProject(null);
        setProbePlan(null);
        setProfile(makeDemoProfile());
        demoApi.profile().then(setProfile).catch(() => undefined);
      }
      return "ready";
    }
    if (project?.id === id) return "ready";
    setBusy(true);
    try {
      const freshProject = await projectApi.getProject(id);
      const freshProfile = await projectApi.getDraftProfile(id).catch(() => null);
      setProject(freshProject);
      setProfile(freshProfile);
      try { setProbePlan(await projectApi.getProbePlan(id)); } catch { setProbePlan(null); }
      setWorkflow(null);
      return "ready";
    } catch {
      return "missing";
    } finally {
      setBusy(false);
    }
  }, [project]);

  const completeProjectAnalysis = useCallback((result: AnalyzeProjectResponse) => {
    setProject(result.project);
    setProfile(result.profile);
    setProbePlan(null);
    setShowNewProject(false);
    router.push(projectPath(result.project.id, "overview"));
    setToast("Draft Project Profile generated from real input");
  }, [router]);

  const loadDemoProject = useCallback(async () => {
    setShowNewProject(false);
    setProject(null);
    setProbePlan(null);
    setWorkflow(null);
    setLegacyVerify(false);
    const remote = await demoApi.reset();
    setSession(remote ?? makeDemoSession());
    setSource(remote ? "api" : "browser");
    try { setProfile(await demoApi.profile()); } catch { setProfile(makeDemoProfile()); }
    try { setProbePlan(await demoApi.probePlan()); } catch { setProbePlan(null); }
    router.push(projectPath(DEMO_PROJECT_ID, "overview"));
    setToast("Built-in ultrasonic demo loaded");
  }, [router]);

  const saveProfile = useCallback(async (nextProfile: ProjectProfile) => {
    if (!project) return nextProfile;
    const stored = await projectApi.saveProfileCorrections(project.id, nextProfile);
    setProfile(stored);
    setToast("Profile corrections persisted");
    return stored;
  }, [project]);

  const confirmProfile = useCallback(async (nextProfile: ProjectProfile) => {
    if (!project) return;
    const stored = await projectApi.saveProfileCorrections(project.id, nextProfile);
    const confirmed = await projectApi.confirmProfile(project.id);
    setProfile(confirmed.profile);
    setProbePlan(confirmed.probe_plan);
    setProject(await projectApi.getProject(project.id));
    router.push(projectPath(project.id, "probe-setup"));
    setToast(`Profile confirmed at revision ${stored.version}; probe plan generated`);
  }, [project, router]);

  const confirmConnections = useCallback(async () => {
    if (!project) return;
    const confirmed = await projectApi.confirmProbeConnections(project.id);
    setProbePlan(confirmed);
    const remote = await demoApi.load();
    if (remote && profile && remote.profile_id === profile.id) { setSession(remote); setSource("api"); }
    router.push(projectPath(project.id, "workbench"));
    setToast("Probe connections confirmed; live diagnostics unlocked");
  }, [project, profile, router]);

  const value: AppState = {
    session, source, busy, toast, project, profile, probePlan, scenarios, selectedScenario, setSelectedScenario,
    recommendation, workflow, testError, legacyVerify, showNewProject, setShowNewProject, currentProjectID, sessionReady,
    runTestAction, runOriginalDemo, recordUserAction, runScenario, loadProject, completeProjectAnalysis,
    loadDemoProject, saveProfile, confirmProfile, confirmConnections,
  };

  return <AppStateContext.Provider value={value}>{children}</AppStateContext.Provider>;
}
