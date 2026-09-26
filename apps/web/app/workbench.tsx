"use client";

import { useEffect, useState } from "react";
import { Activity, ArrowDownToLine, ArrowUpRight, Bolt, Check, ChevronRight, Cpu, FileText, History, LockKeyhole, Radio, Waves } from "lucide-react";
import type { DemoSession, HistorySummary, Project, ProjectProfile } from "@reweird/shared-types";
import { historyApi } from "@/lib/api";
import { ProbeCard, RuleRow, SignalChart } from "./page";

type Destination = "dashboard" | "profile" | "connect" | "diagnosis" | "guided" | "verify" | "history" | "reports" | "simulator";
type Props = {
  session: DemoSession;
  project: Project | null;
  profile: ProjectProfile | null;
  source: "api" | "browser";
  onNavigate: (view: Destination) => void;
};

export function Workbench({ session, project, profile, source, onNavigate }: Props) {
  const [recent, setRecent] = useState<HistorySummary[]>([]);
  const [historyState, setHistoryState] = useState<"loading" | "ready" | "offline">("loading");
  useEffect(() => {
    let active = true;
    historyApi.list().then(({ items }) => {
      if (active) { setRecent(items.slice(0, 3)); setHistoryState("ready"); }
    }).catch(() => { if (active) setHistoryState("offline"); });
    return () => { active = false; };
  }, []);
  const matchesProject = !project || session.profile_id === profile?.id;
  const failures = session.evidence.rule_results.filter((rule) => rule.status === "fail");
  const activeProbes = session.probes.filter((probe) => probe.status !== "idle");
  const charted = session.probes.filter((probe) => probe.samples?.length).slice(0, 2);
  const projectName = project?.name ?? profile?.project_name ?? session.project_name;
  const destinations: Destination[] = ["dashboard", "diagnosis", "guided", "verify"];
  const currentStep = { detect: 0, diagnose: 1, test: 2, repair: 2, verify: 3 }[session.stage];
  const confidence = Math.round(session.diagnosis.confidence * 100);

  return <div className="workbench">
    <div className="workbench-main">
      <section className="bench-welcome">
        <div className="welcome-copy"><span className="bench-label"></span><h1>Welcome to your<br />workbench.</h1><p>Understand your project.<br />Follow the evidence. Find your next move.</p><span className="welcome-foot"></span></div>
        <div className="welcome-photo" aria-hidden="true" />
        <span className="welcome-caption" aria-hidden="true"></span>
      </section>

      <section className="bench-stats" aria-label="Current project summary">
        <div><Activity size={17} /><strong>{matchesProject ? activeProbes.length : "—"}<small>Active probes</small></strong></div>
        <div><Waves size={17} /><strong>{matchesProject && session.raw_telemetry ? `${session.raw_telemetry.window_ms / 1000}s` : "—"}<small>Capture window</small></strong></div>
        <div className={matchesProject && failures.length ? "stat-attention" : ""}><Radio size={17} /><strong>{matchesProject ? failures.length : "—"}<small>Failed checks</small></strong></div>
        <div><LockKeyhole size={17} /><strong>Locked<small>PATCH output</small></strong></div>
      </section>

      <section className="bench-panel bench-signals">
        <div className="bench-panel-head"><div><span className="bench-label">01 / Observe</span><h2>Signal monitor</h2></div></div>
        {matchesProject ? <>
          <div className="probe-grid">{session.probes.map((probe) => <ProbeCard reading={probe} key={probe.probe} />)}</div>
          <div className="two-column wide-left">
            <div className="panel"><div className="panel-heading"><div><span className="eyebrow">Raw activity buckets</span><h2>Signal activity</h2></div><div className="chart-legend">{charted.map((probe) => <span key={probe.probe}>{probe.probe} {probe.role}</span>)}</div></div><SignalChart session={session} /></div>
            <div className="panel"><div className="panel-heading"><div><span className="eyebrow">Analysis</span><h2>Rule engine</h2></div></div><div className="rules-list">{session.evidence.rule_results.map((rule) => <RuleRow rule={rule} key={rule.id} />)}</div><div className="engine-note"><Bolt size={16} /><span>Deterministic checks run before any AI interpretation.</span></div></div>
          </div>
        </> : <div className="bench-empty"><Radio size={24} /><h3>Waiting for this project’s signals</h3><p>Complete probe setup and connect a matching telemetry source.</p><button className="secondary" onClick={() => onNavigate("connect")}>Open probe setup <ChevronRight size={15} /></button></div>}
      </section>

      <section className="bench-panel bench-flow">
        <div className="bench-panel-head"><div><span className="bench-label">The diagnostic process</span><h2>Every step, backed by evidence.</h2></div></div>
        <div className="bench-steps">{session.timeline.map((step, index) => <button key={step.id} className={matchesProject && index === currentStep ? "current" : ""} onClick={() => onNavigate(matchesProject ? destinations[index] ?? "dashboard" : "connect")}><span className="bench-step-number">{matchesProject && step.complete ? <Check size={14} /> : `0${index + 1}`}</span><strong>{step.label}</strong><small>{matchesProject ? step.detail : "Awaiting project capture"}</small></button>)}</div>
      </section>
    </div>

    <div className="workbench-rail">
      <section className="bench-panel bench-analysis">
        <div className="bench-panel-head"><div><span className="bench-label">02 / Understand</span><h2>Latest analysis</h2></div><span className="analysis-source">{!matchesProject ? "Pending" : source === "browser" ? "Demo" : session.telemetry_mode === "serial" ? "Serial" : "Simulated"}</span></div>
        {matchesProject ? <>
          <div className="analysis-headline"><span className={`analysis-mark ${failures.length ? "attention" : ""}`}><Activity size={21} /></span><div><h3>{session.diagnosis.headline}</h3><span className="analysis-confidence">{confidence}% confidence <span>· interpretation</span></span></div></div>
          <p className="analysis-summary">{session.diagnosis.summary}</p>
          <div className="hypothesis-heading"><span className="bench-label">Possible causes</span><small>Not yet confirmed</small></div>
          <ol className="bench-hypotheses">{session.diagnosis.possible_causes.slice(0, 3).map((cause, index) => <li key={cause}><span>{String(index + 1).padStart(2, "0")}</span><p>{cause}</p><ChevronRight size={13} /></li>)}</ol>
          <div className="bench-next-test"><span className="bench-label">Next diagnostic test</span><p>{session.diagnosis.next_test}</p><button className="text-button" onClick={() => onNavigate("guided")}>Open test planner <ArrowUpRight size={15} /></button></div>
          <div className="bench-analysis-actions"><button className="secondary" onClick={() => onNavigate("diagnosis")}>Review evidence</button><button className="primary" onClick={() => onNavigate("simulator")}>Simulator <ArrowUpRight size={15} /></button></div>
        </> : <div className="bench-empty"><Activity size={24} /><h3>Ready when your project is.</h3><p>A diagnosis will appear after a matching capture is available.</p></div>}
      </section>

      <section className="bench-panel bench-project">
        <div className="bench-panel-head"><h2>On your bench</h2><span className="bench-label">{project ? "Project" : "Built-in demo"}</span></div>
        <button className="bench-project-link" onClick={() => onNavigate("profile")}><span className="project-thumbnail"><Cpu size={28} strokeWidth={1.25} /></span><span><strong>{projectName}</strong><small>{profile?.controller ?? project?.controller ?? "Controller unspecified"} · {profile?.logic_voltage ?? project?.logic_voltage ?? "—"} V logic</small></span><ChevronRight size={16} /></button>
        <div className="bench-project-meta"><span>{profile?.components.length ?? 0} components</span><span>{profile?.confirmed ? "Profile confirmed" : "Review profile"}</span></div>
      </section>

      <section className="bench-panel bench-history">
        <div className="bench-panel-head"><h2>Recent sessions</h2><button className="text-button" onClick={() => onNavigate("history")} aria-label="View diagnostic history"><ArrowUpRight size={17} /></button></div>
        {recent.length > 0 ? recent.map((item) => <button className="bench-history-row" key={item.id} onClick={() => onNavigate("history")}><History size={16} /><span><strong>{item.project_name}</strong><small>{item.status.replaceAll("_", " ").toLowerCase()} · {item.telemetry_source || "No capture"}</small></span><ChevronRight size={14} /></button>) : <div className="bench-history-empty"><History size={19} /><p>{historyState === "loading" ? "Loading saved sessions…" : historyState === "offline" ? "Connect the API to browse your saved diagnostic sessions." : "Your first guided test starts your diagnostic history."}</p></div>}
        <button className="bench-report-link" onClick={() => onNavigate("reports")}><FileText size={15} /><span>Diagnostic reports</span><ArrowDownToLine size={14} /></button>
      </section>
      <p className="bench-safety"><LockKeyhole size={13} /> Passive sensing. Electrical output stays locked.</p>
    </div>
  </div>;
}
