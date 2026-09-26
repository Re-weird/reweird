"use client";

// Layout/placement matches the planned wireframe exactly. No data fetching -
// every grid/value renders neutral/zero (not hidden behind empty-state text)
// since there is no real backend for per-user activity yet. Wiring real
// numbers is a separate follow-up task.

const HEATMAP_WEEKS = 20;
const weeks = Array.from({ length: HEATMAP_WEEKS }, (_, w) => w);
const days = Array.from({ length: 7 }, (_, d) => d);

export function AccountDashboardView({ onNavigate }: { onNavigate: (view: "projects" | "settings") => void }) {
  return <div className="account-dashboard">
    <div className="account-layout">
      <aside className="account-sidebar">
        <div className="account-avatar">—</div>
        <div className="account-identity"><strong>Local session</strong><small>No user sign-in</small></div>
        <button className="text-button" disabled>Edit profile</button>
        <div className="account-joined">Joined —</div>
        <div className="account-stat-list">
          <div><span>Projects</span><strong>0</strong></div>
          <div><span>Diagnostic sessions</span><strong>0</strong></div>
          <div><span>Resolved</span><strong>0</strong></div>
          <div><span>Unresolved</span><strong>0</strong></div>
          <div><span>Avg. time to resolve</span><strong>—</strong></div>
        </div>
      </aside>

      <div className="account-main">
        <div className="account-tabs">
          <button className="active">Overview</button>
          <button onClick={() => onNavigate("projects")}>Projects</button>
          <button onClick={() => onNavigate("settings")}>Settings</button>
        </div>

        <section className="account-section">
          <div className="account-section-head"><h2>0 diagnostic sessions in the last year</h2>
            <div className="heatmap-legend">Less<span className="heatmap-cell" /><span className="heatmap-cell" /><span className="heatmap-cell" /><span className="heatmap-cell" />More</div>
          </div>
          <div className="heatmap-grid">
            {weeks.map((w) => <div className="heatmap-col" key={w}>{days.map((d) => <div className="heatmap-cell" key={d} />)}</div>)}
          </div>
        </section>

        <section className="account-section">
          <div className="account-section-head"><h2>Recent projects</h2></div>
          <div className="account-plain-row"><span>No projects yet</span></div>
        </section>

        <section className="account-section">
          <div className="account-section-head"><h2>Activity overview</h2></div>
          <div className="account-activity">
            <p className="account-activity-copy">No diagnostic activity yet.</p>
            <svg width="220" height="220" viewBox="0 0 220 220" className="activity-chart">
              <line x1="110" y1="110" x2="110" y2="30" stroke="var(--line)" strokeWidth="1" />
              <line x1="110" y1="110" x2="190" y2="110" stroke="var(--line)" strokeWidth="1" />
              <line x1="110" y1="110" x2="110" y2="190" stroke="var(--line)" strokeWidth="1" />
              <line x1="110" y1="110" x2="30" y2="110" stroke="var(--line)" strokeWidth="1" />
              <circle cx="110" cy="110" r="3" fill="var(--muted)" />
              <text x="110" y="20" textAnchor="middle" fontSize="10" fill="var(--subtle-text)">Guided tests</text>
              <text x="200" y="114" textAnchor="start" fontSize="10" fill="var(--subtle-text)">Computer checks</text>
              <text x="110" y="206" textAnchor="middle" fontSize="10" fill="var(--subtle-text)">Git syncs</text>
              <text x="20" y="114" textAnchor="end" fontSize="10" fill="var(--subtle-text)">Diagnoses</text>
            </svg>
          </div>
        </section>

        <section className="account-section">
          <div className="account-section-head"><h2>Session outcomes</h2><small>0 total, all time</small></div>
          <div className="outcome-bar"><span style={{ width: "100%", background: "var(--line-soft)" }} /></div>
          <div className="outcome-legend">
            <span><i className="status-dot stable" />Resolved <strong>0</strong></span>
            <span><i className="status-dot active" />Improved <strong>0</strong></span>
            <span><i className="status-dot intermittent" />Unresolved <strong>0</strong></span>
          </div>
        </section>
      </div>
    </div>
  </div>;
}
