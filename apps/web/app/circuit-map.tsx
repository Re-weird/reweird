"use client";

import { useState } from "react";
import { AnimatePresence, motion, useReducedMotion } from "framer-motion";
import { Activity, ArrowUpRight, Cable, CircleAlert, Cpu, History, Radio } from "lucide-react";
import type { DemoSession, ProbePlan, ProjectProfile } from "@reweird/shared-types";
import { Button } from "@/components/ui/button";
import { useAppState } from "@/lib/app-state";
import { buildCircuitMap, circuitDisplayState, type CircuitDisplayState, type CircuitMapConnection } from "@/lib/circuit-map";
import { cn } from "@/lib/utils";

type Destination = "connect" | "live" | "diagnosis" | "guided" | "history";

const EASE_OUT = [0.23, 1, 0.32, 1] as const;

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

function measuredValue(item: CircuitMapConnection): string {
  if (item.reading?.value == null) return "No numeric reading";
  return `${Number(item.reading.value.toFixed(2))} ${item.reading.unit}`.trim();
}

const tone: Record<CircuitDisplayState, { text: string; line: string; ring: string; dot: string }> = {
  waiting: { text: "text-subtle", line: "bg-border", ring: "ring-border", dot: "bg-subtle" },
  normal: { text: "text-pass", line: "bg-pass/40", ring: "ring-border", dot: "bg-pass" },
  recovered: { text: "text-pass", line: "bg-pass/40", ring: "ring-pass/40", dot: "bg-pass" },
  testing: { text: "text-warn", line: "bg-warn/40", ring: "ring-warn/40", dot: "bg-warn" },
  suspect: { text: "text-fail", line: "bg-fail/40", ring: "ring-fail/50", dot: "bg-fail" },
};

// A thin trace from the pin to the part. Live traces carry a travelling
// pulse (CSS, off the main thread); a suspect trace pulses intermittently.
function Trace({ state, live }: { state: CircuitDisplayState; live: boolean }) {
  const reduce = useReducedMotion();
  const colors = tone[state];
  const animate = live && !reduce && state !== "waiting";
  return (
    <span aria-hidden className="relative block h-3 flex-1 overflow-hidden">
      {state === "waiting" ? (
        <span className="absolute inset-x-0 top-1/2 h-px -translate-y-1/2" style={{ backgroundImage: "linear-gradient(90deg, var(--line) 50%, transparent 50%)", backgroundSize: "6px 1px" }} />
      ) : (
        <span className={cn("absolute inset-x-0 top-1/2 h-px -translate-y-1/2", colors.line)} />
      )}
      {animate && (
        <span className="absolute inset-0" style={{ animation: `trace-flow ${state === "suspect" ? "2.6s" : "1.8s"} cubic-bezier(0.45, 0, 0.55, 1) infinite`, animationDelay: state === "suspect" ? "0.9s" : "0s" }}>
          <span className={cn("absolute top-1/2 left-0 size-1.5 -translate-x-full -translate-y-1/2 rounded-full", colors.dot)} />
        </span>
      )}
    </span>
  );
}

export function CircuitMap({
  profile,
  plan,
  session,
  demoMode,
  onNavigate,
  showActions = true,
}: {
  profile: ProjectProfile;
  plan: ProbePlan | null;
  session: DemoSession | null;
  demoMode: boolean;
  onNavigate: (destination: Destination) => void;
  /** Links to the live tabs under the selected connection; pages that already show those tabs can hide them. */
  showActions?: boolean;
}) {
  const { workflow } = useAppState();
  const reduce = useReducedMotion();
  const [selectedID, setSelectedID] = useState<string | null>(null);
  const model = buildCircuitMap(profile, plan, session, demoMode);
  const connections = model.groups.flatMap((group) => group.connections);
  const selected = connections.find((item) => item.connection.id === selectedID) ?? connections[0];
  const captureDescription = model.hasMatchingCapture
    ? model.captureKind === "serial" ? "ESP32 serial capture" : "Simulated capture"
    : profile.confirmed ? model.planConnected || demoMode ? "No matching capture" : "Probe setup not confirmed" : "Draft profile";
  const displayState = (item: CircuitMapConnection) => circuitDisplayState(item, model.hasMatchingCapture, session, workflow);
  const displayLabel = (item: CircuitMapConnection) => {
    const state = displayState(item);
    return state === "recovered" ? "Recovered · VERIFY passed" : state === "testing" ? "Testing · guided plan" : state === "suspect" ? "Suspect" : state === "normal" ? "Normal" : signalLabel(item, model.hasMatchingCapture);
  };

  return (
    <section data-tw aria-labelledby="circuit-map-title" className="min-w-0">
      <div className="flex flex-wrap items-end justify-between gap-3">
        <div>
          <p className="font-mono text-[11px] tracking-[0.14em] text-subtle uppercase">Topology</p>
          <h2 id="circuit-map-title" className="mt-1 text-lg font-semibold tracking-tight text-foreground">Circuit map</h2>
        </div>
        <span className={cn("inline-flex items-center gap-1.5 rounded-full px-2.5 py-1 font-mono text-[11px] ring-1", model.hasMatchingCapture ? "text-pass ring-pass/40" : "text-muted-foreground ring-border")}>
          <Radio className="size-3" strokeWidth={1.5} /> {captureDescription}
        </span>
      </div>

      {model.groups.length === 0 ? (
        <p className="mt-4 rounded-lg border border-dashed border-border px-4 py-6 text-sm text-muted-foreground">Add components and connections to see a circuit map.</p>
      ) : (
        <>
          <div className="relative mt-4 grid grid-cols-1 gap-4 overflow-hidden rounded-xl bg-surface p-4 ring-1 ring-border md:grid-cols-[180px_minmax(0,1fr)] md:gap-0 md:p-5">
            <div aria-hidden className="pointer-events-none absolute inset-0 opacity-50 [background-image:radial-gradient(var(--line-soft)_1px,transparent_1px)] [background-size:16px_16px]" />
            <div className="relative flex flex-row items-center gap-3 self-center rounded-lg bg-background/80 p-4 ring-1 ring-border md:flex-col md:items-start md:gap-2">
              <span className="grid size-10 place-items-center rounded-md bg-surface-2 text-signal ring-1 ring-border"><Cpu className="size-5" strokeWidth={1.5} /></span>
              <div>
                <p className="font-mono text-[10px] tracking-[0.14em] text-subtle uppercase">Controller</p>
                <p className="text-base font-semibold text-foreground">{profile.controller}</p>
                <p className="font-mono text-[11px] text-muted-foreground">{profile.logic_voltage} V logic</p>
              </div>
            </div>

            <div className="relative space-y-4 md:pl-2">
              {model.groups.map((group) => (
                <div key={group.key}>
                  <p className="mb-2 flex items-baseline gap-2 pl-1 text-sm font-medium text-foreground">
                    {group.name}
                    <span className="font-mono text-[11px] font-normal text-subtle">{group.connections.length} connection{group.connections.length === 1 ? "" : "s"}</span>
                  </p>
                  {group.connections.length === 0 ? (
                    <p className="pl-1 text-xs text-muted-foreground">No connection defined in this profile.</p>
                  ) : (
                    <div className="space-y-1">
                      {group.connections.map((item) => {
                        const state = displayState(item);
                        const active = selected?.connection.id === item.connection.id;
                        return (
                          <button
                            type="button"
                            key={item.connection.id}
                            aria-pressed={active}
                            aria-label={`${group.name} ${item.connection.role}: ${item.connection.target}. ${item.instruction?.probe ?? "No probe assigned"}. ${displayLabel(item)}.`}
                            onClick={() => setSelectedID(item.connection.id)}
                            className={cn(
                              "flex w-full items-center gap-3 rounded-md px-2 py-2 text-left ring-1 transition-[background-color,box-shadow,transform] duration-150 ease-out active:scale-[0.99]",
                              active ? cn("bg-surface-2", tone[state].ring) : "ring-transparent hover:bg-surface-2/60",
                            )}
                          >
                            <span className="w-16 shrink-0 font-mono text-[11px] text-muted-foreground">{item.connection.gpio != null ? `GPIO${item.connection.gpio}` : "—"}</span>
                            <Trace state={state} live={model.hasMatchingCapture} />
                            <span className="w-7 shrink-0 text-center font-mono text-[11px] text-subtle">{item.instruction?.probe ?? "—"}</span>
                            <span className="w-16 shrink-0 truncate text-sm font-medium text-foreground">{item.connection.role}</span>
                            <span className={cn("hidden w-28 shrink-0 text-right font-mono text-[11px] sm:block", tone[state].text)}>{displayLabel(item)}</span>
                          </button>
                        );
                      })}
                    </div>
                  )}
                </div>
              ))}
            </div>
          </div>

          <AnimatePresence mode="wait" initial={false}>
            {selected && (
              <motion.div
                key={selected.connection.id}
                aria-live="polite"
                initial={reduce ? { opacity: 0 } : { opacity: 0, transform: "translateY(4px)" }}
                animate={{ opacity: 1, transform: "translateY(0px)" }}
                exit={{ opacity: 0 }}
                transition={{ duration: 0.18, ease: EASE_OUT }}
                className="mt-4"
              >
                <div className="flex flex-wrap items-baseline justify-between gap-2">
                  <h3 className="text-sm font-semibold text-foreground">{selected.connection.component_name} · {selected.connection.role}</h3>
                  <span className={cn("font-mono text-[11px]", tone[displayState(selected)].text)}>{displayLabel(selected)}</span>
                </div>
                <dl className="mt-3 grid grid-cols-2 gap-x-6 gap-y-3 border-y border-line-soft py-3 lg:grid-cols-4">
                  {[
                    ["Target", selected.connection.target],
                    ["Expected signal", selected.connection.expected.signal_type || selected.connection.behavior || "Not specified"],
                    ["Probe", selected.instruction?.probe ?? "Not assigned"],
                    ["Observed", model.hasMatchingCapture ? measuredValue(selected) : "Not measured"],
                  ].map(([label, value]) => (
                    <div key={label} className="min-w-0">
                      <dt className="text-[11px] text-subtle">{label}</dt>
                      <dd className="mt-0.5 truncate font-mono text-xs text-foreground" title={value}>{value}</dd>
                    </div>
                  ))}
                </dl>
                {selected.instruction?.safe_warning && (
                  <p className="mt-3 flex items-start gap-2 text-xs leading-relaxed text-warn"><CircleAlert className="mt-px size-3.5 shrink-0" strokeWidth={1.5} />{selected.instruction.safe_warning}</p>
                )}
                {selected.rules.length > 0 && (
                  <ul className="mt-3 space-y-1">
                    {selected.rules.map((rule) => (
                      <li key={rule.id} className="flex gap-2 text-xs">
                        <span className={cn("w-8 shrink-0 font-mono uppercase", rule.status === "pass" ? "text-pass" : rule.status === "warn" ? "text-warn" : "text-fail")}>{rule.status}</span>
                        <span className="text-muted-foreground">{rule.message}</span>
                      </li>
                    ))}
                  </ul>
                )}
                {showActions && (
                  <div className="mt-4 flex flex-wrap gap-2">
                    {model.hasMatchingCapture ? (
                      <>
                        <Button size="sm" variant="outline" onClick={() => onNavigate("live")}><Activity /> Inspect signals</Button>
                        <Button size="sm" variant="outline" onClick={() => onNavigate("diagnosis")}>Review evidence <ArrowUpRight /></Button>
                        <Button size="sm" variant="outline" onClick={() => onNavigate("guided")}>Next test <ArrowUpRight /></Button>
                        <Button size="sm" variant="ghost" onClick={() => onNavigate("history")}><History /> History</Button>
                      </>
                    ) : profile.confirmed && !demoMode && (
                      <Button size="sm" variant="outline" onClick={() => onNavigate("connect")}><Cable /> Open probe setup</Button>
                    )}
                  </div>
                )}
              </motion.div>
            )}
          </AnimatePresence>
          <p className="mt-4 text-[11px] text-subtle">Shows the intended wiring from the profile. Status reflects captured probe readings, not a visual check of the physical wires.</p>
        </>
      )}
    </section>
  );
}
