"use client";

import { memo, useMemo } from "react";
import Link from "next/link";
import { motion, useReducedMotion, type Variants } from "framer-motion";
import { ArrowUpRight, FolderGit2 } from "lucide-react";
import { Button } from "@/components/ui/button";
import { cn } from "@/lib/utils";

// Layout and motion only. There is no per-user activity backend yet, so every
// figure is an honest zero rather than an invented number (design/design.md:
// no fabricated metrics). Wiring real data is a separate task.

const WEEKS = 52;
const DAY_MS = 86_400_000;
const spring = { type: "spring", stiffness: 100, damping: 20 } as const;

const stagger: Variants = { hidden: {}, show: { transition: { staggerChildren: 0.07, delayChildren: 0.05 } } };
const rise: Variants = { hidden: { opacity: 0, y: 14 }, show: { opacity: 1, y: 0, transition: spring } };


const outcomes = [
  { label: "Resolved", tone: "bg-pass" },
  { label: "Improved", tone: "bg-warn" },
  { label: "Unresolved", tone: "bg-fail" },
];

function useCalendar() {
  return useMemo(() => {
    const today = new Date();
    today.setHours(0, 0, 0, 0);
    const start = new Date(today.getTime() - (today.getDay() + (WEEKS - 1) * 7) * DAY_MS);
    const weeks = Array.from({ length: WEEKS }, (_, w) => new Date(start.getTime() + w * 7 * DAY_MS));
    const months = weeks.map((week, index) => {
      const previous = weeks[index - 1];
      return !previous || previous.getMonth() !== week.getMonth() ? week.toLocaleString("en", { month: "short" }) : "";
    });
    return { weeks, months };
  }, []);
}

function SectionHead({ index, title, meta }: { index: string; title: string; meta?: string }) {
  return (
    <div className="mb-5 flex items-baseline justify-between gap-4">
      <div className="flex items-baseline gap-3">
        <span className="font-mono text-[10px] tracking-[0.12em] text-subtle">{index}</span>
        <h2 className="text-[13px] font-semibold tracking-tight text-foreground">{title}</h2>
      </div>
      {meta && <span className="font-mono text-[11px] text-subtle">{meta}</span>}
    </div>
  );
}


const ScanSweep = memo(function ScanSweep() {
  const reduce = useReducedMotion();
  if (reduce) return null;
  return (
    <div className="pointer-events-none absolute inset-0 overflow-hidden" aria-hidden="true">
      <motion.div
        className="h-full w-1/6 bg-linear-to-r from-transparent via-signal/12 to-transparent"
        initial={{ x: "-100%" }}
        animate={{ x: "600%" }}
        transition={{ duration: 7, repeat: Infinity, ease: "easeInOut", repeatDelay: 1.5 }}
      />
    </div>
  );
});

const RadarCore = memo(function RadarCore() {
  const reduce = useReducedMotion();
  return (
    <g>
      {!reduce && (
        <motion.circle
          cx="130" cy="130" r="4"
          className="fill-signal"
          animate={{ r: [4, 16], opacity: [0.35, 0] }}
          transition={{ duration: 2.8, repeat: Infinity, ease: "easeOut" }}
        />
      )}
      <circle cx="130" cy="130" r="3.5" className="fill-signal" />
    </g>
  );
});

// A full year that stretches to the row width: one fluid CSS grid, week
// columns share the width (minmax keeps cells legible and scrolls on small
// screens). Cells fade in column by column via a CSS animation - 364 motion
// components would be needlessly heavy.
function ActivityHeatmap() {
  const { weeks, months } = useCalendar();
  const columns = `1.75rem repeat(${WEEKS}, minmax(9px, 1fr))`;
  return (
    <div className="relative w-full">
      <div className="relative grid gap-[3px] font-mono text-[10px] text-subtle" style={{ gridTemplateColumns: columns }}>
        <span />
        {months.map((label, index) => <span key={index} className="overflow-visible whitespace-nowrap">{label}</span>)}
        {Array.from({ length: 7 }, (_, day) => (
          <div key={day} className="contents">
            <span className="flex items-center leading-none">{day % 2 === 1 ? ["", "Mon", "", "Wed", "", "Fri", ""][day] : ""}</span>
            {weeks.map((week, column) => (
              <span
                key={week.getTime()}
                className="aspect-square w-full rounded-[2px] bg-line-soft motion-reduce:[animation:none]"
                style={{ animation: `heat-in 520ms cubic-bezier(0.16, 1, 0.3, 1) ${column * 14 + day * 10}ms both` }}
                title={`${new Date(week.getTime() + day * DAY_MS).toLocaleDateString()}: 0 sessions`}
              />
            ))}
          </div>
        ))}
        <div className="pointer-events-none absolute inset-y-0 right-0 left-7"><ScanSweep /></div>
      </div>
      <div className="mt-4 flex items-center justify-end gap-1.5 font-mono text-[10px] text-subtle">
        Less
        {["bg-line-soft", "bg-signal/25", "bg-signal/50", "bg-signal/75", "bg-signal"].map((tone) => <span key={tone} className={cn("size-[11px] rounded-[2px]", tone)} />)}
        More
      </div>
    </div>
  );
}

function ActivityRadar() {
  const axes = [
    { x2: 130, y2: 30, label: "Guided tests", lx: 130, ly: 16, anchor: "middle" as const },
    { x2: 230, y2: 130, label: "Computer checks", lx: 236, ly: 134, anchor: "start" as const },
    { x2: 130, y2: 230, label: "Git syncs", lx: 130, ly: 252, anchor: "middle" as const },
    { x2: 30, y2: 130, label: "Diagnoses", lx: 24, ly: 134, anchor: "end" as const },
  ];
  const rings = [0.33, 0.66, 1].map((scale) => {
    const r = 100 * scale;
    return `130,${130 - r} ${130 + r},130 130,${130 + r} ${130 - r},130`;
  });
  return (
    <svg viewBox="-70 0 400 262" className="h-auto w-full max-w-[380px] overflow-visible" role="img" aria-label="Activity mix across diagnoses, guided tests, computer checks and git syncs: no activity yet">
      {rings.map((points) => <polygon key={points} points={points} className="fill-none stroke-line-soft" strokeWidth="1" />)}
      {axes.map((axis, index) => (
        <g key={axis.label}>
          <motion.line
            x1="130" y1="130" x2={axis.x2} y2={axis.y2}
            className="stroke-border" strokeWidth="1"
            initial={{ pathLength: 0 }}
            animate={{ pathLength: 1 }}
            transition={{ duration: 0.9, delay: 0.25 + index * 0.12, ease: [0.16, 1, 0.3, 1] }}
          />
          <text x={axis.lx} y={axis.ly} textAnchor={axis.anchor} className="fill-muted-foreground text-[11px]">{axis.label}</text>
          <text x={axis.lx} y={axis.ly + 13} textAnchor={axis.anchor} className="fill-subtle font-mono text-[10px]">0%</text>
        </g>
      ))}
      <RadarCore />
    </svg>
  );
}

export function AccountDashboardView() {
  return (
    <motion.div data-tw variants={stagger} initial="hidden" animate="show" className="mx-auto w-full max-w-[1100px]">

        <motion.section variants={rise} className="relative">
          <div
            className="pointer-events-none absolute -inset-x-4 -inset-y-4 opacity-60 [background-image:radial-gradient(var(--line-soft)_1px,transparent_1px)] [background-size:14px_14px] [mask-image:radial-gradient(ellipse_at_30%_40%,black,transparent_75%)]"
            aria-hidden="true"
          />
          <div className="relative">
            <SectionHead index="01" title="0 diagnostic sessions in the last year" meta="All projects" />
            <div className="mx-auto w-full overflow-x-auto pb-1 lg:w-[70%]">
              <ActivityHeatmap />
            </div>
          </div>
        </motion.section>

        <div className="mt-12 grid grid-cols-1 gap-12 border-t border-line-soft pt-10 xl:grid-cols-[minmax(0,1.2fr)_minmax(0,1fr)] xl:gap-16">
          <motion.section variants={rise}>
            <SectionHead index="02" title="Activity overview" meta="All projects" />
            <div className="grid grid-cols-1 items-center gap-6 sm:grid-cols-[minmax(0,0.8fr)_minmax(0,1.2fr)]">
              <p className="max-w-[34ch] text-sm leading-relaxed text-muted-foreground">
                Nothing to break down yet. Diagnoses, guided tests, computer checks, and git syncs will shape this once sessions exist.
              </p>
              <ActivityRadar />
            </div>
          </motion.section>

          <motion.section variants={rise}>
            <SectionHead index="03" title="Session outcomes" meta="0 total · all time" />
            <div className="h-1.5 w-full overflow-hidden rounded-full bg-line-soft" role="img" aria-label="No session outcomes yet" />
            <ul className="mt-6 divide-y divide-line-soft">
              {outcomes.map((outcome) => (
                <li key={outcome.label} className="flex items-center gap-3 py-3">
                  <span className={cn("size-2 rounded-full opacity-70", outcome.tone)} />
                  <span className="flex-1 text-sm text-muted-foreground">{outcome.label}</span>
                  <span className="h-1 w-24 rounded-full bg-line-soft" aria-hidden="true" />
                  <span className="w-8 text-right font-mono text-sm tabular-nums text-foreground">0</span>
                </li>
              ))}
            </ul>
          </motion.section>
        </div>

        <motion.section variants={rise} className="mt-12 border-t border-line-soft pt-10">
          <SectionHead index="04" title="Recent projects" />
          <div className="grid grid-cols-1 items-center gap-8 md:grid-cols-[minmax(0,1fr)_minmax(0,1.1fr)]">
            <div className="flex flex-col gap-2" aria-hidden="true">
              {[1, 0.55, 0.25].map((opacity) => (
                <div key={opacity} className="flex items-center gap-3 border-b border-line-soft py-3" style={{ opacity }}>
                  <span className="size-2 rounded-full bg-line-soft" />
                  <span className="h-2 w-40 rounded-full bg-line-soft" />
                  <span className="ml-auto h-2 w-12 rounded-full bg-line-soft" />
                </div>
              ))}
            </div>
            <div className="flex flex-col items-start gap-4">
              <span className="grid size-10 place-items-center rounded-lg border border-border bg-surface-2 text-muted-foreground">
                <FolderGit2 className="size-[18px]" strokeWidth={1.5} />
              </span>
              <div>
                <h3 className="text-sm font-semibold text-foreground">No projects on the bench yet</h3>
                <p className="mt-1 max-w-[40ch] text-sm leading-relaxed text-muted-foreground">Projects you open or analyze will line up here, most recent first.</p>
              </div>
              <Button asChild variant="outline" size="sm" className="active:scale-[0.98]">
                <Link href="/projects">Go to projects <ArrowUpRight /></Link>
              </Button>
            </div>
          </div>
        </motion.section>
    </motion.div>
  );
}
