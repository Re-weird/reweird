"use client";

import { useCallback, useEffect, useState } from "react";
import { ExternalLink, Github, RefreshCw, Unplug, WifiOff } from "lucide-react";
import type { GitHubStatus } from "@reweird/shared-types";
import { Button } from "@/components/ui/button";
import { Skeleton } from "@/components/ui/skeleton";
import { githubApi } from "@/lib/api";
import { rememberGitHubReturn } from "@/lib/github-return";

export function useGitHubStatus() {
  const [status, setStatus] = useState<GitHubStatus | null>(null);
  const [error, setError] = useState("");
  const refresh = useCallback(async () => {
    setError("");
    try { setStatus(await githubApi.status()); }
    catch (cause) { setStatus(null); setError(cause instanceof Error ? cause.message : "The ReWeird API is unavailable."); }
  }, []);
  useEffect(() => { void refresh(); }, [refresh]);
  return { status, setStatus, error, refresh };
}

/** Sends the user to GitHub to install the ReWeird App, remembering where to come back to. */
export function startGitHubInstall(installURL: string, reopenNewProject: boolean) {
  rememberGitHubReturn({ path: window.location.pathname, reopenNewProject });
  window.location.assign(installURL);
}

export function GitHubNotConfigured() {
  return (
    <div className="rounded-lg border border-dashed border-border px-4 py-3 text-sm leading-relaxed text-muted-foreground">
      GitHub isn&apos;t set up on this ReWeird server yet. Add the GitHub App settings (<span className="font-mono text-xs text-foreground">GITHUB_APP_*</span>, <span className="font-mono text-xs text-foreground">GITHUB_WEBHOOK_SECRET</span>) to the API environment and restart it.
    </div>
  );
}

export function GitHubSettingsSection() {
  const { status, setStatus, error, refresh } = useGitHubStatus();
  const [busy, setBusy] = useState(false);
  const [actionError, setActionError] = useState("");

  async function disconnect() {
    setBusy(true); setActionError("");
    try { setStatus(await githubApi.disconnect()); }
    catch (cause) { setActionError(cause instanceof Error ? cause.message : "Couldn't disconnect GitHub."); }
    finally { setBusy(false); }
  }

  return (
    <section data-tw aria-labelledby="github-settings-title" className="mb-10 border-b border-border pb-10">
      <div className="mb-4 flex items-start justify-between gap-4">
        <div>
          <h2 id="github-settings-title" className="text-base font-semibold text-foreground">GitHub</h2>
          <p className="mt-1 max-w-[62ch] text-sm leading-relaxed text-muted-foreground">
            Each project follows one of your repositories. When you push to its default branch, ReWeird reads the firmware and updates the project&apos;s analysis. Access is read-only.
          </p>
        </div>
      </div>

      {error ? (
        <div className="flex flex-wrap items-center gap-3 text-sm text-muted-foreground">
          <WifiOff className="size-4" strokeWidth={1.5} /> {error}
          <Button size="sm" variant="outline" onClick={() => void refresh()}><RefreshCw /> Try again</Button>
        </div>
      ) : !status ? (
        <Skeleton className="h-16 w-full max-w-xl bg-surface-2" />
      ) : !status.configured ? (
        <GitHubNotConfigured />
      ) : status.connected ? (
        <div className="flex max-w-xl flex-wrap items-center gap-4 rounded-lg bg-surface px-4 py-3 ring-1 ring-border">
          {/* eslint-disable-next-line @next/next/no-img-element */}
          <img src={`https://github.com/${encodeURIComponent(status.account_login ?? "")}.png?size=64`} alt="" className="size-9 rounded-full ring-1 ring-border" />
          <div className="min-w-0 flex-1">
            <p className="truncate text-sm font-semibold text-foreground">{status.account_login}</p>
            <p className="text-xs text-muted-foreground">
              {status.account_type === "Organization" ? "Organization" : "Personal account"}
              {status.connected_at_ms ? ` · connected ${new Date(status.connected_at_ms).toLocaleDateString()}` : ""}
            </p>
          </div>
          <div className="flex gap-2">
            {status.install_url && <Button size="sm" variant="outline" onClick={() => startGitHubInstall(status.install_url!, false)}><ExternalLink /> Repository access</Button>}
            <Button size="sm" variant="ghost" disabled={busy} onClick={() => void disconnect()}><Unplug /> Disconnect</Button>
          </div>
        </div>
      ) : (
        <div className="flex max-w-xl flex-wrap items-center gap-4">
          <Button onClick={() => status.install_url && startGitHubInstall(status.install_url, false)} disabled={!status.install_url}><Github /> Connect GitHub</Button>
          <p className="text-xs leading-relaxed text-muted-foreground">You choose which repositories ReWeird can read on GitHub.</p>
        </div>
      )}
      {actionError && <p role="alert" className="mt-3 text-sm text-fail">{actionError}</p>}
    </section>
  );
}
