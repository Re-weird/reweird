"use client";

import { useEffect, useMemo, useState } from "react";
import {
  Activity,
  BarChart3,
  Bolt,
  Box,
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
  Upload,
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
import type { DemoSession, ProbeReading, RuleResult } from "@reweird/shared-types";
import { demoApi } from "@/lib/api";
import { makeDemoSession } from "@/lib/demo";

type View = "dashboard" | "profile" | "live" | "diagnosis" | "verify" | "reports";

const nav: { id: View; label: string; icon: typeof Activity }[] = [
  { id: "dashboard", label: "Overview", icon: LayoutDashboard },
  { id: "profile", label: "Project profile", icon: Box },
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
  children,
  onNewProject,
}: {
  active: View;
  setActive: (view: View) => void;
  session: DemoSession;
  source: "api" | "browser";
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
          <div><small>Active project</small><strong>{session.project_name}</strong></div>
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
          <div className="topbar-title"><span>{nav.find((item) => item.id === active)?.label}</span><small>ESP32 / HC-SR04</small></div>
          <div className="top-actions">
            <div className="connection-pill"><span /> Hardware connected</div>
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

function ProfileView() {
  const [editing, setEditing] = useState(false);
  return (
    <>
      <section className="page-heading"><div><p className="kicker">Confirmed project context</p><h1>Project profile</h1><p>AI suggestions remain editable until a person confirms the hardware assumptions.</p></div><button className={editing ? "primary" : "secondary"} onClick={() => setEditing(!editing)}>{editing ? <Check size={16} /> : <Settings size={16} />}{editing ? "Save corrections" : "Correct profile"}</button></section>
      <section className="profile-grid">
        <div className="panel profile-summary"><div className="project-visual"><Cpu size={46} /><span>ESP32</span></div><div><span className="eyebrow">Demo project</span><h2>Ultrasonic Distance Sensor</h2><p>Measures distance continuously and reports a return pulse from an HC-SR04 sensor.</p><div className="spec-chips"><span>3.3 V logic</span><span>5 V rail</span><span>Pulse interface</span></div></div></div>
        <div className="panel"><div className="panel-heading"><div><span className="eyebrow">GPIO map</span><h2>Confirmed connections</h2></div><span className="confirmed"><Check size={13} /> Human confirmed</span></div><div className="pin-map"><div><span>HC-SR04 TRIG</span><b>GPIO 5</b><small>Output · P2</small></div><div><span>HC-SR04 ECHO</span><b>GPIO 18</b><small>Input · P3</small></div><div><span>VCC</span><b>5V rail</b><small>Power · P1</small></div><div><span>GND</span><b>Common</b><small>Reference</small></div></div></div>
      </section>
      <section className="panel component-table"><div className="panel-heading"><div><span className="eyebrow">Component catalog</span><h2>Detected hardware</h2></div></div><div className="table-head"><span>Component</span><span>Source</span><span>Confidence</span><span>Status</span></div><div className="table-row"><span><Cpu size={17} /> ESP32 DevKit</span><span>Image + code</span><span>96%</span><span className="status-label stable">Confirmed</span></div><div className="table-row"><span><Radio size={17} /> HC-SR04</span><span>Image + library</span><span>94%</span><span className="status-label stable">Confirmed</span></div><div className="table-row"><span><Box size={17} /> Breadboard + jumpers</span><span>Image</span><span>89%</span><span className="status-label stable">Confirmed</span></div></section>
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

function NewProjectModal({ onClose, onCreated }: { onClose: () => void; onCreated: () => void }) {
  return (
    <div className="modal-backdrop" role="presentation" onMouseDown={onClose}>
      <form className="modal" onSubmit={(event) => { event.preventDefault(); onCreated(); }} onMouseDown={(event) => event.stopPropagation()}>
        <div className="modal-head"><div><span className="eyebrow">New workspace</span><h2>Create a project</h2></div><button type="button" className="icon-button" onClick={onClose}><X size={18} /></button></div>
        <label>Project name<input required defaultValue="My electronics project" /></label>
        <label>Description<textarea rows={3} placeholder="What should the project do?" /></label>
        <div className="form-row"><label>Controller<select defaultValue="ESP32"><option>ESP32</option><option>Arduino Uno</option><option>Raspberry Pi Pico</option></select></label><label>Logic voltage<select defaultValue="3.3 V"><option>3.3 V</option><option>5 V</option></select></label></div>
        <div className="upload-grid"><label><Upload size={20} /><span>Hardware photo</span><small>PNG, JPG, or video</small><input type="file" accept="image/*,video/*" /></label><label><Upload size={20} /><span>Project code</span><small>.ino, .cpp, .py, or ZIP</small><input type="file" accept=".ino,.cpp,.h,.py,.zip" /></label></div>
        <div className="modal-actions"><button type="button" className="secondary" onClick={onClose}>Cancel</button><button className="primary" type="submit">Create project <ChevronRight size={16} /></button></div>
      </form>
    </div>
  );
}

export default function Home() {
  const [session, setSession] = useState<DemoSession>(() => makeDemoSession());
  const [active, setActive] = useState<View>("dashboard");
  const [source, setSource] = useState<"api" | "browser">("browser");
  const [busy, setBusy] = useState(false);
  const [showNewProject, setShowNewProject] = useState(false);
  const [toast, setToast] = useState<string | null>(null);

  useEffect(() => {
    demoApi.load().then((remote) => {
      if (remote) { setSession(remote); setSource("api"); }
    });
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

  const view = useMemo(() => {
    if (active === "dashboard") return <DashboardView session={session} setActive={setActive} />;
    if (active === "profile") return <ProfileView />;
    if (active === "live") return <LiveView session={session} />;
    if (active === "diagnosis") return <DiagnosisView session={session} onWiggle={() => action("wiggle")} onRepair={() => action("repair")} busy={busy} />;
    if (active === "verify") return <VerifyView session={session} onReset={() => action("reset")} busy={busy} />;
    return <ReportsView session={session} />;
  }, [active, session, busy]);

  return (
    <AppShell active={active} setActive={setActive} session={session} source={source} onNewProject={() => setShowNewProject(true)}>
      {view}
      {showNewProject && <NewProjectModal onClose={() => setShowNewProject(false)} onCreated={() => { setShowNewProject(false); setActive("profile"); setToast("Project shell created — confirm the generated profile next"); }} />}
      {toast && <div className="toast"><CheckCircle2 size={18} />{toast}</div>}
    </AppShell>
  );
}
