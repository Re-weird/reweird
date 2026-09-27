"use client";

import { useState } from "react";
import { AnimatePresence, motion, useReducedMotion } from "framer-motion";
import { Activity, ArrowUpRight, Cable, CircleAlert, History, Radio } from "lucide-react";
import type { DemoSession, ProbePlan, ProjectProfile } from "@reweird/shared-types";
import { Button } from "@/components/ui/button";
import { useAppState } from "@/lib/app-state";
import { buildCircuitMap, circuitDisplayState, type CircuitDisplayState, type CircuitMapConnection, type CircuitMapModel } from "@/lib/circuit-map";
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

// ---- Schematic board -------------------------------------------------------
// The profile drawn as a PCB: the controller (U1) on the left, each part
// (U2, U3, …) on the right, one right-angle copper trace per connection with
// its probe test pad. Geometry is computed, so any profile lays out.

const PIN_GAP = 30;
const CHIP_W = 196;
const CHIP_HEAD = 48; // title block above the pins
const BOARD_W = 1180;
const PAD = 30;

const stroke: Record<CircuitDisplayState, string> = {
  waiting: "var(--line)",
  normal: "var(--green)",
  recovered: "var(--green)",
  testing: "var(--amber)",
  suspect: "var(--red)",
};

function controllerPinLabel(item: CircuitMapConnection) {
  if (item.connection.gpio != null) return `GPIO${item.connection.gpio}`;
  const behavior = (item.connection.behavior ?? "").toLowerCase();
  if (behavior.includes("ground") || item.connection.role.toUpperCase() === "GND") return "GND";
  if (behavior.includes("voltage") || behavior.includes("power") || /VCC|VIN|5V|3V3/.test(item.connection.role.toUpperCase())) return "PWR";
  return "—";
}

// PCB-style route: out from the controller pin, a 45° chamfer at this
// trace's own column (so diagonals never overlap), then into the part pin.
function route(x1: number, y1: number, x2: number, y2: number, midX: number) {
  const dy = y2 - y1;
  if (Math.abs(dy) < 1) return { d: `M ${x1} ${y1} H ${x2}`, bends: [] as [number, number][] };
  const half = Math.abs(dy) / 2;
  const a = midX - half;
  const b = midX + half;
  return { d: `M ${x1} ${y1} H ${a} L ${b} ${y2} H ${x2}`, bends: [[a, y1], [b, y2]] as [number, number][] };
}

function SchematicBoard({ controller, voltage, groups, selectedID, onSelect, stateOf, live }: {
  controller: string;
  voltage: number;
  groups: CircuitMapModel["groups"];
  selectedID: string | undefined;
  onSelect: (id: string) => void;
  stateOf: (item: CircuitMapConnection) => CircuitDisplayState;
  live: boolean;
}) {
  const reduce = useReducedMotion();
  const flat = groups.flatMap((group) => group.connections);
  const n = flat.length;

  // Parts stacked on the right, each tall enough for its pins.
  let cursor = PAD + 20;
  const parts = groups.map((group, index) => {
    const height = CHIP_HEAD + Math.max(1, group.connections.length) * PIN_GAP + 8;
    const part = { group, ref: `U${index + 2}`, y: cursor, height };
    cursor += height + 26;
    return part;
  });
  const rightHeight = cursor - 26 + PAD;
  const controllerHeight = CHIP_HEAD + n * PIN_GAP + 8;
  const height = Math.max(rightHeight, controllerHeight + PAD * 2 + 20);
  // One part: line the chips up so every trace runs straight. Several parts:
  // center the controller and let the traces fan out with chamfers.
  const controllerY = parts.length === 1 ? parts[0].y : (height - controllerHeight) / 2;
  const leftX = PAD + 40;
  const rightX = BOARD_W - PAD - 40 - CHIP_W;
  const span = rightX - (leftX + CHIP_W);

  const traces = parts.flatMap((part) => part.group.connections.map((item, pinIndex) => ({ item, part, pinIndex })))
    .map(({ item, part, pinIndex }, index) => {
      const x1 = leftX + CHIP_W + 14;
      const y1 = controllerY + CHIP_HEAD + 14 + index * PIN_GAP;
      const x2 = rightX - 14;
      const y2 = part.y + CHIP_HEAD + 14 + pinIndex * PIN_GAP;
      const midX = leftX + CHIP_W + span * ((index + 1) / (n + 1));
      const path = route(x1, y1, x2, y2, midX);
      const firstBend = path.bends[0]?.[0] ?? (x1 + x2) / 2;
      const lastBend = path.bends[1]?.[0] ?? x1;
      // Probe pad on the first straight run, break mark on the last one.
      // Straight traces stagger their pads so neighbouring labels don't stack.
      const padX = path.bends.length ? x1 + (firstBend - x1) * 0.5 : x1 + (x2 - x1) * (0.18 + 0.14 * (index % 3));
      return { item, x1, y1, x2, y2, ...path, state: stateOf(item), padX, breakX: lastBend + (x2 - lastBend) * (path.bends.length ? 0.5 : 0.62) };
    });

  function Chip({ x, y, h, refdes, title, subtitle, pins, side }: { x: number; y: number; h: number; refdes: string; title: string; subtitle: string; pins: string[]; side: "left" | "right" }) {
    return (
      <g>
        <text x={x} y={y - 8} className="fill-subtle font-mono text-[10px]">{refdes}</text>
        <rect x={x} y={y} width={CHIP_W} height={h} rx={6} className="fill-[var(--surface-2)] stroke-[var(--line)]" strokeWidth={1} />
        <path d={`M ${x + CHIP_W / 2 - 9} ${y} a 9 9 0 0 0 18 0`} className="fill-[var(--bg)] stroke-[var(--line)]" strokeWidth={1} />
        <text x={x + 14} y={y + 24} className="fill-foreground text-[12.5px] font-semibold">{title.length > 26 ? `${title.slice(0, 25)}…` : title}<title>{title}</title></text>
        <text x={x + 14} y={y + 38} className="fill-subtle font-mono text-[9.5px]">{subtitle}</text>
        <line x1={x + 10} y1={y + CHIP_HEAD - 2} x2={x + CHIP_W - 10} y2={y + CHIP_HEAD - 2} stroke="var(--line-soft)" />
        {pins.map((pin, index) => {
          const py = y + CHIP_HEAD + 14 + index * PIN_GAP;
          const edge = side === "right" ? x + CHIP_W : x;
          const out = side === "right" ? 14 : -14;
          return (
            <g key={`${pin}-${index}`}>
              <line x1={edge} y1={py} x2={edge + out} y2={py} className="stroke-[var(--muted)]" strokeWidth={1.5} />
              <rect x={edge + out - 3} y={py - 3} width={6} height={6} className="fill-[var(--muted)]" />
              <text x={side === "right" ? edge - 8 : edge + 8} y={py + 3.5} textAnchor={side === "right" ? "end" : "start"} className="fill-muted-foreground font-mono text-[10px]">{pin}</text>
            </g>
          );
        })}
      </g>
    );
  }

  return (
    <div className="overflow-x-auto rounded-xl ring-1 ring-border">
      <svg viewBox={`0 0 ${BOARD_W} ${height}`} className="block h-auto w-full min-w-[640px]" role="group" aria-label={`Schematic: ${controller} connected to ${groups.map((group) => group.name).join(", ")}`}>
        <defs>
          <pattern id="soldermask" width="14" height="14" patternUnits="userSpaceOnUse">
            <circle cx="1" cy="1" r="0.9" fill="var(--line-soft)" />
          </pattern>
        </defs>
        <rect width={BOARD_W} height={height} fill="var(--scope-bg)" />
        <rect width={BOARD_W} height={height} fill="url(#soldermask)" />
        <rect x={10} y={10} width={BOARD_W - 20} height={height - 20} rx={10} fill="none" stroke="var(--line-soft)" strokeDasharray="2 5" />
        <text x={BOARD_W - 22} y={height - 18} textAnchor="end" className="fill-subtle font-mono text-[9px] tracking-[0.2em]">REWEIRD · PROFILE SCHEMATIC</text>

        {/* traces (under the chips) */}
        {traces.map((trace, index) => {
          const id = trace.item.connection.id;
          const selected = id === selectedID;
          const color = stroke[trace.state];
          const running = live && !reduce && trace.state !== "waiting";
          const label = `${trace.item.connection.component_name} ${trace.item.connection.role}, ${trace.item.instruction?.probe ?? "no probe"}, ${trace.state}`;
          return (
            <g key={id} role="button" tabIndex={0} aria-pressed={selected} aria-label={label} className="cursor-pointer outline-none focus-visible:[&>path.hit]:stroke-[var(--cyan)]"
              onClick={() => onSelect(id)} onKeyDown={(event) => { if (event.key === "Enter" || event.key === " ") { event.preventDefault(); onSelect(id); } }}>
              <path className="hit" d={trace.d} fill="none" stroke="transparent" strokeWidth={16} />
              {selected && <path d={trace.d} fill="none" stroke={color} strokeOpacity={0.18} strokeWidth={8} strokeLinecap="round" strokeLinejoin="round" />}
              <motion.path
                d={trace.d} fill="none" stroke={color} strokeWidth={selected ? 2.25 : 1.5} strokeLinecap="round" strokeLinejoin="round"
                strokeOpacity={trace.state === "waiting" ? 1 : selected ? 1 : 0.7}
                strokeDasharray={trace.state === "waiting" ? "3 4" : undefined}
                initial={reduce ? false : { pathLength: 0 }} animate={{ pathLength: 1 }}
                transition={{ duration: 0.5, delay: 0.1 + index * 0.06, ease: [0.23, 1, 0.32, 1] }}
                style={trace.state === "suspect" && !reduce ? { animation: "trace-flicker 3.2s linear infinite" } : undefined}
              />
              {running && (
                <path d={trace.d} pathLength={100} fill="none" stroke={color} strokeWidth={2.5} strokeLinecap="round"
                  strokeDasharray={trace.state === "suspect" ? "2 98" : "4 96"}
                  style={{ animation: `trace-run ${trace.state === "suspect" ? "3.4s" : "2.2s"} linear infinite`, animationDelay: `${index * 0.35}s` }} />
              )}
              {trace.bends.map(([bx, by]) => (
                <g key={`${bx}-${by}`}>
                  <circle cx={bx} cy={by} r={3.2} fill="var(--scope-bg)" stroke={color} strokeWidth={1.25} strokeOpacity={0.8} />
                  <circle cx={bx} cy={by} r={1} fill={color} />
                </g>
              ))}
              {trace.state === "suspect" && (
                <g transform={`translate(${trace.breakX} ${trace.y2})`}>
                  <rect x={-7} y={-5} width={14} height={10} fill="var(--scope-bg)" />
                  <path d="M -6 5 L -2 -5 M 2 5 L 6 -5" stroke="var(--red)" strokeWidth={1.5} strokeLinecap="round" />
                </g>
              )}
              {trace.item.instruction && (
                <g transform={`translate(${trace.padX} ${trace.y1})`}>
                  <circle r={7} fill="var(--scope-bg)" stroke={color} strokeWidth={1.5} />
                  <circle r={2.5} fill={color} />
                  <text y={-11} textAnchor="middle" className="fill-muted-foreground font-mono text-[9.5px]">{trace.item.instruction.probe}</text>
                </g>
              )}
            </g>
          );
        })}

        <Chip x={leftX} y={controllerY} h={controllerHeight} refdes="U1" title={controller} subtitle={`MCU · ${voltage} V logic`} pins={flat.map(controllerPinLabel)} side="right" />
        {parts.map((part) => (
          <Chip key={part.group.key} x={rightX} y={part.y} h={part.height} refdes={part.ref} title={part.group.name} subtitle={`${part.group.connections.length} pin${part.group.connections.length === 1 ? "" : "s"} in profile`} pins={part.group.connections.map((item) => item.connection.role)} side="left" />
        ))}
      </svg>
    </div>
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
          <div className="mt-4">
            <SchematicBoard controller={profile.controller} voltage={profile.logic_voltage} groups={model.groups} selectedID={selected?.connection.id}
              onSelect={setSelectedID} stateOf={displayState} live={model.hasMatchingCapture} />
            <p className="mt-2 flex flex-wrap gap-x-4 gap-y-1 font-mono text-[10px] text-subtle">
              {([["Normal", "var(--green)"], ["Suspect", "var(--red)"], ["Testing", "var(--amber)"], ["No capture", "var(--line)"]] as const).map(([label, color]) => (
                <span key={label} className="inline-flex items-center gap-1.5"><span className="h-0.5 w-4 rounded-full" style={{ background: color }} />{label}</span>
              ))}
              <span className="inline-flex items-center gap-1.5"><span className="size-2 rounded-full ring-1 ring-muted-foreground" />Probe test pad</span>
            </p>
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
