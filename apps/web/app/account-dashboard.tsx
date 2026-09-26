"use client";

import { useEffect, useMemo, useState } from "react";
import { Activity, BarChart3, CheckCircle2, FolderGit2 } from "lucide-react";
import type { HistoryStatus, HistorySummary } from "@reweird/shared-types";
import { historyApi, projectApi } from "@/lib/api";

const WEEKS = 20;
const DAY_MS = 24 * 60 * 60 * 1000;

function formatDuration(ms: number) {
  const totalMinutes = Math.round(ms / 60_000);
  if (totalMinutes < 1) return "<1m";
  const hours = Math.floor(totalMinutes / 60);
  const minutes = totalMinutes % 60;
  return hours > 0 ? `${hours}h ${minutes}m` : `${minutes}m`;
}

function startOfDay(ms: number) {
  const date = new Date(ms);
  date.setHours(0, 0, 0, 0);
  return date.getTime();
}

export function AccountDashboardView() {
  const [items, setItems] = useState<HistorySummary[] | null>(null);
  const [projectCount, setProjectCount] = useState<number | null>(null);
  const [error, setError] = useState("");

  useEffect(() => {
    let live = true;
    historyApi.list({ sort: "newest" }).then((response) => { if (live) setItems(response.items); })
      .catch((cause) => { if (live) { setError(cause instanceof Error ? cause.message : "History is unavailable."); setItems([]); } });
    projectApi.listProjects().then((list) => { if (live) setProjectCount((list ?? []).length); }).catch(() => { if (live) setProjectCount(0); });
    return () => { live = false; };
  }, []);

  const stats = useMemo(() => {
    if (!items) return null;
    const counts: Partial<Record<HistoryStatus, number>> = {};
    for (const item of items) counts[item.status] = (counts[item.status] ?? 0) + 1;
    const resolvedDurations = items
      .filter((item) => item.status === "RESOLVED" && item.ended_at_ms)
      .map((item) => item.ended_at_ms! - item.started_at_ms);
    const avgResolveMs = resolvedDurations.length ? resolvedDurations.reduce((a, b) => a + b, 0) / resolvedDurations.length : null;
    return { total: items.length, counts, avgResolveMs };
  }, [items]);

  const heatmap = useMemo(() => {
    if (!items || items.length === 0) return null;
    const perDay = new Map<number, number>();
    for (const item of items) {
      const day = startOfDay(item.started_at_ms);
      perDay.set(day, (perDay.get(day) ?? 0) + 1);
    }
    const today = startOfDay(Date.now());
    const totalDays = WEEKS * 7;
    const start = today - (totalDays - 1) * DAY_MS;
    const max = Math.max(...perDay.values(), 1);
    const weeks: { day: number; count: number }[][] = [];
    for (let w = 0; w < WEEKS; w++) {
      const week: { day: number; count: number }[] = [];
      for (let d = 0; d < 7; d++) {
        const day = start + (w * 7 + d) * DAY_MS;
        week.push({ day, count: perDay.get(day) ?? 0 });
      }
      weeks.push(week);
    }
    return { weeks, max };
  }, [items]);

  return <div className="account-dashboard">
    <section className="page-heading">
      <div><span className="kicker">Account</span><h1>Your activity</h1><p>Diagnostic sessions across every project on this ReWeird instance. Figures come straight from stored history — nothing here is estimated.</p></div>
    </section>

    {error && <div className="form-error page-error">{error}</div>}

    <section className="bench-stats" aria-label="Account summary">
      <div><FolderGit2 size={17} /><strong>{projectCount ?? "—"}<small>Projects</small></strong></div>
      <div><Activity size={17} /><strong>{stats ? stats.total : "—"}<small>Diagnostic sessions</small></strong></div>
      <div><CheckCircle2 size={17} /><strong>{stats ? stats.counts.RESOLVED ?? 0 : "—"}<small>Resolved</small></strong></div>
      <div><BarChart3 size={17} /><strong>{stats && stats.avgResolveMs != null ? formatDuration(stats.avgResolveMs) : "—"}<small>Avg. time to resolve</small></strong></div>
    </section>

    <section className="panel account-heatmap">
      <div className="bench-panel-head"><div><span className="bench-label">Last {WEEKS} weeks</span><h2>Sessions per day</h2></div></div>
      {heatmap ? <div className="heatmap-grid">
        {heatmap.weeks.map((week) => <div className="heatmap-col" key={week[0].day}>
          {week.map((cell) => <div key={cell.day} className="heatmap-cell" style={{ opacity: cell.count === 0 ? 1 : Math.min(1, 0.25 + (cell.count / heatmap.max) * 0.75) }} data-active={cell.count > 0} title={`${new Date(cell.day).toLocaleDateString()}: ${cell.count} session${cell.count === 1 ? "" : "s"}`} />)}
        </div>)}
      </div> : <div className="bench-empty"><Activity size={24} /><h3>No sessions yet</h3><p>Run a guided test on any project to start building history here.</p></div>}
    </section>

    <section className="panel account-outcomes">
      <div className="bench-panel-head"><div><span className="bench-label">All time</span><h2>Session outcomes</h2></div></div>
      {stats && stats.total > 0 ? <div className="outcome-rows">
        {(["RESOLVED", "IMPROVED", "UNRESOLVED", "OPEN", "TESTING", "WAITING_FOR_USER", "VERIFYING", "CANCELLED", "INCONCLUSIVE"] as HistoryStatus[])
          .filter((status) => (stats.counts[status] ?? 0) > 0)
          .map((status) => <div className="outcome-row" key={status}>
            <span className={`history-status status-${status.toLowerCase()}`}>{status.replaceAll("_", " ")}</span>
            <span className="outcome-bar"><span style={{ width: `${((stats.counts[status] ?? 0) / stats.total) * 100}%` }} /></span>
            <strong>{stats.counts[status]}</strong>
          </div>)}
      </div> : <div className="bench-empty"><BarChart3 size={24} /><h3>No sessions yet</h3><p>Outcome counts appear once a guided test reaches a final status.</p></div>}
    </section>
  </div>;
}
