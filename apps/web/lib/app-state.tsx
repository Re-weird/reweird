"use client";

import { createContext, useCallback, useContext, useEffect, useRef, useState } from "react";
import { useRouter } from "next/navigation";
import type { AnalyzeProjectResponse, DemoSession, DiagnosticWorkflow, ProbePlan, Project, ProjectProfile, SimulatorScenario, TestRecommendation } from "@reweird/shared-types";
import { refreshAccountActivity } from "@/lib/account-activity";
import { ApiError, demoApi, projectApi, testApi } from "@/lib/api";
import { makeDemoProfile, makeDemoSession } from "@/lib/demo";
import { DEMO_HISTORY_PROJECT_ID, DEMO_PROJECT_ID, projectPath } from "@/lib/project-routes";

export type ProjectLoadResult = "ready" | "missing" | "unavailable";

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
  scenarioError: string | null;
  mysteryPending: boolean;
  revealMystery: () => void;
  setSelectedScenario: (id: string) => void;
  recommendation: TestRecommendation | null;
  workflow: DiagnosticWorkflow | null;
  testError: string | null;
  legacyVerify: boolean;
  showNewProject: boolean;
  setShowNewProject: (value: boolean) => void;
  currentProjectID: string;
  /** Project id that this project's diagnostic history is recorded under. */
  historyProjectID: string;
  /** False until the first API session request settles; before that, `session` is only the local fixture. */
  sessionReady: boolean;

  runTestAction: (action: "plan" | "start" | "capture" | "remeasure" | "cancel") => Promise<void>;
  runOriginalDemo: (action: "wiggle" | "repair" | "reset") => Promise<void>;
  recordUserAction: (description: string) => Promise<void>;
  runScenario: (scenarioID?: string, mystery?: boolean) => Promise<void>;
  loadProject: (id: string) => Promise<ProjectLoadResult>;
  completeProjectAnalysis: (result: AnalyzeProjectResponse) => void;
  loadDemoProject: () => Promise<void>;
  saveProfile: (nextProfile: ProjectProfile) => Promise<ProjectProfile>;
  /** Re-reads the linked GitHub repo's default branch; throws with the reason when it can't be analyzed. */
  syncRepository: () => Promise<void>;
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
  const [scenarioError, setScenarioError] = useState<string | null>(null);
  const [mysteryPending, setMysteryPending] = useState(false);
  const [recommendation, setRecommendation] = useState<TestRecommendation | null>(null);
  const [workflow, setWorkflow] = useState<DiagnosticWorkflow | null>(null);
  const [testError, setTestError] = useState<string | null>(null);
  const [legacyVerify, setLegacyVerify] = useState(false);
  const [sessionReady, setSessionReady] = useState(false);
  // The id most recently asked for. A slower, earlier request must not
  // overwrite the project the user has since navigated to.
  const requestedProjectRef = useRef<string | null>(null);


  const currentProjectID = project?.id ?? DEMO_PROJECT_ID;
  const historyProjectID = project?.id ?? DEMO_HISTORY_PROJECT_ID;
  // The API's "current" workflow is global; only expose it when it belongs to
  // the project being viewed, so one project's pages never act on another's test.
  const scopedWorkflow = workflow && workflow.project_id === historyProjectID ? workflow : null;

  useEffect(() => {
    demoApi.load().then((remote) => {
      if (remote) { setSession(remote); setSource("api"); }
      setSessionReady(true);
    });
    // Only while still in demo mode: if a real project URL was opened directly,
    // this can resolve after loadProject and would overwrite its profile.
    demoApi.profile().then((demoProfile) => {
      const requested = requestedProjectRef.current;
      if (requested === null || requested === DEMO_PROJECT_ID) setProfile(demoProfile);
    }).catch(() => undefined);
    demoApi.scenarios().then((result) => {
      setScenarios(result.scenarios);
      setSelectedScenario(result.active);
    }).catch(() => undefined);
    testApi.current().then(setWorkflow).catch(() => undefined);
    testApi.recommendation().then(setRecommendation).catch(() => undefined);
  }, []);

  // From main: with no real project open, show the demo's probe plan (used by
  // the circuit map).
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
        if (!scopedWorkflow) throw new Error("Create a test plan first.");
        next = await testApi[action](scopedWorkflow.id);
      }
      setWorkflow(next);
      if (next.verification) router.push(projectPath(currentProjectID, "verify"));
      setToast(`Guided test: ${next.status.replaceAll("_", " ").toLowerCase()}`);
    } catch (error) {
      setTestError(error instanceof ApiError || error instanceof Error ? error.message : "The guided test could not continue.");
    } finally { setBusy(false); }
  }, [router, currentProjectID, source, project, profile, session.profile_id, scopedWorkflow]);

  const runOriginalDemo = useCallback(async (action: "wiggle" | "repair" | "reset") => {
    setBusy(true);
    setMysteryPending(false);
    const remote = await demoApi[action]();
    setProject(null);
    setProbePlan(null);
    setProfile(makeDemoProfile());
    const next = remote ?? makeDemoSession(action === "wiggle" ? "test" : action === "repair" ? "verify" : "diagnose");
    setSession(next);
    setSource(remote ? "api" : "browser");
    setLegacyVerify(action === "repair");
    // These actions swap in demo data, so they always land on the demo
    // project's URL; staying on a real project's URL left its layout waiting
    // for a project that is no longer in state.
    router.push(projectPath(DEMO_PROJECT_ID, action === "repair" ? "verify" : "simulator"));
    setBusy(false);
  }, [router]);

  const recordUserAction = useCallback(async (description: string) => {
    if (!scopedWorkflow) return;
    setBusy(true);
    setTestError(null);
    try { setWorkflow(await testApi.recordAction(scopedWorkflow.id, description)); setToast("User action added to diagnostic history"); }
    catch (cause) { setTestError(cause instanceof Error ? cause.message : "The action could not be recorded."); }
    finally { setBusy(false); }
  }, [scopedWorkflow]);

  const runScenario = useCallback(async (scenarioID?: string, mystery = false) => {
    setBusy(true);
    setScenarioError(null);
    try {
      const chosen = scenarioID ?? selectedScenario;
      const next = await demoApi.selectScenario(chosen);
      const nextRecommendation = await testApi.recommendation().catch(() => null);
      // Fetch first, then swap to demo data and navigate in the same tick, so
      // a real project's URL is never left showing without its project.
      setProject(null);
      setProbePlan(null);
      setProfile(makeDemoProfile());
      setSession(next);
      setSelectedScenario(chosen);
      setMysteryPending(mystery);
      setSource("api");
      setWorkflow(null);
      setLegacyVerify(false);
      setRecommendation(nextRecommendation);
      router.push(projectPath(DEMO_PROJECT_ID, "simulator"));
      setToast(mystery ? "Mystery scenario analyzed from simulated raw telemetry" : `${scenarios.find((scenario) => scenario.id === chosen)?.name ?? "Scenario"} analyzed from simulated raw telemetry`);
    } catch (cause) {
      const message = cause instanceof Error ? cause.message : "The simulator could not analyze this fault.";
      setScenarioError(`${message} The selected result has not changed.`);
      setToast("Could not load the selected fault");
    } finally {
      setBusy(false);
    }
  }, [selectedScenario, scenarios, router]);

  const loadProject = useCallback(async (id: string): Promise<ProjectLoadResult> => {
    requestedProjectRef.current = id;
    if (id === DEMO_PROJECT_ID) {
      if (project) {
        setProject(null);
        setProbePlan(null);
        setProfile(makeDemoProfile());
        demoApi.profile().then((demoProfile) => { if (requestedProjectRef.current === DEMO_PROJECT_ID) setProfile(demoProfile); }).catch(() => undefined);
      }
      return "ready";
    }
    if (project?.id === id) return "ready";
    const stale = () => requestedProjectRef.current !== id;
    setBusy(true);
    try {
      const freshProject = await projectApi.getProject(id);
      const freshProfile = await projectApi.getDraftProfile(id).catch(() => null);
      const freshPlan = await projectApi.getProbePlan(id).catch(() => null);
      if (stale()) return "ready";
      setProject(freshProject);
      setProfile(freshProfile);
      setProbePlan(freshPlan);
      setWorkflow(null);
      return "ready";
    } catch (cause) {
      // Only a 404 means the project doesn't exist; a timeout or an unreachable
      // API (status 0) must not be reported as "not found".
      return cause instanceof ApiError && cause.status === 404 ? "missing" : "unavailable";
    } finally {
      setBusy(false);
    }
  }, [project]);

  const completeProjectAnalysis = useCallback((result: AnalyzeProjectResponse) => {
    refreshAccountActivity();
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
    setMysteryPending(false);
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

  const syncRepository = useCallback(async () => {
    if (!project) return;
    const projectID = project.id;
    try {
      const result = await projectApi.syncRepository(projectID);
      if (requestedProjectRef.current !== projectID) return;
      setProject(result.project);
      if (result.profile) {
        setProfile(result.profile);
        setProbePlan(null);
        setToast(`Analyzed ${result.project.repository?.last_commit?.sha.slice(0, 7) ?? "the latest commit"}`);
      } else {
        setToast("Already up to date with the default branch");
      }
    } catch (cause) {
      // The failed sync is recorded on the project; reload it so the page shows why.
      const fresh = await projectApi.getProject(projectID).catch(() => null);
      if (fresh && requestedProjectRef.current === projectID) setProject(fresh);
      throw cause;
    }
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
    session, source, busy, toast, project, profile, probePlan, scenarios, selectedScenario, scenarioError, setSelectedScenario, mysteryPending, revealMystery: () => setMysteryPending(false),
    recommendation, workflow: scopedWorkflow, testError, legacyVerify, showNewProject, setShowNewProject, currentProjectID, historyProjectID, sessionReady,
    runTestAction, runOriginalDemo, recordUserAction, runScenario, loadProject, completeProjectAnalysis,
    loadDemoProject, saveProfile, syncRepository, confirmProfile, confirmConnections,
  };

  return <AppStateContext.Provider value={value}>{children}</AppStateContext.Provider>;
}
