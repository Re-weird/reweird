"use client";

import { memo } from "react";
import { motion, useReducedMotion } from "framer-motion";
import { Avatar, AvatarFallback } from "@/components/ui/avatar";
import { Button } from "@/components/ui/button";
import { Tooltip, TooltipContent, TooltipTrigger } from "@/components/ui/tooltip";

// Persistent left rail on every page. No per-user backend yet, so every figure
// is an honest zero (design/design.md: no fabricated metrics).
const stats = [
  { label: "Projects", value: "0" },
  { label: "Diagnostic sessions", value: "0" },
  { label: "Resolved", value: "0" },
  { label: "Unresolved", value: "0" },
  { label: "Avg. time to resolve", value: "—" },
];

const PresencePulse = memo(function PresencePulse() {
  const reduce = useReducedMotion();
  return (
    <span className="absolute right-1 bottom-1 grid size-3.5 place-items-center rounded-full bg-background">
      {!reduce && (
        <motion.span
          className="absolute size-2.5 rounded-full bg-subtle"
          animate={{ scale: [1, 2.1], opacity: [0.45, 0] }}
          transition={{ duration: 2.4, repeat: Infinity, ease: "easeOut" }}
        />
      )}
      <span className="relative size-2 rounded-full bg-subtle" />
    </span>
  );
});

export function ProfileRail() {
  return (
    <aside data-tw aria-label="Operator profile" className="hidden flex-col gap-6 self-start lg:sticky lg:top-[80px] lg:flex">
      <div className="relative w-fit">
        <Avatar className="size-24 ring-1 ring-border">
          <AvatarFallback className="bg-surface-3 font-mono text-2xl font-medium text-muted-foreground">RW</AvatarFallback>
        </Avatar>
        <PresencePulse />
      </div>

      <div>
        <p className="font-mono text-[10px] tracking-[0.14em] text-subtle uppercase">Operator</p>
        <h2 className="mt-1.5 text-xl font-semibold tracking-tight text-foreground">Local session</h2>
        <p className="mt-0.5 text-sm text-muted-foreground">No user sign-in yet</p>
      </div>

      <Tooltip>
        <TooltipTrigger asChild>
          <span tabIndex={0} className="w-full rounded-md">
            <Button variant="outline" size="sm" disabled className="w-full">Edit profile</Button>
          </span>
        </TooltipTrigger>
        <TooltipContent side="right">Profiles arrive with sign-in</TooltipContent>
      </Tooltip>

      <dl className="divide-y divide-line-soft border-y border-line-soft">
        {stats.map((stat) => (
          <div key={stat.label} className="flex items-baseline justify-between py-2.5">
            <dt className="text-xs text-muted-foreground">{stat.label}</dt>
            <dd className="font-mono text-base tabular-nums text-foreground">{stat.value}</dd>
          </div>
        ))}
      </dl>

      <p className="font-mono text-[11px] text-subtle">Joined —</p>
    </aside>
  );
}
