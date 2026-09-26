"use client";

import { useEffect, useMemo, useState } from "react";
import Image from "next/image";
import {
  Activity,
  BarChart3,
  Bolt,
  Box,
  Cable,
  Check,
  CheckCircle2,
  ChevronRight,
  CircleDot,
  Cpu,
  FileBarChart,
  LayoutDashboard,
  Menu,
  Microscope,
  Moon,
  Plus,
  Radio,
  RefreshCw,
  Settings,
  ShieldCheck,
  Sparkles,
  Sun,
  TestTube2,
  TriangleAlert,
  X,
  Upload,
} from "lucide-react";
import {
  Area,
  AreaChart,
  CartesianGrid,
  ResponsiveContainer,
  Tooltip,
  XAxis,
  YAxis,
} from "recharts";
import type { AnalyzeProjectResponse, DemoSession, DiagnosticWorkflow, ProbePlan, ProbeReading, Project, ProjectProfile, RuleResult, SimulatorScenario, TestRecommendation } from "@reweird/shared-types";
import { ApiError, demoApi, projectApi, testApi } from "@/lib/api";
import { makeDemoProfile, makeDemoSession } from "@/lib/demo";
import { NewProjectModal, ProbePlanView, ProjectProfileView } from "./project-workflow";
import { GuidedTestView } from "./guided-test";
import { HistoryReportView } from "./history-report";
import { ComputerDiagnosticsView } from "./computer-diagnostics";
import { SettingsStatusView } from "./settings-status";
import { Workbench } from "./workbench";

type View = "dashboard" | "profile" | "connect" | "simulator" | "live" | "diagnosis" | "guided" | "verify" | "history" | "reports" | "computer" | "settings";
type Theme = "light" | "dark";

const nav: { id: View; label: string; icon: typeof Activity; group: "Workspace" | "Diagnostic flow" | "Records" }[] = [
  { id: "dashboard", label: "Workbench", icon: LayoutDashboard, group: "Workspace" },
  { id: "profile", label: "Project overview", icon: Box, group: "Workspace" },
  { id: "connect", label: "Probe setup", icon: Cable, group: "Workspace" },
  { id: "simulator", label: "Simulator", icon: TestTube2, group: "Diagnostic flow" },
  { id: "live", label: "Live signals", icon: Activity, group: "Diagnostic flow" },
  { id: "diagnosis", label: "Diagnosis", icon: Microscope, group: "Diagnostic flow" },
  { id: "guided", label: "Next test", icon: TestTube2, group: "Diagnostic flow" },
  { id: "verify", label: "Verify result", icon: CheckCircle2, group: "Diagnostic flow" },
  { id: "history", label: "History", icon: RefreshCw, group: "Records" },
  { id: "reports", label: "Reports", icon: FileBarChart, group: "Records" },
  { id: "computer", label: "Computer checks", icon: Cpu, group: "Records" },
];

const navGroups = ["Workspace", "Diagnostic flow", "Records"] as const;

function StatusDot({ status }: { status: ProbeReading["status"] }) {
  return <span className={`status-dot ${status}`} aria-label={status} />;
}

function MiniChart({ values, danger = false }: { values: number[] | null; danger?: boolean }) {
  if (!values?.length) return <div className="mini-empty">No probe assigned</div>;
  const max = Math.max(...values, 1);
  const points = values
    .map((value, index) => `${values.length === 1 ? 60 : (index / (values.length - 1)) * 120},${35 - (value / max) * 29}`)
    .join(" ");
  return (
    <svg className="mini-chart" viewBox="0 0 120 38" preserveAspectRatio="none" role="img" aria-label="Recent signal trend">
      <polyline className={danger ? "danger-line" : "signal-line"} points={points} fill="none" />
    </svg>
  );
}

function ProbeCard({ reading }: { reading: ProbeReading }) {
  const danger = reading.status === "intermittent";
  return (
    <article className={`probe-card ${danger ? "probe-alert" : ""} ${reading.status === "idle" ? "muted-card" : ""}`}>
      <div className="probe-top">
        <div>
          <span className="eyebrow">{reading.probe}</span>
          <h3>{reading.role}</h3>
        </div>
        <StatusDot status={reading.status} />
      </div>
      <div className="probe-reading">
        <strong>{reading.value === null ? "—" : reading.value.toFixed(reading.unit === "V" ? 2 : 1)}</strong>
        <span>{reading.unit}</span>
      </div>
      <MiniChart values={reading.samples} danger={danger} />
      <div className="probe-foot">
        <span className={`status-label ${reading.status}`}>{reading.status}</span>
        {reading.dropouts > 0 && <span>{reading.dropouts} dropouts/min</span>}
      </div>
    </article>
  );
}

function RuleRow({ rule }: { rule: RuleResult }) {
  const Icon = rule.status === "pass" ? Check : rule.status === "warn" ? CircleDot : TriangleAlert;
  return (
    <div className={`rule-row ${rule.status}`}>
      <span className="rule-icon"><Icon size={15} /></span>
      <span>{rule.message}</span>
    </div>
  );
}

function ConfidenceRing({ value }: { value: number }) {
  const degrees = Math.round(value * 360);
  return (
    <div className="confidence-ring" style={{ background: `conic-gradient(var(--cyan) ${degrees}deg, var(--line) 0deg)` }}>
      <div><strong>{Math.round(value * 100)}%</strong><span>confidence</span></div>
    </div>
  );
}

function SignalChart({ session }: { session: DemoSession }) {
  const charted = session.probes.filter((probe) => probe.samples?.length).slice(0, 2);
  const rows = (charted[0]?.samples ?? []).map((_, index) => ({
    time: `${index * 5}s`,
    primary: charted[0]?.samples?.[index],
    secondary: charted[1]?.samples?.[index],
  }));
  return (
    <div className="chart-wrap">
      <ResponsiveContainer width="100%" height={240}>
        <AreaChart data={rows} margin={{ top: 12, right: 8, left: -25, bottom: 0 }}>
          <defs>
            <linearGradient id="echoGradient" x1="0" y1="0" x2="0" y2="1">
              <stop offset="0%" stopColor="var(--cyan)" stopOpacity={0.12} />
              <stop offset="100%" stopColor="var(--cyan)" stopOpacity={0} />
            </linearGradient>
          </defs>
          <CartesianGrid stroke="var(--line)" vertical={false} />
          <XAxis dataKey="time" stroke="var(--muted)" fontSize={11} tickLine={false} axisLine={false} />
          <YAxis stroke="var(--muted)" fontSize={11} tickLine={false} axisLine={false} />
          <Tooltip contentStyle={{ background: "var(--surface)", color: "var(--text)", border: "1px solid var(--line)", borderRadius: 10, fontSize: 12 }} />
          <Area type="linear" dataKey="primary" stroke="var(--cyan)" strokeWidth={1.5} fill="url(#echoGradient)" name={charted[0] ? `${charted[0].probe} ${charted[0].role}` : "Signal"} />
          {charted[1] && <Area type="linear" dataKey="secondary" stroke="var(--green)" strokeWidth={1.5} fill="transparent" name={`${charted[1].probe} ${charted[1].role}`} />}
        </AreaChart>
      </ResponsiveContainer>
    </div>
  );
}

function AppShell({
  active,
  setActive,
  session,
  source,
  projectName,
  projectContext,
  hardwareConnected,
  children,
  onNewProject,
  onLoadDemo,
}: {
  active: View;
  setActive: (view: View) => void;
  session: DemoSession;
  source: "api" | "browser";
  projectName: string;
  projectContext: string;
  hardwareConnected: boolean;
  children: React.ReactNode;
  onNewProject: () => void;
  onLoadDemo: () => void;
}) {
  const [mobileOpen, setMobileOpen] = useState(false);
  const [theme, setTheme] = useState<Theme>("dark");

  useEffect(() => {
    let stored: string | null = null;
    try { stored = window.localStorage.getItem("reweird-theme"); } catch { /* Theme still works when browser storage is unavailable. */ }
    const nextTheme: Theme = stored === "light" || stored === "dark"
      ? stored
      : window.matchMedia("(prefers-color-scheme: light)").matches ? "light" : "dark";
    setTheme(nextTheme);
    document.documentElement.dataset.theme = nextTheme;
  }, []);

  const toggleTheme = () => {
    const nextTheme: Theme = theme === "dark" ? "light" : "dark";
    setTheme(nextTheme);
    document.documentElement.dataset.theme = nextTheme;
    try { window.localStorage.setItem("reweird-theme", nextTheme); } catch { /* Keep the in-session preference. */ }
  };

  const sourceLabel = source === "browser"
    ? "Demo data"
    : session.telemetry_mode === "serial" ? "ESP32 serial" : "API simulator";

  return (
    <div className="app-shell">
      <aside className={`sidebar ${mobileOpen ? "mobile-open" : ""}`}>
        <button className="mobile-close" onClick={() => setMobileOpen(false)} aria-label="Close menu"><X size={20} /></button>
        <div className="brand"><span className="brand-mark"><Image src="/images/reweird-logo.png" alt="" width={72} height={72} className="brand-logo-image" priority /></span><span>Re<span>Weird</span></span></div>
        <button className="project-switcher" onClick={() => { setActive("profile"); setMobileOpen(false); }}>
          <span className="device-icon"><Cpu size={18} /></span>
          <div><small>Active project</small><strong>{projectName}</strong></div>
          <ChevronRight size={16} />
        </button>
        <nav aria-label="Primary navigation">
          {navGroups.map((group) => (
            <div className="nav-group" key={group}>
              <p className="nav-label">{group}</p>
              {nav.filter((item) => item.group === group).map((item) => {
                const Icon = item.icon;
                return (
                  <button key={item.id} className={active === item.id ? "active" : ""} aria-current={active === item.id ? "page" : undefined} onClick={() => { setActive(item.id); setMobileOpen(false); }}>
                    <Icon size={17} /><span>{item.label}</span>
                    {item.id === "diagnosis" && session.stage !== "verify" && <i />}
                  </button>
                );
              })}
              {group === "Workspace" && <button onClick={() => { onNewProject(); setMobileOpen(false); }}><Upload size={17} /><span>Upload project</span></button>}
            </div>
          ))}
        </nav>
        <div className="sidebar-bottom">
          <button onClick={() => { setActive("settings"); setMobileOpen(false); }} aria-current={active === "settings" ? "page" : undefined}><Settings size={17} /> Settings & status</button>
          <div className="operator"><span>RW</span><div><strong>Local session</strong><small>No user sign-in</small></div></div>
        </div>
      </aside>

      {mobileOpen && <button className="nav-backdrop" onClick={() => setMobileOpen(false)} aria-label="Dismiss navigation" />}
      <main>
        <header className="topbar">
          <button className="mobile-menu" onClick={() => setMobileOpen(true)} aria-label="Open menu"><Menu size={20} /></button>
          <div className="topbar-title"><span>{active === "settings" ? "Settings & status" : nav.find((item) => item.id === active)?.label}</span><small>{projectContext}</small></div>
          <div className="top-actions">
            <div className={`connection-pill ${hardwareConnected ? "" : "waiting"}`}><span /> {hardwareConnected ? "Hardware connected" : "Hardware offline"}</div>
            <div className={`mode-pill source-${source === "browser" ? "demo" : session.telemetry_mode === "serial" ? "hardware" : "simulator"}`}><Radio size={13} /> {sourceLabel}</div>
            <button className="icon-button theme-toggle" onClick={toggleTheme} aria-label={`Switch to ${theme === "dark" ? "light" : "dark"} mode`} title={`Switch to ${theme === "dark" ? "light" : "dark"} mode`}>
              {theme === "dark" ? <Sun size={16} /> : <Moon size={16} />}
            </button>
            <button className="secondary compact" onClick={onLoadDemo}>Load demo</button>
            <button className="primary compact" onClick={onNewProject}><Plus size={16} /> New project</button>
          </div>
        </header>
        <div className={`content ${active === "dashboard" ? "workbench-content" : ""}`}>{children}</div>
      </main>
    </div>
  );
}


function LiveView({ session }: { session: DemoSession }) {
  const charted = session.probes.filter((probe) => probe.samples?.length).slice(0, 2);
  return (
    <>
      <section className="page-heading"><div><p className="kicker">Session {session.id}</p><h1>Live diagnostics</h1><p>{session.telemetry_mode === "serial" ? "ESP32 serial" : "Simulator"} telemetry is normalized through the same contract used by every transport.</p></div><div className="live-badge"><span /> {session.telemetry_mode === "serial" ? "SERIAL" : "SIMULATED"} · {session.raw_telemetry ? `${(1000 / session.raw_telemetry.window_ms).toFixed(2)} Hz` : "No raw frame"}</div></section>
      <section className="probe-grid">{session.probes.map((probe) => <ProbeCard reading={probe} key={probe.probe} />)}</section>
      <section className="two-column wide-left">
        <div className="panel"><div className="panel-heading"><div><span className="eyebrow">Raw activity buckets</span><h2>Signal activity</h2></div><div className="chart-legend">{charted.map((probe) => <span key={probe.probe}>{probe.probe} {probe.role}</span>)}</div></div><SignalChart session={session} /></div>
        <div className="panel"><div className="panel-heading"><div><span className="eyebrow">Analysis</span><h2>Rule engine</h2></div></div><div className="rules-list">{session.evidence.rule_results.map((rule) => <RuleRow rule={rule} key={rule.id} />)}</div><div className="engine-note"><Bolt size={16} /><span>Deterministic checks run before any AI interpretation.</span></div></div>
      </section>
    </>
  );
}

function DiagnosisView({ session, source, onPlan, busy }: { session: DemoSession; source: "api" | "browser"; onPlan: () => void; busy: boolean }) {
  const tested = session.stage === "test" || session.stage === "repair";
  const focus = session.probes.find((probe) => probe.probe === session.evidence.probe);
  const expected = Object.entries(session.evidence.expected).slice(0, 3);
  const observed = Object.entries(session.evidence.observed).slice(0, 3);
  const baseline = Object.entries(session.evidence.baseline).slice(0, 3);
  const renderFacts = (facts: [string, unknown][]) => facts.map(([key, value]) => <small key={key}>{key.replaceAll("_", " ")}: {String(value)}</small>);
  return (
    <>
      <section className="page-heading"><div><p className="kicker">Evidence review</p><h1>{session.diagnosis.headline}</h1><p>Measurement, rule output, and inference are kept visibly separate.</p></div><ConfidenceRing value={session.diagnosis.confidence} /></section>
      <section className="diagnosis-layout">
        <div className="panel evidence-panel">
          <div className="panel-heading"><div><span className="eyebrow">Structured evidence</span><h2>Expected vs. observed</h2></div><span className="probe-tag">{session.evidence.probe} · {session.evidence.role}</span></div>
          <div className="compare-grid">
            <div><span>Expected</span><strong>{String(session.evidence.expected.signal ?? "Configured behavior")}</strong>{renderFacts(expected)}</div>
            <div className="observed"><span>Observed</span><strong>{focus?.dropouts ?? 0} detected dropouts</strong>{renderFacts(observed)}</div>
            <div><span>Baseline</span><strong>{String(session.evidence.baseline.status ?? "Unknown")}</strong>{renderFacts(baseline)}</div>
          </div>
          <div className="rules-list">{session.evidence.rule_results.map((rule) => <RuleRow rule={rule} key={rule.id} />)}</div>
        </div>
        <div className="panel interpretation-card">
          <div className="interpretation-label"><Microscope size={16} /> PROBE interpretation <span>{source === "browser" ? "Demo" : session.telemetry_mode === "serial" ? "Serial evidence" : "Simulated evidence"}</span></div>
          <h2>{session.diagnosis.summary}</h2>
          <p>Possible causes, ranked but not asserted as measured truth:</p>
          <ol>{session.diagnosis.possible_causes.map((cause, index) => <li key={cause}><span>{index + 1}</span>{cause}</li>)}</ol>
        </div>
      </section>
      <section className="test-callout">
        <div className="test-icon"><TestTube2 size={24} /></div>
        <div><span className="eyebrow">Recommended next test</span><h2>{session.diagnosis.next_test}</h2><p>{tested ? "The repeated movement correlation provides stronger evidence than the initial anomaly alone." : "This test is user-guided and only monitors input. PATCH output remains disabled."}</p></div>
        <button className="primary" onClick={onPlan} disabled={busy}>{busy ? <RefreshCw className="spin" size={17} /> : <Activity size={17} />} Open guided test planner</button>
      </section>
    </>
  );
}

function SimulatorView({
  session,
  scenarios,
  selected,
  setSelected,
  onRun,
  onPlan,
  onDemoTest,
  onDemoRepair,
  busy,
}: {
  session: DemoSession;
  scenarios: SimulatorScenario[];
  selected: string;
  setSelected: (id: string) => void;
  onRun: () => void;
  onPlan: () => void;
  onDemoTest: () => void;
  onDemoRepair: () => void;
  busy: boolean;
}) {
  const activeScenario = scenarios.find((scenario) => scenario.id === session.scenario_id);
  return (
    <>
      <section className="page-heading"><div><p className="kicker">Layers 3–4 software harness</p><h1>Raw telemetry fault simulator</h1><p>Each case emits bounded electrical samples through the same validation, normalization, profile matching, storage, analysis, and diagnosis path reserved for ESP32 serial.</p></div><div className="live-badge"><span /> PATCH locked</div></section>
      <section className="pipeline-strip" aria-label="Telemetry processing pipeline">
        {["Raw samples", "Validate", "Normalize", "Match profile", "Store window", "Signal analysis", "Evidence", "Diagnosis"].map((step, index) => <div key={step}><span>{index + 1}</span>{step}</div>)}
      </section>
      <section className="simulator-layout">
        <div className="panel scenario-panel">
          <div className="panel-heading"><div><span className="eyebrow">Simulated faults</span><h2>Select a deterministic input</h2></div></div>
          <div className="scenario-list">
            {scenarios.map((scenario) => (
              <label className={selected === scenario.id ? "selected" : ""} key={scenario.id}>
                <input type="radio" name="scenario" value={scenario.id} checked={selected === scenario.id} onChange={() => setSelected(scenario.id)} />
                <span><strong>{scenario.name}</strong><small>{scenario.description}</small></span>
              </label>
            ))}
          </div>
          <button className="primary full" disabled={busy || !selected} onClick={onRun}>{busy ? <RefreshCw className="spin" size={17} /> : <TestTube2 size={17} />} Load raw samples and analyze</button>
        </div>
        <div className="panel simulator-result">
          <div className="panel-heading"><div><span className="eyebrow">Current result</span><h2>{session.diagnosis.headline}</h2></div><span className="session-id">#{session.measurement_id ?? "memory"}</span></div>
          <p>{session.stage === "verify" ? "VERIFY captured a fresh healthy window and compared it with the original fault evidence." : activeScenario?.description ?? "Select a scenario to run it through the backend pipeline."}</p>
          <div className="result-meta"><span>Contract v{session.raw_telemetry?.schema_version ?? 2}</span><span>{session.raw_telemetry?.device_id ?? "browser fixture"}</span><span>Profile {session.profile_id ?? "ultrasonic-demo"}</span></div>
          <div className="analysis-table">
            <div className="analysis-head"><span>Probe</span><span>Raw input</span><span>Derived facts</span><span>Status</span></div>
            {session.probes.filter((probe) => probe.role !== "UNASSIGNED").map((probe) => {
              const raw = session.raw_telemetry?.samples.find((sample) => sample.probe === probe.probe);
              const facts = session.analysis?.probes.find((item) => item.probe === probe.probe);
              const rawCount = raw?.analog_mv?.length ?? raw?.periods_us?.length ?? raw?.activity_counts?.length ?? 0;
              const derived = facts?.average_voltage !== undefined
                ? `${facts.average_voltage.toFixed(2)} V · Δ ${facts.voltage_variation?.toFixed(2) ?? "0.00"} V`
                : facts?.frequency_hz !== undefined
                  ? `${facts.frequency_hz.toFixed(1)} Hz · ${facts.duty_cycle_percent?.toFixed(1) ?? "—"}% duty`
                  : `${facts?.digital_transitions ?? 0} transitions`;
              return <div className="analysis-row" key={probe.probe}><span><b>{probe.probe}</b><small>{probe.role}</small></span><span>{rawCount} samples<small>{raw?.max_gap_us ? `max gap ${raw.max_gap_us} µs` : raw?.mode}</small></span><span>{derived}<small>{facts?.jitter_us !== undefined ? `jitter ${facts.jitter_us.toFixed(2)} µs` : `${facts?.dropout_events ?? probe.dropouts} dropouts`}</small></span><span className={`status-label ${probe.status}`}>{probe.status}</span></div>;
            })}
          </div>
          {!!session.analysis?.simultaneous_dropout_groups?.length && <div className="shared-failure"><TriangleAlert size={17} /> Shared failure group: {session.analysis.simultaneous_dropout_groups.map((group) => group.join(" + ")).join(", ")}</div>}
          <div className="simulator-actions"><button className="primary" onClick={onPlan} disabled={busy}><Activity size={16} /> Plan guided test &amp; VERIFY</button><button className="secondary" onClick={onDemoTest} disabled={busy}><TestTube2 size={16} /> Original demo test</button><button className="secondary" onClick={onDemoRepair} disabled={busy}><CheckCircle2 size={16} /> Original demo repair</button></div>
        </div>
      </section>
      <section className="security-note"><ShieldCheck size={20} /><div><strong>Input-only by design</strong><span>The simulator and future ESP32 serial adapter can only supply measurements. The PATCH endpoint remains physically and logically disabled.</span></div></section>
    </>
  );
}

function LegacyDemoVerifyView({ session, onReset, busy }: { session: DemoSession; onReset: () => void; busy: boolean }) {
  const verified = session.stage === "verify";
  return <>
    <section className="page-heading"><div><p className="kicker">Original demo · Browser fallback</p><h1>{verified ? "Demo signal returned to baseline" : "Demo verification is waiting"}</h1><p>This is the original simulated HC-SR04 loop, separate from persisted generic guided tests.</p></div>{verified && <div className="verified-seal"><CheckCircle2 size={24} /> DEMO VERIFIED</div>}</section>
    <section className={`verification-panel ${verified ? "ready" : "locked"}`}><div className="before-after"><div><span>Before demo repair</span><strong>{session.before.dropouts_per_minute}</strong><small>dropouts / minute</small></div><div className="delta-arrow"><ChevronRight size={26} /></div><div><span>After demo repair</span><strong>{session.after?.dropouts_per_minute ?? "—"}</strong><small>dropouts / minute</small></div></div><div className="verification-summary"><div><h2>{verified ? "Original demo correction simulated" : "No demo repair yet"}</h2><p>{verified ? session.diagnosis.summary : "Run the original demo test and repair from the Fault simulator."}</p></div></div></section>
    {verified && <button className="secondary center-button" onClick={onReset} disabled={busy}><RefreshCw size={16} /> Reset original demo</button>}
  </>;
}

function ProjectLivePending({ project, profile, plan }: { project: Project; profile: ProjectProfile | null; plan: ProbePlan | null }) {
  return (
    <>
      <section className="page-heading"><div><p className="kicker">Live diagnostics · Project ready</p><h1>Waiting for matching telemetry</h1><p>{project.name} is confirmed and its probe plan is connected. Live cards will populate when the ReWeird device sends frames matching this profile.</p></div><div className="live-badge"><span /> Armed</div></section>
      <section className="panel live-pending-panel"><Cable size={34} /><div><h2>{profile?.project_name ?? project.name}</h2><p>The browser will not substitute HC-SR04 demo measurements for this project. Start the API with this profile and connect the configured ESP32 telemetry source.</p><div className="spec-chips"><span>Profile: {profile?.id}</span><span>{plan?.instructions.length ?? 0} placement steps</span><span>PATCH locked</span></div></div></section>
      <section className="security-note"><ShieldCheck size={20} /><div><strong>No fabricated measurements</strong><span>Only validated telemetry that matches the confirmed P1–P6 modes can enter the diagnostic engine.</span></div></section>
    </>
  );
}

function DemoProbePlanView({ plan, onContinue }: { plan: ProbePlan | null; onContinue: () => void }) {
  return <section><div className="page-heading"><div><span className="eyebrow">BUILT-IN SIMULATOR</span><h1>Demo probe plan</h1><p>This illustrative plan is generated from the confirmed demo profile. No physical probe connection is claimed.</p></div></div>{plan ? <div className="probe-plan-grid">{plan.instructions.map((step) => <article className={`probe-instruction ${step.probe === "GND" ? "ground" : ""}`} key={step.probe}><div className="probe-badge">{step.probe}</div><div><span className="eyebrow">{step.role}</span><h2>{step.target}</h2><p>{step.expected} · {step.signal_type}</p><div className="safety-warning"><ShieldCheck size={13} />{step.safe_warning}</div></div></article>)}</div> : <div className="empty-state panel"><p>Start the API to generate the demo placement plan from the profile.</p></div>}<button className="primary" onClick={onContinue}>Continue to simulator <ChevronRight size={15} /></button></section>;
}

export default function Home() {
  const [session, setSession] = useState<DemoSession>(() => makeDemoSession());
  const [active, setActive] = useState<View>("dashboard");
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

  useEffect(() => {
    demoApi.load().then((remote) => {
      if (remote) { setSession(remote); setSource("api"); }
    });
    demoApi.profile().then(setProfile).catch(() => undefined);
    demoApi.scenarios().then((result) => {
      setScenarios(result.scenarios);
      setSelectedScenario(result.active);
    }).catch(() => undefined);
    testApi.current().then(setWorkflow).catch(() => undefined);
    testApi.recommendation().then(setRecommendation).catch(() => undefined);
  }, []);

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

  const runTestAction = async (action: "plan" | "start" | "capture" | "remeasure" | "cancel") => {
    setLegacyVerify(false);
    setActive("guided");
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
      if (next.verification) setActive("verify");
      setToast(`Guided test: ${next.status.replaceAll("_", " ").toLowerCase()}`);
    } catch (error) {
      setTestError(error instanceof ApiError || error instanceof Error ? error.message : "The guided test could not continue.");
    } finally { setBusy(false); }
  };

  const runOriginalDemo = async (action: "wiggle" | "repair" | "reset") => {
    setBusy(true);
    setProject(null);
    setProbePlan(null);
    setProfile(makeDemoProfile());
    const remote = await demoApi[action]();
    const next = remote ?? makeDemoSession(action === "wiggle" ? "test" : action === "repair" ? "verify" : "diagnose");
    setSession(next);
    setSource(remote ? "api" : "browser");
    setLegacyVerify(action === "repair");
    if (action === "repair") setActive("verify");
    if (action === "reset") setActive("simulator");
    setBusy(false);
  };

  const recordUserAction = async (description: string) => {
    if (!workflow) return;
    setBusy(true);
    setTestError(null);
    try { setWorkflow(await testApi.recordAction(workflow.id, description)); setToast("User action added to diagnostic history"); }
    catch (cause) { setTestError(cause instanceof Error ? cause.message : "The action could not be recorded."); }
    finally { setBusy(false); }
  };

  const runScenario = async () => {
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
  };

  const completeProjectAnalysis = (result: AnalyzeProjectResponse) => {
    setProject(result.project);
    setProfile(result.profile);
    setProbePlan(null);
    setShowNewProject(false);
    setActive("profile");
    setToast("Draft Project Profile generated from real input");
  };

  const loadDemoProject = async () => {
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
    setActive("profile");
    setToast("Built-in ultrasonic demo loaded");
  };

  const saveProfile = async (nextProfile: ProjectProfile) => {
    if (!project) return nextProfile;
    const stored = await projectApi.saveProfileCorrections(project.id, nextProfile);
    setProfile(stored);
    setToast("Profile corrections persisted");
    return stored;
  };

  const confirmProfile = async (nextProfile: ProjectProfile) => {
    if (!project) return;
    const stored = await projectApi.saveProfileCorrections(project.id, nextProfile);
    const confirmed = await projectApi.confirmProfile(project.id);
    setProfile(confirmed.profile);
    setProbePlan(confirmed.probe_plan);
    setProject(await projectApi.getProject(project.id));
    setActive("connect");
    setToast(`Profile confirmed at revision ${stored.version}; probe plan generated`);
  };

  const confirmConnections = async () => {
    if (!project) return;
    const confirmed = await projectApi.confirmProbeConnections(project.id);
    setProbePlan(confirmed);
    const remote = await demoApi.load();
    if (remote && profile && remote.profile_id === profile.id) { setSession(remote); setSource("api"); }
    setActive("live");
    setToast("Probe connections confirmed; live diagnostics unlocked");
  };

  const view = useMemo(() => {
    if (active === "dashboard") return <Workbench session={session} project={project} profile={profile} source={source} onNavigate={setActive} onUpload={() => setShowNewProject(true)} />;
    if (active === "profile") return <ProjectProfileView project={project} profile={profile} plan={probePlan ?? project?.probe_plan ?? null} session={session} onSave={saveProfile} onConfirm={confirmProfile} onNavigate={setActive} />;
    if (active === "connect") return project ? <ProbePlanView project={project} plan={probePlan ?? project?.probe_plan ?? null} onConnected={confirmConnections} /> : <DemoProbePlanView plan={probePlan} onContinue={() => setActive("simulator")} />;
    if (active === "simulator") return <SimulatorView session={session} scenarios={scenarios} selected={selectedScenario} setSelected={setSelectedScenario} onRun={runScenario} onPlan={() => runTestAction("plan")} onDemoTest={() => runOriginalDemo("wiggle")} onDemoRepair={() => runOriginalDemo("repair")} busy={busy} />;
    if (active === "live") {
      if (project && session.profile_id !== profile?.id) return <ProjectLivePending project={project} profile={profile} plan={probePlan} />;
      return <LiveView session={session} />;
    }
    if (active === "diagnosis") return project && session.profile_id !== profile?.id
      ? <ProjectLivePending project={project} profile={profile} plan={probePlan} />
      : <DiagnosisView session={session} source={source} onPlan={() => runTestAction("plan")} busy={busy} />;
    if (active === "verify" && legacyVerify) return <LegacyDemoVerifyView session={session} onReset={() => runOriginalDemo("reset")} busy={busy} />;
    if (active === "guided" || active === "verify") return <GuidedTestView workflow={workflow} recommendation={recommendation} busy={busy} error={testError} onPlan={() => runTestAction("plan")} onStart={() => runTestAction("start")} onCapture={() => runTestAction("capture")} onRemeasure={() => runTestAction("remeasure")} onCancel={() => runTestAction("cancel")} onRecordAction={recordUserAction} />;
    if (active === "history") return <HistoryReportView mode="history" />;
    if (active === "computer") return <ComputerDiagnosticsView />;
    if (active === "settings") return <SettingsStatusView />;
    return <HistoryReportView mode="reports" />;
  }, [active, session, source, busy, project, profile, probePlan, scenarios, selectedScenario, workflow, recommendation, testError, legacyVerify]);

  return (
    <AppShell active={active} setActive={setActive} session={session} source={source} projectName={project?.name ?? session.project_name} projectContext={project ? `${project.controller} · ${project.analysis_status}` : "ESP32 · Built-in demo"} hardwareConnected={source === "api" && session.telemetry_mode === "serial" && (!project || session.profile_id === profile?.id) ? session.hardware_connected : false} onNewProject={() => setShowNewProject(true)} onLoadDemo={loadDemoProject}>
      {view}
      {showNewProject && <NewProjectModal onClose={() => setShowNewProject(false)} onComplete={completeProjectAnalysis} onLoadDemo={loadDemoProject} />}
      {toast && <div className="toast" role="status"><CheckCircle2 size={18} />{toast}</div>}
    </AppShell>
  );
}
