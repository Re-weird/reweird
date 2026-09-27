"use client";

import { useCallback, useEffect, useState } from "react";
import { AnimatePresence, motion, useReducedMotion } from "framer-motion";
import { ArrowRight, Check, CircleAlert, Cpu, HeartPulse, RefreshCw } from "lucide-react";
import type { BaselineSource, DevicePassport, HistoryDetail, KnownGoodBaseline, PassportCapture, PassportStatus, ProjectProfile } from "@reweird/shared-types";
import { Button } from "@/components/ui/button";
import { Skeleton } from "@/components/ui/skeleton";
import { ApiError, historyApi, passportApi } from "@/lib/api";
import { CalibrationPanel } from "./calibration-panel";
import { cn } from "@/lib/utils";

type Destination = "profile" | "connect" | "live" | "diagnosis" | "guided" | "history";

const EASE_OUT = [0.23, 1, 0.32, 1] as const;
const reveal = { hidden: { opacity: 0, transform: "translateY(8px)" }, show: { opacity: 1, transform: "translateY(0px)", transition: { duration: 0.28, ease: EASE_OUT } } };

type Tone = "pass" | "warn" | "fail" | "neutral";

const statusMeta: Record<PassportStatus, { label: string; tone: Tone }> = {
  BASELINE_INCOMPATIBLE: { label: "Known Good incompatible with this revision", tone: "warn" },
  HEALTHY: { label: "Healthy", tone: "pass" },
  DEVIATION_DETECTED: { label: "Deviation detected", tone: "fail" },
  NEEDS_VERIFICATION: { label: "Needs verification", tone: "warn" },
  NO_PHYSICAL_BASELINE: { label: "No baseline yet", tone: "neutral" },
  SIMULATED_BASELINE: { label: "Simulated baseline saved", tone: "neutral" },
  SIMULATED_MATCH: { label: "Matches simulated baseline", tone: "pass" },
  SIMULATED_DEVIATION: { label: "Differs from simulated baseline", tone: "fail" },
};

const toneText: Record<Tone, string> = { pass: "text-pass", warn: "text-warn", fail: "text-fail", neutral: "text-muted-foreground" };
const toneDot: Record<Tone, string> = { pass: "bg-pass", warn: "bg-warn", fail: "bg-fail", neutral: "bg-subtle" };

const TIMELINE_LIMIT = 8;

function when(timestamp: number): string {
  return new Date(timestamp).toLocaleString(undefined, { month: "short", day: "numeric", hour: "numeric", minute: "2-digit" });
}

function probeSummary(probe: PassportCapture["probes"][number]): string {
  const parts: string[] = [];
  if (probe.average_voltage != null) parts.push(`${Number(probe.average_voltage.toFixed(2))} V`);
  if (probe.frequency_hz != null) parts.push(probe.frequency_hz >= 1000 ? `${Number((probe.frequency_hz / 1000).toFixed(2))} kHz` : `${Number(probe.frequency_hz.toFixed(2))} Hz`);
  parts.push(`${probe.dropout_events} ${probe.dropout_events === 1 ? "dropout" : "dropouts"}`);
  return parts.join(" · ");
}

function SourceTag({ source }: { source: BaselineSource }) {
  const physical = source === "PHYSICAL";
  return <span className={cn("rounded-full px-1.5 py-px font-mono text-[10px] uppercase ring-1", physical ? "text-signal ring-signal/40" : "text-subtle ring-border")}>{physical ? "Physical" : "Simulated"}</span>;
}

function SectionTitle({ eyebrow, title, action }: { eyebrow: string; title: string; action?: React.ReactNode }) {
  return (
    <div className="flex flex-wrap items-end justify-between gap-3">
      <div>
        <p className="font-mono text-[11px] tracking-[0.14em] text-subtle uppercase">{eyebrow}</p>
        <h2 className="mt-1 text-lg font-semibold tracking-tight text-foreground">{title}</h2>
      </div>
      {action}
    </div>
  );
}

function BaselineSlot({ label, baseline, empty }: { label: string; baseline?: KnownGoodBaseline; empty: string }) {
  return (
    <div className="min-w-0 p-4">
      <p className="font-mono text-[11px] text-subtle">{label}</p>
      {baseline ? (
        <>
          <p className="mt-2 text-sm font-semibold text-foreground">Capture <span className="font-mono">#{baseline.measurement_id}</span></p>
          <p className="mt-1 truncate text-xs text-muted-foreground">{baseline.device_id} · {when(baseline.captured_at_ms || baseline.ingested_at_ms)}</p>
          <p className="mt-1 text-xs text-subtle">{baseline.provenance ?? baseline.source} · Profile revision {baseline.profile_version} · {baseline.window_count ?? 1} windows</p>
          {baseline.note && <p className="mt-1 truncate text-xs text-subtle">&ldquo;{baseline.note}&rdquo;</p>}
        </>
      ) : <p className="mt-2 text-sm text-muted-foreground">{empty}</p>}
    </div>
  );
}

function SaveBaseline({ captures, knownGood, onSave, onNavigate }: {
  captures: PassportCapture[];
  knownGood: KnownGoodBaseline[];
  onSave: (capture: PassportCapture, note: string) => Promise<void>;
  onNavigate: (destination: Destination) => void;
}) {
  const [selectedID, setSelectedID] = useState<number | null>(null);
  const [note, setNote] = useState("");
  const [confirmed, setConfirmed] = useState(false);
  const [saving, setSaving] = useState(false);
  const [saved, setSaved] = useState(false);
  const [error, setError] = useState("");

  if (!captures.length) {
    return (
      <div className="flex flex-wrap items-center justify-between gap-3 rounded-xl border border-dashed border-border p-4">
        <p className="text-sm text-muted-foreground">No stored capture yet. Take one, then mark it as known good here.</p>
        <Button size="sm" variant="outline" className="active:scale-[0.97]" onClick={() => onNavigate("live")}>Open live signals <ArrowRight /></Button>
      </div>
    );
  }

  const selected = captures.find((capture) => capture.measurement_id === selectedID) ?? captures[0];
  const alreadySaved = knownGood.some((record) => record.measurement_id === selected.measurement_id);
  const probes = selected.probes.filter((probe) => probe.role !== "UNASSIGNED");

  async function save() {
    if (!confirmed || alreadySaved) return;
    setSaving(true); setError(""); setSaved(false);
    try {
      await onSave(selected, note);
      setConfirmed(false); setNote(""); setSaved(true);
    } catch (cause) {
      setError(cause instanceof ApiError || cause instanceof Error ? cause.message : "Could not save this capture.");
    } finally { setSaving(false); }
  }

  return (
    <div className="grid grid-cols-1 gap-6 rounded-xl p-4 ring-1 ring-border md:grid-cols-[minmax(0,1fr)_minmax(0,1fr)]">
      <div className="flex min-w-0 flex-col gap-2">
        <label htmlFor="baseline-capture" className="text-xs font-medium text-muted-foreground">Capture</label>
        <select id="baseline-capture" value={selected.measurement_id} onChange={(event) => { setSelectedID(Number(event.target.value)); setConfirmed(false); setSaved(false); }}
          className="h-9 w-full rounded-md bg-background px-2.5 text-sm text-foreground ring-1 ring-border outline-none transition-shadow duration-150 focus:ring-signal">
          {captures.map((capture) => (
            <option key={capture.measurement_id} value={capture.measurement_id}>
              #{capture.measurement_id} · {capture.source === "PHYSICAL" ? "ESP32 serial" : "Simulator"} · {when(capture.captured_at_ms || capture.ingested_at_ms)}
            </option>
          ))}
        </select>
        <ul className="mt-1 divide-y divide-line-soft">
          {probes.map((probe) => (
            <li key={probe.probe} className="flex items-baseline justify-between gap-3 py-1.5 text-xs">
              <span className="text-foreground"><span className="font-mono text-subtle">{probe.probe}</span> {probe.role}</span>
              <span className={cn("font-mono", probe.dropout_events > 0 ? "text-fail" : "text-muted-foreground")}>{probeSummary(probe)}</span>
            </li>
          ))}
        </ul>
      </div>

      <div className="flex min-w-0 flex-col gap-3">
        <div className="flex flex-col gap-2">
          <label htmlFor="baseline-note" className="text-xs font-medium text-muted-foreground">Note <span className="text-subtle">optional</span></label>
          <input id="baseline-note" value={note} maxLength={500} onChange={(event) => setNote(event.target.value)} placeholder="What was working correctly?"
            className="h-9 w-full rounded-md bg-background px-2.5 text-sm text-foreground ring-1 ring-border outline-none transition-shadow duration-150 placeholder:text-subtle focus:ring-signal" />
        </div>
        <label className="flex cursor-pointer items-start gap-2.5 text-sm text-foreground">
          <input type="checkbox" checked={confirmed} disabled={alreadySaved} onChange={(event) => setConfirmed(event.target.checked)} className="mt-0.5 size-4 accent-primary" />
          <span>{selected.source === "SIMULATED" ? "This simulated behavior is the expected reference." : "The device was working correctly during this capture."}</span>
        </label>
        <p className="text-xs leading-relaxed text-subtle">
          {selected.source === "SIMULATED" ? "Simulated baselines only compare against simulator captures. They never count as physical health." : "Physical Known Good requires 10 consecutive REAL SERIAL windows ending at this capture and your confirmation. Use Physical calibration above; this is not a certification."}
        </p>
        <div className="mt-auto flex flex-wrap items-center gap-3">
          <Button size="sm" className="active:scale-[0.97]" disabled={!confirmed || saving || alreadySaved} onClick={() => void save()}>
            {saving ? <RefreshCw className="animate-spin" /> : <Check />}{alreadySaved ? "Already saved" : "Save as known good"}
          </Button>
          <AnimatePresence>
            {saved && !error && <motion.span initial={{ opacity: 0 }} animate={{ opacity: 1 }} exit={{ opacity: 0 }} transition={{ duration: 0.15 }} className="text-xs text-pass">Saved.</motion.span>}
          </AnimatePresence>
          {error && <p role="alert" className="text-xs text-fail">{error}</p>}
        </div>
      </div>
    </div>
  );
}

function LoadingState() {
  return (
    <div data-tw aria-busy="true" aria-label="Loading device health" className="space-y-10">
      <div className="space-y-3 border-b border-border pb-8">
        <Skeleton className="h-3 w-28 bg-surface-2" />
        <Skeleton className="h-7 w-64 bg-surface-2" />
        <Skeleton className="h-4 w-96 max-w-full bg-surface-2" />
      </div>
      <Skeleton className="h-24 w-full bg-surface-2" />
      <div className="space-y-2">{[0, 1, 2, 3].map((row) => <Skeleton key={row} className="h-10 w-full bg-surface-2" />)}</div>
    </div>
  );
}

export function DeviceHealthView({ profile, onNavigate }: {
  profile: ProjectProfile | null;
  onNavigate: (destination: Destination) => void;
}) {
  const reduce = useReducedMotion();
  const [record, setRecord] = useState<DevicePassport | null>(null);
  const [story, setStory] = useState<HistoryDetail[]>([]);
  const [loading, setLoading] = useState(false);
  const [error, setError] = useState("");

  const profileID = profile?.id;
  const load = useCallback(async (isLive: () => boolean = () => true) => {
    if (!profileID) return;
    setLoading(true); setError("");
    try {
      const result = await passportApi.get(profileID);
      if (!isLive()) return;
      setRecord(result);
      const details = await Promise.allSettled(result.history.slice(0, TIMELINE_LIMIT).map((item) => historyApi.detail(item.id)));
      if (!isLive()) return;
      setStory(details.filter((entry): entry is PromiseFulfilledResult<HistoryDetail> => entry.status === "fulfilled").map((entry) => entry.value));
    } catch (cause) {
      if (isLive()) setError(cause instanceof Error ? cause.message : "Device health could not be loaded.");
    } finally {
      if (isLive()) setLoading(false);
    }
  }, [profileID]);

  useEffect(() => {
    if (!profile?.confirmed) return;
    let live = true;
    void load(() => live);
    return () => { live = false; };
  }, [profile?.confirmed, load]);

  if (!profile?.confirmed) {
    return (
      <div data-tw className="flex max-w-lg flex-col items-start gap-3 py-6">
        <span className="grid size-11 place-items-center rounded-lg bg-surface-2 text-muted-foreground ring-1 ring-border"><HeartPulse className="size-5" strokeWidth={1.5} /></span>
        <h3 className="text-base font-semibold text-foreground">Nothing to track yet</h3>
        <p className="text-sm leading-relaxed text-muted-foreground">Device health starts once the project profile is confirmed. Then you can mark a known-good capture and compare later ones against it.</p>
        <Button size="sm" variant="outline" className="active:scale-[0.97]" onClick={() => onNavigate("profile")}>Review profile <ArrowRight /></Button>
      </div>
    );
  }

  if (!record) {
    if (error) {
      return (
        <div data-tw className="flex max-w-lg flex-col items-start gap-3 py-6">
          <span className="grid size-11 place-items-center rounded-lg bg-surface-2 text-fail ring-1 ring-border"><CircleAlert className="size-5" strokeWidth={1.5} /></span>
          <h3 className="text-base font-semibold text-foreground">Couldn&apos;t load device health</h3>
          <p className="text-sm leading-relaxed text-muted-foreground">{error}</p>
          <Button size="sm" variant="outline" className="active:scale-[0.97]" onClick={() => void load()}><RefreshCw /> Try again</Button>
        </div>
      );
    }
    return <LoadingState />;
  }

  const status = statusMeta[record.status];
  const latest = record.recent_captures[0];
  const events = story
    .flatMap((detail) => detail.timeline.map((event) => ({ ...event, source: detail.summary.telemetry_source || "Source not recorded" })))
    .concat(record.known_good.map((baseline) => ({ id: `baseline-${baseline.id}`, timestamp_ms: baseline.saved_at_ms, kind: "KNOWN_GOOD", description: `Capture #${baseline.measurement_id} marked as known good.`, provenance: "USER" as const, source: baseline.source })))
    .sort((a, b) => b.timestamp_ms - a.timestamp_ms)
    .slice(0, TIMELINE_LIMIT);
  const repairs = record.verified_repairs.length;

  async function saveBaseline(capture: PassportCapture, note: string) {
    if (!profileID) return;
    await passportApi.saveKnownGood(profileID, capture.measurement_id, note);
    setRecord(await passportApi.get(profileID));
  }

  return (
    <motion.div data-tw initial={reduce ? false : "hidden"} animate="show" variants={{ show: { transition: { staggerChildren: 0.05 } } }} className="space-y-10">
      <motion.header variants={reveal} className="grid grid-cols-1 gap-6 border-b border-border pb-8 lg:grid-cols-[minmax(0,1fr)_auto] lg:items-end">
        <div className="min-w-0">
          <p className="font-mono text-[11px] tracking-[0.14em] text-subtle uppercase">Device health</p>
          <h1 className={cn("mt-3 flex items-center gap-2.5 text-2xl font-semibold tracking-tight", toneText[status.tone])}>
            <span className={cn("size-2 shrink-0 rounded-full", toneDot[status.tone])} aria-hidden="true" />
            {status.label}
          </h1>
          <p className="mt-2 max-w-[70ch] text-sm leading-relaxed text-muted-foreground">{record.status_detail}</p>
          <p className="mt-3 flex flex-wrap gap-x-4 gap-y-1 font-mono text-[11px] text-subtle">
            <span className="inline-flex items-center gap-1.5"><Cpu className="size-3" strokeWidth={1.5} />{record.profile.controller}</span>
            <span>{record.profile.logic_voltage} V logic</span>
            <span>Profile v{record.profile.version}</span>
            <span>{latest ? `Last capture ${when(latest.captured_at_ms || latest.ingested_at_ms)}` : "No capture yet"}</span>
          </p>
        </div>
        <Button variant="outline" size="sm" className="active:scale-[0.97]" onClick={() => void load()} disabled={loading}>
          <RefreshCw className={cn(loading && "animate-spin")} /> Refresh
        </Button>
      </motion.header>

      {error && <p role="alert" className="text-sm text-fail">{error}</p>}

      {profile.id !== "ultrasonic-demo" && <CalibrationPanel profileID={profile.id} onSaved={() => void load()} />}

      <motion.section variants={reveal} aria-labelledby="known-good-title" className="space-y-4">
        <SectionTitle eyebrow="Reference" title="Known good" />
        <div className="grid grid-cols-1 divide-y divide-line-soft rounded-xl bg-surface ring-1 ring-border md:grid-cols-2 md:divide-x md:divide-y-0">
          <BaselineSlot label="Physical · ESP32 serial" baseline={record.physical_baseline} empty="None yet. Learn 10 matching REAL SERIAL windows, then confirm healthy operation." />
          <BaselineSlot label="Simulated · test harness" baseline={record.simulated_baseline} empty="None saved. Never counts as physical health." />
        </div>
        <SaveBaseline captures={record.recent_captures} knownGood={record.known_good} onSave={saveBaseline} onNavigate={onNavigate} />
      </motion.section>

      <motion.section variants={reveal} aria-labelledby="timeline-title" className="space-y-4">
        <SectionTitle
          eyebrow={repairs ? `History · ${repairs} verified ${repairs === 1 ? "repair" : "repairs"}` : "History"}
          title="Timeline"
          action={<Button variant="ghost" size="sm" className="active:scale-[0.97]" onClick={() => onNavigate("history")}>Full history <ArrowRight /></Button>}
        />
        {events.length ? (
          <ol className="divide-y divide-line-soft border-y border-line-soft">
            {events.map((event) => (
              <li key={event.id} className="grid grid-cols-1 gap-1 py-3 sm:grid-cols-[9rem_minmax(0,1fr)_auto] sm:items-baseline sm:gap-4">
                <time dateTime={new Date(event.timestamp_ms).toISOString()} className="font-mono text-[11px] text-subtle">{when(event.timestamp_ms)}</time>
                <p className="min-w-0 text-sm text-foreground">
                  <span className="font-medium">{event.kind.replaceAll("_", " ").toLowerCase().replace(/^\w/, (letter) => letter.toUpperCase())}</span>
                  <span className="text-muted-foreground"> · {event.kind === "USER_ACTION" ? `You reported: ${event.description}` : event.description}</span>
                </p>
                {event.source === "PHYSICAL" || event.source === "SIMULATED" ? <SourceTag source={event.source} /> : <span className="font-mono text-[10px] text-subtle">{event.source}</span>}
              </li>
            ))}
          </ol>
        ) : <p className="text-sm text-muted-foreground">No events yet. A guided test starts the record.</p>}
      </motion.section>

      <motion.p variants={reveal} className="text-xs leading-relaxed text-subtle">
        No health score is invented. Healthy means a later physical capture matched a physical baseline you reported, within stated tolerances.
      </motion.p>
    </motion.div>
  );
}
