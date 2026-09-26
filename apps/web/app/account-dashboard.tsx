"use client";

import { Activity, BarChart3, Cpu, FolderGit2 } from "lucide-react";

// Layout/placement only, matching the planned wireframe. No data fetching yet -
// every analytics section renders its real "no data" empty state (per
// design.md: never a decorative chart on empty data). Wiring real numbers is
// a separate task.

export function AccountDashboardView({ onNavigate }: { onNavigate: (view: "projects" | "settings") => void }) {
  return <div className="account-dashboard">
    <div className="account-layout">
      <aside className="account-sidebar">
        <div className="account-avatar">—</div>
        <div className="account-identity"><strong>Local session</strong><small>No user sign-in</small></div>
        <button className="secondary" disabled>Edit profile</button>
        <div className="account-joined">Joined —</div>
        <div className="account-stat-list">
          <div><span>Projects</span><strong>—</strong></div>
          <div><span>Diagnostic sessions</span><strong>—</strong></div>
          <div><span>Resolved</span><strong>—</strong></div>
          <div><span>Unresolved</span><strong>—</strong></div>
          <div><span>Avg. time to resolve</span><strong>—</strong></div>
        </div>
      </aside>

      <div className="account-main">
        <div className="account-tabs">
          <button className="active">Overview</button>
          <button onClick={() => onNavigate("projects")}>Projects</button>
          <button onClick={() => onNavigate("settings")}>Settings</button>
        </div>

        <section className="panel account-section">
          <div className="bench-panel-head"><div><span className="bench-label">Contribution graph</span><h2>Sessions per day</h2></div></div>
          <div className="bench-empty"><Activity size={24} /><h3>No sessions yet</h3><p>Run a guided test on any project to start building history here.</p></div>
        </section>

        <section className="panel account-section">
          <div className="bench-panel-head"><h2>Recent projects</h2></div>
          <div className="bench-empty"><FolderGit2 size={24} /><h3>No projects yet</h3><p>Upload a project to see it listed here.</p></div>
        </section>

        <section className="panel account-section">
          <div className="bench-panel-head"><h2>Activity overview</h2></div>
          <div className="bench-empty"><Cpu size={24} /><h3>Nothing to summarize yet</h3><p>Diagnoses, guided tests, computer checks, and git syncs will break down here once you have some.</p></div>
        </section>

        <section className="panel account-section">
          <div className="bench-panel-head"><h2>Session outcomes</h2></div>
          <div className="bench-empty"><BarChart3 size={24} /><h3>No sessions yet</h3><p>Outcome counts appear once a guided test reaches a final status.</p></div>
        </section>
      </div>
    </div>
  </div>;
}
