"use client";

import { Activity, Cable, ChevronRight, Microscope, RefreshCw, ShieldCheck, TestTube2, TriangleAlert, CheckCircle2 } from "lucide-react";
import type { DemoSession, ProbePlan, Project, ProjectProfile, SimulatorScenario } from "@reweird/shared-types";
import { ConfidenceRing, RuleRow } from "./signal-components";
import { telemetryLabel } from "@/lib/weird-demo";

export function DiagnosisView({ session, source, onPlan, busy }: { session: DemoSession; source: "api" | "browser"; onPlan: () => void; busy: boolean }) {
  const tested = session.stage === "test" || session.stage === "repair";
  const focus = session.probes.find((probe) => probe.probe === session.evidence.probe);
  const expected = Object.entries(session.evidence.expected).slice(0, 3);
  const observed = Object.entries(session.evidence.observed).slice(0, 3);
  const baseline = Object.entries(session.evidence.baseline).slice(0, 3);
  const renderFacts = (facts: [string, unknown][]) => facts.map(([key, value]) => <small key={key}>{key.replaceAll("_", " ")}: {String(value)}</small>);
  const failed = session.evidence.rule_results.filter((rule) => rule.status === "fail");
  const uncertain = !failed.length && (Boolean(session.evidence.unresolved_questions?.length) || session.evidence.rule_results.some((rule) => rule.status === "warn"));
  return (
    <>
      <section className={`weird-result-banner ${failed.length ? "suspect" : ""}`} aria-live="polite"><span className="eyebrow">{telemetryLabel(session, source)} · evidence-led detection</span><h2>{failed.length ? "SOMETHING’S WEIRD." : uncertain ? "STILL WEIRD." : "SUSPICIOUSLY NORMAL."}</h2><p>{failed.length ? `${session.evidence.probe} · ${session.evidence.role}: ${failed.map((rule) => rule.message).join(" ")}` : uncertain ? "Another measurement is needed before narrowing this down." : "No failed deterministic checks in this capture."}</p></section>
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
          <div className="interpretation-label"><Microscope size={16} /> PROBE interpretation <span>{telemetryLabel(session, source)}</span></div>
          <h2>{session.diagnosis.summary}</h2>
          <p>Possible causes, ranked but not asserted as measured truth:</p>
          <ol>{session.diagnosis.possible_causes.map((cause, index) => <li key={cause}><span>{index + 1}</span>{cause}</li>)}</ol>
        </div>
      </section>
      <section className="test-callout">
        <div className="test-icon"><TestTube2 size={24} /></div>
        <div><span className="eyebrow">I have a theory. Let’s test it.</span><h2>{session.diagnosis.next_test}</h2><p>{tested ? "Review the actual test evidence before treating a cause as confirmed." : "This test is user-guided and only monitors input. PATCH output remains disabled."}</p></div>
        <button className="primary" onClick={onPlan} disabled={busy}>{busy ? <RefreshCw className="spin" size={17} /> : <Activity size={17} />} Open guided test planner</button>
      </section>
    </>
  );
}

export function SimulatorView({
  session,
  source,
  scenarios,
  selected,
  setSelected,
  mysteryPending,
  onRevealMystery,
  onRun,
  onPlan,
  onDemoTest,
  onDemoRepair,
  busy,
}: {
  session: DemoSession;
  source: "api" | "browser";
  scenarios: SimulatorScenario[];
  selected: string;
  setSelected: (id: string) => void;
  mysteryPending: boolean;
  onRevealMystery: () => void;
  onRun: () => void;
  onPlan: () => void;
  onDemoTest: () => void;
  onDemoRepair: () => void;
  busy: boolean;
}) {
  const activeScenario = scenarios.find((scenario) => scenario.id === session.scenario_id);
  return (
    <>
      <section className="page-heading"><div><p className="kicker">{telemetryLabel(session, source)} · Layers 3–4 software harness</p><h1>Raw telemetry fault simulator</h1><p>Each API case emits bounded electrical samples through validation, normalization, profile matching, storage, analysis, and diagnosis. Browser fallback uses an illustrative in-memory fixture.</p></div><div className="live-badge"><span /> PATCH locked</div></section>
      <section className="pipeline-strip" aria-label="Telemetry processing pipeline">
        {["Raw samples", "Validate", "Normalize", "Match profile", "Store window", "Signal analysis", "Evidence", "Diagnosis"].map((step, index) => <div key={step}><span>{index + 1}</span>{step}</div>)}
      </section>
      <section className="simulator-layout">
        <div className="panel scenario-panel">
          <div className="panel-heading"><div><span className="eyebrow">Simulated faults</span><h2>Select a deterministic input</h2></div></div>
          {mysteryPending && <div className="weird-result-banner"><span className="eyebrow">Mystery simulation</span><h2>The fault is still hidden.</h2><p>Read the measurement and diagnosis first, then reveal which existing scenario supplied these samples.</p><button className="secondary" onClick={onRevealMystery}>Reveal selected scenario</button></div>}
          {!mysteryPending && <div className="scenario-list">
            {scenarios.map((scenario) => (
              <label className={selected === scenario.id ? "selected" : ""} key={scenario.id}>
                <input type="radio" name="scenario" value={scenario.id} checked={selected === scenario.id} onChange={() => setSelected(scenario.id)} />
                <span><strong>{scenario.name}</strong><small>{scenario.description}</small></span>
              </label>
            ))}
          </div>}
          {!mysteryPending && <button className="primary full" disabled={busy || !selected} onClick={onRun}>{busy ? <RefreshCw className="spin" size={17} /> : <TestTube2 size={17} />} Load raw samples and analyze</button>}
        </div>
        <div className="panel simulator-result">
          <div className="panel-heading"><div><span className="eyebrow">Current result</span><h2>{session.diagnosis.headline}</h2></div><span className="session-id">#{session.measurement_id ?? "memory"}</span></div>
          <p>{session.stage === "verify" ? "VERIFY captured a fresh healthy window and compared it with the original fault evidence." : mysteryPending ? "This hidden scenario was analyzed from simulated raw samples. Inspect the evidence before revealing the scenario." : activeScenario?.description ?? "Select a scenario to run it through the backend pipeline."}</p>
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

export function LegacyDemoVerifyView({ session, onReset, busy }: { session: DemoSession; onReset: () => void; busy: boolean }) {
  const verified = session.stage === "verify";
  return <>
    <section className="page-heading"><div><p className="kicker">Original demo · Browser fallback</p><h1>{verified ? "Demo signal returned to baseline" : "Demo verification is waiting"}</h1><p>This is the original simulated HC-SR04 loop, separate from persisted generic guided tests.</p></div>{verified && <div className="verified-seal"><CheckCircle2 size={24} /> DEMO VERIFIED</div>}</section>
    <section className={`verification-panel ${verified ? "ready" : "locked"}`}><div className="before-after"><div><span>Before demo repair</span><strong>{session.before.dropouts_per_minute}</strong><small>dropouts / minute</small></div><div className="delta-arrow"><ChevronRight size={26} /></div><div><span>After demo repair</span><strong>{session.after?.dropouts_per_minute ?? "—"}</strong><small>dropouts / minute</small></div></div><div className="verification-summary"><div><h2>{verified ? "Original demo correction simulated" : "No demo repair yet"}</h2><p>{verified ? session.diagnosis.summary : "Run the original demo test and repair from the Fault simulator."}</p></div></div></section>
    {verified && <button className="secondary center-button" onClick={onReset} disabled={busy}><RefreshCw size={16} /> Reset original demo</button>}
  </>;
}

export function ProjectLivePending({ project, profile, plan }: { project: Project; profile: ProjectProfile | null; plan: ProbePlan | null }) {
  return (
    <>
      <section className="page-heading"><div><p className="kicker">Live diagnostics · Project ready</p><h1>Waiting for matching telemetry</h1><p>{project.name} is confirmed and its probe plan is connected. Live cards will populate when the ReWeird device sends frames matching this profile.</p></div><div className="live-badge"><span /> Armed</div></section>
      <section className="panel live-pending-panel"><Cable size={34} /><div><h2>{profile?.project_name ?? project.name}</h2><p>The browser will not substitute HC-SR04 demo measurements for this project. Start the API with this profile and connect the configured ESP32 telemetry source.</p><div className="spec-chips"><span>Profile: {profile?.id}</span><span>{plan?.instructions.length ?? 0} placement steps</span><span>PATCH locked</span></div></div></section>
      <section className="security-note"><ShieldCheck size={20} /><div><strong>No fabricated measurements</strong><span>Only validated telemetry that matches the confirmed P1–P6 modes can enter the diagnostic engine.</span></div></section>
    </>
  );
}

export function DemoProbePlanView({ plan, onContinue }: { plan: ProbePlan | null; onContinue: () => void }) {
  return <section><div className="page-heading"><div><span className="eyebrow">BUILT-IN SIMULATOR</span><h1>Demo probe plan</h1><p>This illustrative plan is generated from the confirmed demo profile. No physical probe connection is claimed.</p></div></div>{plan ? <div className="probe-plan-grid">{plan.instructions.map((step) => <article className={`probe-instruction ${step.probe === "GND" ? "ground" : ""}`} key={step.probe}><div className="probe-badge">{step.probe}</div><div><span className="eyebrow">{step.role}</span><h2>{step.target}</h2><p>{step.expected} · {step.signal_type}</p><div className="safety-warning"><ShieldCheck size={13} />{step.safe_warning}</div></div></article>)}</div> : <div className="empty-state panel"><p>Start the API to generate the demo placement plan from the profile.</p></div>}<button className="primary" onClick={onContinue}>Continue to simulator <ChevronRight size={15} /></button></section>;
}
