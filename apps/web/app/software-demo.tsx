"use client";

import { useEffect, useMemo, useState } from "react";
import Link from "next/link";
import { ArrowLeft, Download, Moon, RotateCcw, Sun } from "lucide-react";
import type { DiagnosticWorkflow, ProjectProfile } from "@reweird/shared-types";
import { TooltipProvider } from "@/components/ui/tooltip";
import { useTheme } from "@/lib/theme";
import { BASELINE, checks, verifies, type Snapshot } from "@/lib/judge-demo";
import { measurement, planWorkflow, recommendationFor, scenarios, snapshotFor, softwarePlan, softwareProfile, softwareSession, summaryFor, testSnapshot, type ScenarioID } from "@/lib/software-demo";
import type { ProjectTabID } from "@/lib/project-routes";
import { Workbench } from "./workbench";
import { ProjectOverviewView } from "./project-overview";
import { ProbePlanView } from "./project-workflow";
import { DiagnosisView, SimulatorView } from "./project-views";
import { GuidedTestView } from "./guided-test";
import { CircuitMap } from "./circuit-map";

const tabs: { id: ProjectTabID; label: string }[] = [
  { id: "workbench", label: "Workbench" }, { id: "overview", label: "Overview" },
  { id: "probe-setup", label: "Probe setup" }, { id: "simulator", label: "Simulator" },
  { id: "diagnosis", label: "Diagnosis" }, { id: "next-test", label: "Next test" },
  { id: "verify", label: "Verify result" }, { id: "health", label: "Device Passport" },
  { id: "physical-history", label: "Physical History" }, { id: "history", label: "History" },
  { id: "reports", label: "Reports" }, { id: "computer", label: "Computer checks" },
];
type Event = { at: number; label: string; values: Snapshot };
const validScenario = (id: string): id is ScenarioID => scenarios.some(item => item.id === id);

export function SoftwareDemo() {
  const [theme, toggleTheme] = useTheme();
  const [tab, setTab] = useState<ProjectTabID>("workbench");
  const [scenario, setScenario] = useState<ScenarioID>("loose-echo");
  const [selected, setSelected] = useState<string>("loose-echo");
  const [snapshot, setSnapshot] = useState<Snapshot>(() => snapshotFor("loose-echo"));
  const [phase, setPhase] = useState<"diagnose" | "test" | "verify">("diagnose");
  const [sequence, setSequence] = useState(1);
  const [profile, setProfile] = useState<ProjectProfile>(softwareProfile);
  const [plan, setPlan] = useState(softwarePlan);
  const [workflow, setWorkflow] = useState<DiagnosticWorkflow | null>(null);
  const [records, setRecords] = useState<DiagnosticWorkflow[]>([]);
  const [events, setEvents] = useState<Event[]>([]);
  const [baselineSaved, setBaselineSaved] = useState(false);
  const [notice, setNotice] = useState("");
  const [computerCase, setComputerCase] = useState("port");
  const [computerRan, setComputerRan] = useState(false);
  const session = useMemo(() => softwareSession(snapshot, scenario, phase, sequence), [snapshot, scenario, phase, sequence]);
  const history = useMemo(() => records.map(summaryFor).reverse(), [records]);
  const recommendation = recommendationFor(scenario);

  useEffect(() => {
    const readHash = () => { const id = window.location.hash.slice(1); if (tabs.some(item => item.id === id)) setTab(id as ProjectTabID); };
    readHash(); window.addEventListener("hashchange", readHash);
    return () => window.removeEventListener("hashchange", readHash);
  }, []);
  function navigate(next: ProjectTabID) { setTab(next); window.history.replaceState(null, "", `#${next}`); window.scrollTo({ top: 0, behavior: "smooth" }); }
  function log(label: string, values = snapshot) { setEvents(items => [...items, { at: Date.now(), label, values }]); }
  function saveWorkflow(next: DiagnosticWorkflow) { setWorkflow(next); setRecords(items => [...items.filter(item => item.id !== next.id), next]); }
  function loadScenario(id: string) {
    if (!validScenario(id)) return;
    const next = snapshotFor(id);
    setScenario(id); setSelected(id); setSnapshot(next); setPhase("diagnose"); setSequence(n => n + 1); setWorkflow(null);
    log(`Loaded simulated scenario: ${scenarios.find(item => item.id === id)?.name}`, next);
    setNotice("Simulated values loaded. The workbench, map, diagnosis, and reports now use this capture.");
  }
  function createPlan() { saveWorkflow(planWorkflow(scenario)); setNotice("Guided test plan created locally."); }
  function startTest() {
    const next = workflow ?? planWorkflow(scenario);
    saveWorkflow({ ...next, status: "WAITING_FOR_USER", baseline: measurement(session), updated_at_ms: Date.now() });
    log("Captured before window");
  }
  function captureTest() {
    const next = workflow ?? planWorkflow(scenario);
    const test = testSnapshot(scenario);
    const during = softwareSession(test.snapshot, scenario, "test", sequence + 1);
    setSnapshot(test.snapshot); setPhase("test"); setSequence(n => n + 1);
    saveWorkflow({ ...next, status: "COMPLETED", baseline: next.baseline ?? measurement(session), during: measurement(during), updated_at_ms: Date.now(),
      result: { test_id: next.id, test_type: next.plan.recommendation.test_type, target_probes: next.plan.recommendation.target_probes, observations: [
        { probe: "P1", metric: "minimum_rail_voltage", value: test.snapshot.power.min_v, unit: "V", provenance: "GUIDED_TEST" },
        { probe: "P2", metric: "trigger_width", value: test.snapshot.trig.width_us, unit: "µs", provenance: "GUIDED_TEST" },
        { probe: "P3", metric: "echo_dropouts", value: test.snapshot.echo.dropouts_per_min, unit: "/min", provenance: "GUIDED_TEST" },
      ], derived_metrics: { capture_source: "SIMULATED" }, result: scenario === "loose-echo" ? "POSITIVE_CORRELATION" : scenario === "unstable-power" ? "RAIL_OUTSIDE_TOLERANCE" : scenario === "timing-drift" ? "TIMING_OUTSIDE_SPECIFICATION" : scenario === "missing-echo" ? "EXPECTED_ACTIVITY_MISSING" : "HEALTHY", interpretation: test.finding, confidence: .92, evidence_provenance: ["GUIDED_TEST"], timestamp_ms: Date.now() } });
    log(`Guided test: ${test.finding}`, test.snapshot);
  }
  function repairAndVerify() {
    const next = workflow ?? planWorkflow(scenario);
    const after = softwareSession(BASELINE, scenario, "verify", sequence + 1);
    const before = next.baseline ?? measurement(session);
    const changes = [
      { probe: "P1", metric: "minimum_rail_voltage", before: snapshotFor(scenario).power.min_v, after: BASELINE.power.min_v, unit: "V" },
      { probe: "P2", metric: "trigger_width", before: snapshotFor(scenario).trig.width_us, after: BASELINE.trig.width_us, unit: "µs" },
      { probe: "P3", metric: "echo_dropouts", before: snapshotFor(scenario).echo.dropouts_per_min, after: 0, unit: "/min" },
      { probe: "P3", metric: "echo_rate", before: snapshotFor(scenario).echo.rate_hz, after: BASELINE.echo.rate_hz, unit: "replies/s" },
    ];
    saveWorkflow({ ...next, status: "RESOLVED", baseline: before, during: next.during ?? measurement(session), after: measurement(after), updated_at_ms: Date.now(), verification: {
      status: "RESOLVED", changes, improvements: changes.filter(change => change.before !== change.after), remaining_issues: [], summary: "The simulated correction restored all four checks to the healthy baseline. No physical hardware was changed.", before_window_id: before.id, after_window_id: after.measurement_id!, timestamp_ms: Date.now(),
    } });
    setSnapshot(BASELINE); setPhase("verify"); setSequence(n => n + 1); log("Simulated correction applied; VERIFY passed all four checks", BASELINE); navigate("verify");
  }
  function download(format: "json" | "md") {
    const report = { simulated: true, project: profile, capture: session, workflow, history: records, events, known_good_saved: baselineSaved };
    const markdown = `# ReWeird simulated diagnostic report\n\nSource: SIMULATED browser values. No physical measurements or live AI calls.\n\n## Result\n${session.diagnosis.headline}\n${session.diagnosis.summary}\n\n## Checks\n${session.evidence.rule_results.map(rule => `- ${rule.status.toUpperCase()}: ${rule.message}`).join("\n")}\n\n## Timeline\n${events.map(event => `- ${new Date(event.at).toISOString()}: ${event.label}`).join("\n")}\n`;
    const blob = new Blob([format === "json" ? JSON.stringify(report, null, 2) : markdown], { type: format === "json" ? "application/json" : "text/markdown" });
    const url = URL.createObjectURL(blob); const a = document.createElement("a"); a.href = url; a.download = `reweird-simulated-report.${format}`; document.body.appendChild(a); a.click(); a.remove(); window.setTimeout(() => URL.revokeObjectURL(url), 10000);
    setNotice("Your simulated report was generated for download.");
  }
  const guided = <GuidedTestView workflow={workflow} recommendation={recommendation} busy={false} error={null} onPlan={createPlan} onStart={startTest} onCapture={captureTest} onRemeasure={repairAndVerify} onCancel={() => { if (workflow) saveWorkflow({ ...workflow, status: "CANCELLED" }); log("Guided test cancelled"); }} onRecordAction={async description => { if (workflow) saveWorkflow({ ...workflow, user_actions: [...workflow.user_actions ?? [], { id: String(Date.now()), description, timestamp_ms: Date.now() }] }); log(`User note: ${description}`); }} onHealth={() => navigate("health")} />;
  return <TooltipProvider><div className="software-demo">
    <div className="sd-banner"><strong>SIMULATED PROJECT DEMO</strong><span>Actual ReWeird interface · generated readings · no sign-in or hardware</span><Link href="/try">Try it yourself ↗</Link></div>
    <header className="sd-header"><Link href="/" className="sd-brand"><img src={`${process.env.NEXT_PUBLIC_DEMO_BASE_PATH ?? ""}/images/reweird-logo-mark.png`} alt="" width={52} height={26} />ReWeird</Link><span>Ultrasonic Distance Sensor <small>ESP32 + HC-SR04</small></span><div><Link href="/demo"><ArrowLeft size={14} /> Demo choices</Link><button onClick={() => { loadScenario("loose-echo"); navigate("workbench"); }}><RotateCcw size={14} /> Reset scenario</button><button onClick={toggleTheme} aria-label={`Switch to ${theme === "dark" ? "light" : "dark"} mode`}>{theme === "dark" ? <Sun size={17} /> : <Moon size={17} />}</button></div></header>
    <nav className="sd-tabs" aria-label="Project sections">{tabs.map(item => <button key={item.id} aria-current={tab === item.id ? "page" : undefined} onClick={() => navigate(item.id)}>{item.label}</button>)}</nav>
    <div className="sd-controls"><label htmlFor="demo-fault">Simulated input</label><select id="demo-fault" value={selected} onChange={event => setSelected(event.target.value)}>{scenarios.map(item => <option key={item.id} value={item.id}>{item.name}</option>)}</select><button className="secondary" onClick={() => loadScenario(selected)}>Load simulated values</button><button className="primary" onClick={() => { createPlan(); navigate("next-test"); }}>Run guided diagnosis</button></div>
    {notice && <p className="sd-notice" role="status">{notice}</p>}
    <main className="sd-main">
      {tab === "workbench" && <Workbench session={session} project={null} profile={profile} source="browser" onNavigate={navigate} historyProjectID="ultrasonic-demo" demoHistory={history} plan={plan} scenarios={scenarios} busy={false} onRunScenario={async id => loadScenario(id)} onBrowserDemo={async () => captureTest()} />}
      {tab === "overview" && <ProjectOverviewView project={null} profile={profile} plan={plan} session={session} onSave={async next => { setProfile(next); return next; }} onConfirm={async next => { setProfile({ ...next, confirmed: true }); log("Confirmed simulated project profile"); }} onSync={async () => { setNotice("This demo uses the built-in example code; no repository connection is required."); }} onNavigate={view => navigate(({ connect: "probe-setup", live: "workbench", diagnosis: "diagnosis", guided: "next-test", history: "history" } as const)[view])} />}
      {tab === "probe-setup" && <ProbePlanView simulated project={null} profile={profile} plan={plan} session={session} onConnected={async () => { setPlan({ ...plan, connected: true }); log("Confirmed simulated probe setup"); navigate("workbench"); }} />}
      {tab === "simulator" && <SimulatorView session={session} source="browser" scenarios={scenarios} selected={selected} error={null} setSelected={setSelected} mysteryPending={false} onRevealMystery={() => {}} onRun={() => loadScenario(selected)} onDemoTest={captureTest} onDemoRepair={repairAndVerify} busy={false} />}
      {tab === "diagnosis" && <DiagnosisView session={session} source="browser" busy={false} onPlan={() => { createPlan(); navigate("next-test"); }} />}
      {(tab === "next-test" || tab === "verify") && guided}
      {tab === "health" && <><section className="page-heading"><div><p className="kicker">Device Passport · SIMULATED</p><h1>{verifies(snapshot) ? "Simulated baseline match" : "Simulated deviation detected"}</h1><p>Compare the current capture against the built-in healthy values. This is not physical device certification.</p></div></section><CircuitMap profile={profile} plan={plan} session={session} demoMode onNavigate={view => navigate(view === "guided" ? "next-test" : view === "live" ? "workbench" : view === "connect" ? "probe-setup" : view)} /><section className="panel sd-record"><h2>Known Good</h2><p>{baselineSaved ? "Healthy simulated capture saved for this browser session." : "A simulated reference is provided. Save your own healthy capture after VERIFY."}</p><button className="primary" disabled={!verifies(snapshot)} onClick={() => { setBaselineSaved(true); log("Saved a SIMULATED Known Good capture"); }}>Save simulated Known Good</button><div className="rules-list">{checks(snapshot).map(rule => <p key={rule.id}>{rule.status.toUpperCase()} · {rule.message}</p>)}</div></section></>}
      {(tab === "history" || tab === "physical-history") && <><section className="page-heading"><div><p className="kicker">{tab === "history" ? "Diagnostic history" : "Physical History"} · SIMULATED</p><h1>{tab === "history" ? "The investigation, recorded" : "Circuit snapshots"}</h1><p>These records stay in this browser session and can be included in your downloaded report.</p></div><button className="secondary" onClick={() => { log("Manual simulated circuit snapshot"); setNotice("Snapshot added below."); }}>Capture snapshot</button></section><section className="panel sd-record">{events.length === 0 ? <p>Load a scenario or run a guided test to create your first record.</p> : <ol className="sd-timeline">{events.map((event, index) => <li key={`${event.at}-${index}`}><time>{new Date(event.at).toLocaleTimeString()}</time><div><strong>{event.label}</strong><p>P1 {event.values.power.mean_v.toFixed(2)} V · TRIG {event.values.trig.width_us} µs · ECHO {event.values.echo.rate_hz} replies/s · {event.values.echo.dropouts_per_min} dropouts/min</p></div></li>)}</ol>}</section></>}
      {tab === "reports" && <><section className="page-heading"><div><p className="kicker">Diagnostic report · SIMULATED</p><h1>{session.diagnosis.headline}</h1><p>{session.diagnosis.summary}</p></div><div className="sd-actions"><button className="primary" onClick={() => download("json")}><Download size={15} /> Download JSON</button><button className="secondary" onClick={() => download("md")}>Download Markdown</button></div></section><section className="panel sd-record"><h2>Evidence and verification</h2><p>Source: browser simulation. Interpretation: deterministic rules; no live LLM request.</p>{session.evidence.rule_results.map(rule => <p key={rule.id}>{rule.status.toUpperCase()} · {rule.message}</p>)}<h3>Result</h3><p>{workflow?.verification?.summary ?? "Repair has not been verified in a guided workflow yet."}</p><h3>Recorded actions</h3><p>{events.length} events · {records.length} guided investigations</p></section></>}
      {tab === "computer" && <><section className="page-heading"><div><p className="kicker">Computer checks · SIMULATED</p><h1>Check the software side</h1><p>Example host readings are separate from electrical measurements. Nothing scans or changes your computer.</p></div></section><section className="panel sd-record"><label htmlFor="computer-case">Example issue</label><select id="computer-case" value={computerCase} onChange={event => { setComputerCase(event.target.value); setComputerRan(false); }}><option value="port">Port already in use</option><option value="memory">High memory usage</option><option value="healthy">Healthy host</option></select><button className="primary" onClick={() => { setComputerRan(true); log(`Ran simulated computer check: ${computerCase}`); }}>Run simulated computer check</button>{computerRan && <div role="status"><h2>{computerCase === "port" ? "Port 8080 is occupied" : computerCase === "memory" ? "Memory pressure detected" : "Host checks passed"}</h2><p>{computerCase === "port" ? "Simulated process: node, PID 4242, listening on 8080. Suggested next step: identify the process or configure a different port. No process was stopped." : computerCase === "memory" ? "Simulated usage: 7.6 GB of 8 GB (95%). Suggested next step: review running processes. No application was closed." : "Simulated CPU 14%, memory 3.2 GB of 8 GB, and expected service port available."}</p></div>}</section></>}
    </main><footer className="sd-footer">SIMULATED browser session · The real ReWeird views use generated values here. Hardware actions and external services are disabled.</footer>
  </div></TooltipProvider>;
}
