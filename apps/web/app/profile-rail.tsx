"use client";

import { memo } from "react";
import Link from "next/link";
import { motion, useReducedMotion } from "framer-motion";
import { Github } from "lucide-react";
import { Avatar, AvatarFallback, AvatarImage } from "@/components/ui/avatar";
import { Button } from "@/components/ui/button";
import { Skeleton } from "@/components/ui/skeleton";
import { formatDuration, medianResolveMS, useAccountActivity } from "@/lib/account-activity";
import { AUTH_ENABLED, useUser } from "@/lib/auth";

const PresencePulse = memo(function PresencePulse({ online }: { online: boolean }) {
  const reduce = useReducedMotion();
  const tone = online ? "bg-pass" : "bg-subtle";
  return (
    <span className="absolute right-1 bottom-1 grid size-3.5 place-items-center rounded-full bg-background">
      {!reduce && (
        <motion.span
          className={`absolute size-2.5 rounded-full ${tone}`}
          animate={{ scale: [1, 2.1], opacity: [0.45, 0] }}
          transition={{ duration: 2.4, repeat: Infinity, ease: "easeOut" }}
        />
      )}
      <span className={`relative size-2 rounded-full ${tone}`} />
    </span>
  );
});

function initials(name: string) {
  const parts = name.replace(/[^a-zA-Z0-9 ]/g, " ").trim().split(/\s+/).filter(Boolean);
  return (parts.length > 1 ? parts[0][0] + parts[1][0] : name.slice(0, 2)).toUpperCase();
}

// Persistent left rail: who is working and their totals, all from the API.
export function ProfileRail() {
  const user = useUser();
  const activity = useAccountActivity();
  const data = activity.status === "ready" ? activity.data : null;
  const github = data?.github;
  const signedIn = AUTH_ENABLED && user.isSignedIn;
  const name = signedIn ? user.name ?? user.email ?? "Signed in" : github?.connected ? github.account_login ?? "Local session" : "Local session";
  const subtitle = signedIn ? user.email ?? "" : github?.connected ? "GitHub account" : "No sign-in on this server";
  const image = signedIn ? user.imageUrl : github?.connected && github.account_login ? `https://github.com/${encodeURIComponent(github.account_login)}.png?size=192` : null;

  const sessions = data?.sessions ?? [];
  const median = medianResolveMS(sessions);
  const firstProject = data?.projects.reduce<number | null>((earliest, project) => (earliest === null || project.created_at_ms < earliest ? project.created_at_ms : earliest), null) ?? null;
  const stats = [
    { label: "Projects", value: data ? String(data.projects.length) : null },
    { label: "Repos linked", value: data ? String(data.projects.filter((project) => project.repository).length) : null },
    { label: "Diagnostic sessions", value: data ? String(sessions.length) : null },
    { label: "Resolved", value: data ? String(sessions.filter((item) => item.status === "RESOLVED").length) : null },
    { label: "Unresolved", value: data ? String(sessions.filter((item) => item.status === "UNRESOLVED").length) : null },
    { label: "Median time to resolve", value: data ? (median === null ? "—" : formatDuration(median)) : null },
  ];

  return (
    <aside data-tw aria-label="Operator profile" className="hidden flex-col gap-6 self-start lg:sticky lg:top-[80px] lg:flex">
      <div className="relative w-fit">
        <Avatar className="size-24 ring-1 ring-border">
          {image && <AvatarImage src={image} alt="" />}
          <AvatarFallback className="bg-surface-3 font-mono text-2xl font-medium text-muted-foreground">{initials(name === "Local session" ? "RW" : name)}</AvatarFallback>
        </Avatar>
        <PresencePulse online={Boolean(signedIn || github?.connected)} />
      </div>

      <div className="min-w-0">
        <p className="font-mono text-[10px] tracking-[0.14em] text-subtle uppercase">Operator</p>
        <h2 className="mt-1.5 truncate text-xl font-semibold tracking-tight text-foreground">{name}</h2>
        <p className="mt-0.5 truncate text-sm text-muted-foreground">{subtitle}</p>
      </div>

      {github?.configured && !github.connected ? (
        <Button asChild variant="outline" size="sm" className="w-full"><Link href="/settings"><Github /> Connect GitHub</Link></Button>
      ) : (
        <Button asChild variant="outline" size="sm" className="w-full"><Link href="/settings">Settings</Link></Button>
      )}

      {activity.status === "error" ? (
        <div className="border-y border-line-soft py-3 text-xs text-muted-foreground">
          Couldn&apos;t load your totals. <button type="button" onClick={activity.reload} className="underline underline-offset-2 hover:text-foreground">Retry</button>
        </div>
      ) : (
        <dl className="divide-y divide-line-soft border-y border-line-soft">
          {stats.map((stat) => (
            <div key={stat.label} className="flex items-baseline justify-between py-2.5">
              <dt className="text-xs text-muted-foreground">{stat.label}</dt>
              <dd className="font-mono text-base tabular-nums text-foreground">{stat.value ?? <Skeleton className="h-4 w-6 bg-surface-2" />}</dd>
            </div>
          ))}
        </dl>
      )}

      <p className="font-mono text-[11px] text-subtle">
        {firstProject ? `First project ${new Date(firstProject).toLocaleDateString(undefined, { month: "short", day: "numeric", year: "numeric" })}` : data ? "No projects yet" : " "}
      </p>
    </aside>
  );
}
