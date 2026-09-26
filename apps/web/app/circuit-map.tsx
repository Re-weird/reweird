"use client";

import { useState } from "react";
import { Activity, ArrowUpRight, Cable, CircleAlert, Cpu, History, Layers3, Radio } from "lucide-react";
import type { DemoSession, ProbePlan, ProjectProfile } from "@reweird/shared-types";
import { buildCircuitMap, type CircuitMapConnection } from "@/lib/circuit-map";

type Destination = "connect" | "live" | "diagnosis" | "guided" | "history";

function signalLabel(item: CircuitMapConnection, hasCapture: boolean): string {
  if (!item.instruction) return "No probe assigned";
  if (!hasCapture) return "Awaiting capture";
  if (!item.reading) return "No reading";
  if (item.rules.some((rule) => rule.status === "fail")) return "Failed check";
  if (item.rules.some((rule) => rule.status === "warn")) return "Check warning";
  if (item.reading.status === "intermittent") return "Intermittent";
  if (item.reading.status === "idle") return "No activity";
  return item.reading.status === "stable" ? "Stable reading" : "Activity observed";
}

function signalTone(item: CircuitMapConnection, hasCapture: boolean): string {
  if (!item.reading || !hasCapture) return "waiting";
  if (item.rules.some((rule) => rule.status === "fail") || item.reading.status === "intermittent") return "fail";
  if (item.rules.some((rule) => rule.status === "warn") || item.reading.status === "idle") return "warn";
  return "observed";
}

function measuredValue(item: CircuitMapConnection): string {
  if (item.reading?.value == null) return "No numeric reading";
  return `${Number(item.reading.value.toFixed(2))} ${item.reading.unit}`.trim();
}

export function CircuitMap({
  profile,
  plan,
  session,
  demoMode,
  onNavigate,
}: {
  profile: ProjectProfile;
  plan: ProbePlan | null;
  session: DemoSession | null;
  demoMode: boolean;
  onNavigate: (destination: Destination) => void;
}) {
  const [selectedID, setSelectedID] = useState<string | null>(null);
  const model = buildCircuitMap(profile, plan, session, demoMode);
  const connections = model.groups.flatMap((group) => group.connections);
  const selected = connections.find((item) => item.connection.id === selectedID) ?? connections[0];
  const captureDescription = model.hasMatchingCapture
    ? model.captureKind === "serial" ? "ESP32 serial capture" : "Simulated API capture"
    : profile.confirmed ? model.planConnected || demoMode ? "No matching capture" : "Probe setup not confirmed" : "Draft profile";

  return <section className="panel circuit-map" aria-labelledby="circuit-map-title">
    <div className="panel-heading circuit-map-heading">
      <div><span className="eyebrow">Profile-driven topology</span><h2 id="circuit-map-title">Circuit map</h2><p>Connections from the {profile.confirmed ? "confirmed" : "draft"} profile, not a verified photo of the wiring.</p></div>
      <span className={`map-capture ${model.hasMatchingCapture ? "has-capture" : ""}`}><Radio size={13} /> {captureDescription}</span>
    </div>
    {model.groups.length === 0 ? <div className="inline-empty">Add components and connections to see a circuit map.</div> : <>
      <div className="circuit-map-board">
        <div className="map-controller"><Cpu size={24} /><span>Controller</span><strong>{profile.controller}</strong><small>{profile.logic_voltage} V logic</small></div>
        <div className="map-groups">
          {model.groups.map((group) => <div className="map-component" key={group.key}>
            <div className="map-component-head"><Layers3 size={16} /><div><strong>{group.name}</strong><small>{group.connections.length} profile connection{group.connections.length === 1 ? "" : "s"}</small></div></div>
            {group.connections.length === 0 ? <p className="map-unlinked">No connection defined in this profile.</p> : <div className="map-wires">
              {group.connections.map((item) => <button
                type="button"
                key={item.connection.id}
                className={`map-wire ${signalTone(item, model.hasMatchingCapture)} ${selected?.connection.id === item.connection.id ? "selected" : ""}`}
                aria-pressed={selected?.connection.id === item.connection.id}
                aria-label={`${group.name} ${item.connection.role}: ${item.connection.target}. ${item.instruction?.probe ?? "No probe assigned"}. ${signalLabel(item, model.hasMatchingCapture)}.`}
                onClick={() => setSelectedID(item.connection.id)}
              >
                <span className="map-wire-pin">{item.connection.gpio != null ? `GPIO${item.connection.gpio}` : "Profile node"}</span>
                <span className="map-wire-trace" aria-hidden="true" />
                <span className="map-wire-probe">{item.instruction?.probe ?? "—"}</span>
                <span className="map-wire-role">{item.connection.role}</span>
                <span className="map-wire-state">{signalLabel(item, model.hasMatchingCapture)}</span>
              </button>)}
            </div>}
          </div>)}
        </div>
      </div>
      {selected && <div className="map-detail" aria-live="polite">
        <div className="map-detail-top"><div><span className="eyebrow">Selected connection</span><h3>{selected.connection.component_name} · {selected.connection.role}</h3></div><span className={`map-status ${signalTone(selected, model.hasMatchingCapture)}`}>{signalLabel(selected, model.hasMatchingCapture)}</span></div>
        <div className="map-detail-facts">
          <div><small>Profile target</small><strong>{selected.connection.target}</strong></div>
          <div><small>Expected signal</small><strong>{selected.connection.expected.signal_type || selected.connection.behavior || "Not specified"}</strong></div>
          <div><small>Probe assignment</small><strong>{selected.instruction?.probe ?? "Not assigned"}</strong></div>
          <div><small>Observed value</small><strong>{model.hasMatchingCapture ? measuredValue(selected) : "Not measured"}</strong></div>
        </div>
        {selected.instruction?.safe_warning && <p className="map-safety"><CircleAlert size={14} />{selected.instruction.safe_warning}</p>}
        {selected.rules.length > 0 && <div className="map-rule-list"><small>Checks for this probe</small>{selected.rules.map((rule) => <span key={rule.id} className={rule.status}>{rule.message}</span>)}</div>}
        <div className="map-actions">
          {model.hasMatchingCapture ? <>
            <button className="secondary" onClick={() => onNavigate("live")}><Activity size={14} /> Inspect signals</button>
            <button className="secondary" onClick={() => onNavigate("diagnosis")}>Review evidence <ArrowUpRight size={14} /></button>
            <button className="secondary" onClick={() => onNavigate("guided")}>Next test <ArrowUpRight size={14} /></button>
            <button className="text-button" onClick={() => onNavigate("history")}><History size={14} /> History</button>
          </> : profile.confirmed && !demoMode && <button className="secondary" onClick={() => onNavigate("connect")}><Cable size={14} /> Open probe setup</button>}
        </div>
      </div>}
      <p className="map-disclaimer">GPIO labels identify controller assignments; other nodes are not assumed to originate at the controller. A profile connection describes intended wiring, and measured status only describes a matching captured probe. Neither visually verifies a physical wire or proves component health.</p>
    </>}
  </section>;
}
