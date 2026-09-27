"use client";

import { useState } from "react";
import { AlertTriangle, CheckCircle2, Clock, ExternalLink, GitBranch, GitCommitHorizontal, Lock, PauseCircle, RefreshCw } from "lucide-react";
import type { LinkedRepository, RepositorySyncStatus } from "@reweird/shared-types";
import { Button } from "@/components/ui/button";
import { cn } from "@/lib/utils";

const statusCopy: Record<RepositorySyncStatus, { label: string; icon: typeof CheckCircle2; tone: string }> = {
  SYNCED: { label: "Analyzed", icon: CheckCircle2, tone: "text-pass" },
  SYNCING: { label: "Reading code", icon: RefreshCw, tone: "text-signal" },
  PENDING: { label: "Not analyzed yet", icon: Clock, tone: "text-muted-foreground" },
  FAILED: { label: "Couldn't analyze", icon: AlertTriangle, tone: "text-fail" },
  BLOCKED: { label: "New commit not analyzed", icon: PauseCircle, tone: "text-warn" },
};

function ago(ms: number) {
  const minutes = Math.max(0, Math.floor((Date.now() - ms) / 60_000));
  if (minutes < 1) return "just now";
  if (minutes < 60) return `${minutes}m ago`;
  const hours = Math.floor(minutes / 60);
  if (hours < 24) return `${hours}h ago`;
  return `${Math.floor(hours / 24)}d ago`;
}

/** The linked GitHub repo: which commit the analysis came from, and whether the latest push was analyzed. */
export function RepositoryStatus({ repository, onSync }: { repository: LinkedRepository; onSync: () => Promise<void> }) {
  const [busy, setBusy] = useState(false);
  const [error, setError] = useState("");
  const status = statusCopy[repository.sync_status] ?? statusCopy.PENDING;
  const StatusIcon = status.icon;
  const commit = repository.last_commit;

  async function sync() {
    setBusy(true); setError("");
    try { await onSync(); }
    catch (cause) { setError(cause instanceof Error ? cause.message : "The repository couldn't be analyzed."); }
    finally { setBusy(false); }
  }

  return (
    <section data-tw aria-label="Linked GitHub repository" className="mb-8 border-b border-border pb-6">
      <div className="flex flex-wrap items-start justify-between gap-4">
        <div className="min-w-0">
          <a href={repository.html_url} target="_blank" rel="noreferrer" className="inline-flex items-center gap-2 text-sm font-semibold text-foreground hover:text-signal">
            {repository.full_name}
            {repository.private && <Lock className="size-3 text-muted-foreground" strokeWidth={1.5} aria-label="Private" />}
            <ExternalLink className="size-3.5 text-muted-foreground" strokeWidth={1.5} />
          </a>
          <p className="mt-1 flex flex-wrap items-center gap-x-3 gap-y-1 text-xs text-muted-foreground">
            <span className="inline-flex items-center gap-1"><GitBranch className="size-3.5" strokeWidth={1.5} /> {repository.default_branch}</span>
            <span className={cn("inline-flex items-center gap-1", status.tone)}><StatusIcon className={cn("size-3.5", repository.sync_status === "SYNCING" && "animate-spin")} strokeWidth={1.5} /> {status.label}</span>
            {repository.synced_at_ms ? <span>checked {ago(repository.synced_at_ms)}</span> : null}
          </p>
        </div>
        <Button size="sm" variant="outline" disabled={busy || repository.sync_status === "SYNCING"} onClick={() => void sync()}>
          <RefreshCw className={cn(busy && "animate-spin")} /> {busy ? "Checking…" : "Check for new commits"}
        </Button>
      </div>

      {commit ? (
        <a href={commit.html_url} target="_blank" rel="noreferrer" className="mt-4 flex items-center gap-3 rounded-lg bg-surface px-3 py-2.5 ring-1 ring-border transition-colors hover:bg-surface-2">
          <GitCommitHorizontal className="size-4 shrink-0 text-muted-foreground" strokeWidth={1.5} />
          <span className="min-w-0 flex-1 truncate text-sm text-foreground">{commit.message || "(no message)"}</span>
          <span className="hidden shrink-0 text-xs text-muted-foreground sm:inline">{commit.author_name} · {ago(commit.committed_at_ms)}</span>
          <span className="shrink-0 font-mono text-xs text-muted-foreground">{commit.sha.slice(0, 7)}</span>
        </a>
      ) : (
        <p className="mt-4 text-sm text-muted-foreground">No commit analyzed yet. ReWeird reads {repository.default_branch} when you push, or when you check now.</p>
      )}

      {commit && repository.analyzed_files?.length ? (
        <p className="mt-2 text-xs text-muted-foreground">
          {repository.analyzed_files.length} file{repository.analyzed_files.length === 1 ? "" : "s"} analyzed
          {repository.skipped_files ? ` · ${repository.skipped_files} skipped (dependencies, other languages, or files that look like credentials)` : ""}
        </p>
      ) : null}
      {(error || repository.sync_error) && <p role="alert" className="mt-3 text-sm text-fail">{error || repository.sync_error}</p>}
    </section>
  );
}
