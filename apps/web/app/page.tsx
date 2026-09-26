"use client";

import { useEffect, useMemo, useState } from "react";
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
  Download,
  FileBarChart,
  Gauge,
  GitBranch,
  LayoutDashboard,
  Menu,
  Microscope,
  Plus,
  Radio,
  RefreshCw,
  Search,
  Settings,
  ShieldCheck,
  Sparkles,
  TestTube2,
  TriangleAlert,
  Waves,
  X,
  Zap,
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
import type { AnalyzeProjectResponse, DemoSession, ProbePlan, ProbeReading, Project, ProjectProfile, RuleResult } from "@reweird/shared-types";
import { demoApi, projectApi } from "@/lib/api";
import { makeDemoProfile, makeDemoSession } from "@/lib/demo";
import { NewProjectModal, ProbePlanView, ProjectProfileView } from "./project-workflow";

type View = "dashboard" | "profile" | "connect" | "live" | "diagnosis" | "verify" | "reports";

const nav: { id: View; label: string; icon: typeof Activity }[] = [
  { id: "dashboard", label: "Overview", icon: LayoutDashboard },
  { id: "profile", label: "Project profile", icon: Box },
  { id: "connect", label: "Connect ReWeird", icon: Cable },
  { id: "live", label: "Live diagnostics", icon: Activity },
  { id: "diagnosis", label: "Diagnosis", icon: Microscope },
  { id: "verify", label: "Verify", icon: CheckCircle2 },
  { id: "reports", label: "Reports", icon: FileBarChart },
];

const stageIndex = { detect: 0, diagnose: 1, test: 2, repair: 2, verify: 3 } as const;

function StatusDot({ status }: { status: ProbeReading["status"] }) {
  return <span className={`status-dot ${status}`} aria-label={status} />;
}

function MiniChart({ values, danger = false }: { values: number[]; danger?: boolean }) {
  if (!values.length) return <div className="mini-empty">No probe assigned</div>;
  const max = Math.max(...values, 1);
  const points = values
    .map((value, index) => `${(index / (values.length - 1)) * 120},${35 - (value / max) * 29}`)
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
      <div><strong>{Math.round(value * 100)}%</strong><span>evidence</span></div>
    </div>
  );
}

function SignalChart({ session }: { session: DemoSession }) {
  const rows = session.probes[0].samples.map((_, index) => ({
    time: `${index * 5}s`,
    power: session.probes[0].samples[index],
    echo: session.probes[2].samples[index],
  }));
  return (
    <div className="chart-wrap">
      <ResponsiveContainer width="100%" height={240}>
        <AreaChart data={rows} margin={{ top: 12, right: 8, left: -25, bottom: 0 }}>
          <defs>
            <linearGradient id="echoGradient" x1="0" y1="0" x2="0" y2="1">
              <stop offset="0%" stopColor="#ff5c7a" stopOpacity={0.32} />
              <stop offset="100%" stopColor="#ff5c7a" stopOpacity={0} />
            </linearGradient>
          </defs>
          <CartesianGrid stroke="#1d2831" vertical={false} />
          <XAxis dataKey="time" stroke="#65717b" fontSize={11} tickLine={false} axisLine={false} />
          <YAxis stroke="#65717b" fontSize={11} tickLine={false} axisLine={false} />
          <Tooltip contentStyle={{ background: "#111a21", border: "1px solid #2b3943", borderRadius: 10, fontSize: 12 }} />
          <Area type="monotone" dataKey="echo" stroke="#ff5c7a" strokeWidth={2} fill="url(#echoGradient)" name="P3 ECHO" />
          <Area type="monotone" dataKey="power" stroke="#23d5ab" strokeWidth={2} fill="transparent" name="P1 POWER" />
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
}) {
  const [mobileOpen, setMobileOpen] = useState(false);
  return (
    <div className="app-shell">
      <aside className={`sidebar ${mobileOpen ? "mobile-open" : ""}`}>
        <button className="mobile-close" onClick={() => setMobileOpen(false)} aria-label="Close menu"><X size={20} /></button>
        <div className="brand"><span className="brand-mark"><Waves size={22} /></span><span>Re<span>Weird</span></span></div>
        <div className="project-switcher">
          <span className="device-icon"><Cpu size={18} /></span>
          <div><small>Active project</small><strong>{projectName}</strong></div>
          <ChevronRight size={16} />
        </div>
        <nav>
          <p className="nav-label">Workspace</p>
          {nav.map((item) => {
            const Icon = item.icon;
            return (
              <button key={item.id} className={active === item.id ? "active" : ""} onClick={() => { setActive(item.id); setMobileOpen(false); }}>
                <Icon size={18} /><span>{item.label}</span>
                {item.id === "diagnosis" && session.stage !== "verify" && <i />}
              </button>
            );
          })}
        </nav>
        <div className="sidebar-bottom">
          <button><GitBranch size={17} /> Integrations</button>
          <button><Settings size={17} /> Settings</button>
          <div className="operator"><span>RA</span><div><strong>Demo operator</strong><small>Local workspace</small></div></div>
        </div>
      </aside>

      <main>
        <header className="topbar">
          <button className="mobile-menu" onClick={() => setMobileOpen(true)} aria-label="Open menu"><Menu size={20} /></button>
          <div className="topbar-title"><span>{nav.find((item) => item.id === active)?.label}</span><small>{projectContext}</small></div>
          <div className="top-actions">
            <div className={`connection-pill ${hardwareConnected ? "" : "waiting"}`}><span /> {hardwareConnected ? "Hardware connected" : "Hardware waiting"}</div>
            <div className="mode-pill"><Radio size={13} /> {source === "api" ? "API simulator" : "Browser simulator"}</div>
            <button className="icon-button" aria-label="Search"><Search size={18} /></button>
            <button className="primary compact" onClick={onNewProject}><Plus size={16} /> New project</button>
          </div>
        </header>
        <div className="content">{children}</div>
      </main>
    </div>
  );
}

function DashboardView({ session, setActive }: { session: DemoSession; setActive: (view: View) => void }) {
  const alertCount = session.stage === "verify" ? 0 : 1;
  return (
    <>
      <section className="hero-row">
        <div>
          <p className="kicker">Diagnostic workspace</p>
          <h1>Good evening, engineer.</h1>
          <p>One active session is collecting evidence from your ultrasonic sensor project.</p>
        </div>
        <button className="secondary" onClick={() => setActive("live")}><Activity size={17} /> Open live session</button>
      </section>

      <section className="metric-grid">
        <article className="metric-card"><span className="metric-icon green"><Radio size={18} /></span><div><small>Hardware</small><strong>Connected</strong><em>ESP32 · 4 probes</em></div></article>
        <article className="metric-card"><span className="metric-icon cyan"><Gauge size={18} /></span><div><small>Samples analyzed</small><strong>18,420</strong><em>+1,240 this session</em></div></article>
        <article className="metric-card"><span className={`metric-icon ${alertCount ? "red" : "green"}`}><TriangleAlert size={18} /></span><div><small>Active findings</small><strong>{alertCount}</strong><em>{alertCount ? "P3 needs attention" : "All signals nominal"}</em></div></article>
        <article className="metric-card"><span className="metric-icon violet"><ShieldCheck size={18} /></span><div><small>Safety state</small><strong>Protected</strong><em>PATCH output locked</em></div></article>
      </section>

      <section className="panel workflow-panel">
        <div className="panel-heading"><div><span className="eyebrow">Current run</span><h2>Detect → Diagnose → Test → Verify</h2></div><span className="session-id">RW-2409-017</span></div>
        <div className="workflow">
          {session.timeline.map((step, index) => (
            <div className={`workflow-step ${step.complete ? "complete" : ""} ${stageIndex[session.stage] === index ? "current" : ""}`} key={step.id}>
              <div className="step-marker">{step.complete ? <Check size={16} /> : index + 1}</div>
              <div><strong>{step.label}</strong><span>{step.detail}</span></div>
              {index < session.timeline.length - 1 && <div className="step-line" />}
            </div>
          ))}
        </div>
      </section>

      <section className="two-column">
        <div className="panel">
          <div className="panel-heading"><div><span className="eyebrow">Live signals</span><h2>Probe overview</h2></div><button className="text-button" onClick={() => setActive("live")}>View all <ChevronRight size={15} /></button></div>
          <div className="compact-probes">
            {session.probes.slice(0, 3).map((probe) => (
              <div className="compact-probe" key={probe.probe}>
                <span className="probe-name"><StatusDot status={probe.status} /><b>{probe.probe}</b> {probe.role}</span>
                <MiniChart values={probe.samples} danger={probe.status === "intermittent"} />
                <strong>{probe.value?.toFixed(probe.unit === "V" ? 2 : 1)} <small>{probe.unit}</small></strong>
              </div>
            ))}
          </div>
        </div>
        <div className={`panel finding-card ${session.stage === "verify" ? "resolved" : ""}`}>
          <div className="finding-title"><span><Sparkles size={18} /></span><div><small>Latest finding</small><h2>{session.diagnosis.headline}</h2></div></div>
          <p>{session.diagnosis.summary}</p>
          <div className="evidence-chips"><span><Check size={13} /> Rail stable</span><span><Check size={13} /> TRIG active</span><span className={session.stage === "verify" ? "" : "danger-chip"}>{session.probes[2].dropouts} dropouts</span></div>
          <button className="primary full" onClick={() => setActive(session.stage === "verify" ? "verify" : "diagnosis")}>{session.stage === "verify" ? "View verification" : "Review diagnosis"}<ChevronRight size={17} /></button>
        </div>
      </section>
    </>
  );
}

function LiveView({ session }: { session: DemoSession }) {
  return (
    <>
      <section className="page-heading"><div><p className="kicker">Session RW-2409-017</p><h1>Live diagnostics</h1><p>Simulator telemetry is normalized through the same contract used by physical hardware.</p></div><div className="live-badge"><span /> LIVE · 20 Hz</div></section>
      <section className="probe-grid">{session.probes.map((probe) => <ProbeCard reading={probe} key={probe.probe} />)}</section>
      <section className="two-column wide-left">
        <div className="panel"><div className="panel-heading"><div><span className="eyebrow">Last 60 seconds</span><h2>Signal activity</h2></div><div className="chart-legend"><span className="echo" />P3 ECHO <span className="power" />P1 POWER</div></div><SignalChart session={session} /></div>
        <div className="panel"><div className="panel-heading"><div><span className="eyebrow">Analysis</span><h2>Rule engine</h2></div></div><div className="rules-list">{session.evidence.rule_results.map((rule) => <RuleRow rule={rule} key={rule.id} />)}</div><div className="engine-note"><Bolt size={16} /><span>Deterministic checks run before any AI interpretation.</span></div></div>
      </section>
    </>
  );
}

function DiagnosisView({ session, onWiggle, onRepair, busy }: { session: DemoSession; onWiggle: () => void; onRepair: () => void; busy: boolean }) {
  const tested = session.stage === "test" || session.stage === "repair";
  return (
    <>
      <section className="page-heading"><div><p className="kicker">Evidence review</p><h1>{session.diagnosis.headline}</h1><p>Measurement, rule output, and inference are kept visibly separate.</p></div><ConfidenceRing value={session.diagnosis.confidence} /></section>
      <section className="diagnosis-layout">
        <div className="panel evidence-panel">
          <div className="panel-heading"><div><span className="eyebrow">Structured evidence</span><h2>Expected vs. observed</h2></div><span className="probe-tag">P3 · ECHO</span></div>
          <div className="compare-grid">
            <div><span>Expected</span><strong>Continuous return pulses</strong><small>0 dropouts / minute</small></div>
            <div className="observed"><span>Observed</span><strong>{session.probes[2].dropouts} unexpected dropouts</strong><small>Power rail stayed at 5.01 V</small></div>
            <div><span>Baseline</span><strong>28.4 pulses / second</strong><small>Captured while healthy</small></div>
          </div>
          <div className="rules-list">{session.evidence.rule_results.map((rule) => <RuleRow rule={rule} key={rule.id} />)}</div>
        </div>
        <div className="panel interpretation-card">
          <div className="interpretation-label"><Sparkles size={16} /> PROBE interpretation <span>mocked</span></div>
          <h2>{session.diagnosis.summary}</h2>
          <p>Possible causes, ranked but not asserted as measured truth:</p>
          <ol>{session.diagnosis.possible_causes.map((cause, index) => <li key={cause}><span>{index + 1}</span>{cause}</li>)}</ol>
        </div>
      </section>
      <section className="test-callout">
        <div className="test-icon"><TestTube2 size={24} /></div>
        <div><span className="eyebrow">Recommended next test</span><h2>{session.diagnosis.next_test}</h2><p>{tested ? "The repeated movement correlation provides stronger evidence than the initial anomaly alone." : "This test is user-guided and only monitors input. PATCH output remains disabled."}</p></div>
        {!tested ? <button className="primary" onClick={onWiggle} disabled={busy}>{busy ? <RefreshCw className="spin" size={17} /> : <Activity size={17} />} Start wiggle test</button> : <button className="primary" onClick={onRepair} disabled={busy}>{busy ? <RefreshCw className="spin" size={17} /> : <Zap size={17} />} Simulate repair</button>}
      </section>
    </>
  );
}

function VerifyView({ session, onReset, busy }: { session: DemoSession; onReset: () => void; busy: boolean }) {
  const verified = session.stage === "verify";
  return (
    <>
      <section className="page-heading"><div><p className="kicker">Repair verification</p><h1>{verified ? "Signal returned to baseline" : "Verification is waiting"}</h1><p>{verified ? "Re-measurement confirms the repair changed the observed behavior." : "Complete the guided test and simulated repair to unlock before/after evidence."}</p></div>{verified && <div className="verified-seal"><CheckCircle2 size={24} /> VERIFIED</div>}</section>
      <section className={`verification-panel ${verified ? "ready" : "locked"}`}>
        <div className="before-after">
          <div><span>Before repair</span><strong>{session.before.dropouts_per_minute}</strong><small>dropouts / minute</small><em className="bad">Intermittent</em></div>
          <div className="delta-arrow"><ChevronRight size={26} /></div>
          <div><span>After repair</span><strong>{session.after?.dropouts_per_minute ?? "—"}</strong><small>dropouts / minute</small><em className={verified ? "good" : "pending"}>{session.after?.stability ?? "Pending"}</em></div>
        </div>
        <div className="verification-summary"><span className="big-check">{verified ? <Check size={30} /> : <Gauge size={30} />}</span><div><h2>{verified ? "Issue appears resolved" : "No post-repair sample yet"}</h2><p>{verified ? "P3 ECHO is stable, the rail remains healthy, and the dropout rate now matches the stored baseline." : "ReWeird will compare the next measurement window with the original fault evidence."}</p></div></div>
      </section>
      {verified && <button className="secondary center-button" onClick={onReset} disabled={busy}><RefreshCw size={16} /> Reset demo</button>}
    </>
  );
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

function ReportsView({ session }: { session: DemoSession }) {
  function downloadReport() {
    const blob = new Blob([JSON.stringify(session, null, 2)], { type: "application/json" });
    const url = URL.createObjectURL(blob);
    const anchor = document.createElement("a");
    anchor.href = url;
    anchor.download = "reweird-diagnostic-RW-2409-017.json";
    anchor.click();
    URL.revokeObjectURL(url);
  }
  return (
    <>
      <section className="page-heading"><div><p className="kicker">Audit-ready history</p><h1>Diagnostic reports</h1><p>Reports preserve measurements, deterministic rules, user actions, and AI interpretation separately.</p></div><button className="primary" onClick={downloadReport}><Download size={16} /> Download JSON</button></section>
      <section className="panel reports-table"><div className="report-row report-head"><span>Report</span><span>Finding</span><span>Status</span><span>Evidence</span><span /></div><div className="report-row"><span><b>RW-2409-017</b><small>Today · 6:42 PM</small></span><span>P3 intermittent ECHO</span><span className={`status-label ${session.stage === "verify" ? "stable" : "intermittent"}`}>{session.stage === "verify" ? "Resolved" : "In progress"}</span><span>{session.evidence.rule_results.length} rule results</span><button className="icon-button" onClick={downloadReport} aria-label="Download report"><Download size={16} /></button></div></section>
      <section className="security-note"><ShieldCheck size={20} /><div><strong>Git synchronization is off</strong><span>No report or project data leaves this workspace without explicit approval and a secrets scan.</span></div></section>
    </>
  );
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

  useEffect(() => {
    demoApi.load().then((remote) => {
      if (remote) { setSession(remote); setSource("api"); }
    });
    demoApi.profile().then(setProfile).catch(() => undefined);
  }, []);

  useEffect(() => {
    if (!toast) return;
    const timer = setTimeout(() => setToast(null), 2800);
    return () => clearTimeout(timer);
  }, [toast]);

  const action = async (kind: "wiggle" | "repair" | "reset") => {
    setBusy(true);
    const remote = await demoApi[kind]();
    const next = remote ?? makeDemoSession(kind === "wiggle" ? "test" : kind === "repair" ? "verify" : "diagnose");
    setSession(next);
    setSource(remote ? "api" : "browser");
    setBusy(false);
    if (kind === "wiggle") setToast("Wiggle test complete: movement correlation detected");
    if (kind === "repair") { setToast("Repair verified: P3 matches the healthy baseline"); setActive("verify"); }
    if (kind === "reset") { setToast("Demo reset to the initial fault"); setActive("dashboard"); }
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
    try { setProfile(await demoApi.profile()); } catch { setProfile(makeDemoProfile()); }
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
    if (active === "dashboard") return <DashboardView session={session} setActive={setActive} />;
    if (active === "profile") return <ProjectProfileView project={project} profile={profile} onSave={saveProfile} onConfirm={confirmProfile} />;
    if (active === "connect") return <ProbePlanView project={project} plan={probePlan ?? project?.probe_plan ?? null} onConnected={confirmConnections} />;
    if (active === "live") {
      if (project && session.profile_id !== profile?.id) return <ProjectLivePending project={project} profile={profile} plan={probePlan} />;
      return <LiveView session={session} />;
    }
    if (active === "diagnosis") return <DiagnosisView session={session} onWiggle={() => action("wiggle")} onRepair={() => action("repair")} busy={busy} />;
    if (active === "verify") return <VerifyView session={session} onReset={() => action("reset")} busy={busy} />;
    return <ReportsView session={session} />;
  }, [active, session, busy, project, profile, probePlan]);

  return (
    <AppShell active={active} setActive={setActive} session={session} source={source} projectName={project?.name ?? session.project_name} projectContext={project ? `${project.controller} · ${project.analysis_status}` : "ESP32 · Built-in demo"} hardwareConnected={!project || session.profile_id === profile?.id ? session.hardware_connected : false} onNewProject={() => setShowNewProject(true)}>
      {view}
      {showNewProject && <NewProjectModal onClose={() => setShowNewProject(false)} onComplete={completeProjectAnalysis} onLoadDemo={loadDemoProject} />}
      {toast && <div className="toast"><CheckCircle2 size={18} />{toast}</div>}
    </AppShell>
  );
}
