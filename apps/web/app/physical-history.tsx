"use client";

import { useEffect, useRef, useState } from "react";
import { AlertTriangle, CheckCircle2, Cpu, GitCommitHorizontal, Image as ImageIcon, RefreshCw, Upload, Zap, X } from "lucide-react";
import type {
  ComponentChange,
  ConnectionChange,
  EvidenceState,
  FieldChange,
  PhysicalCommit,
  PhysicalCommitDetail,
  PhysicalCommitDiff,
  ProbeElectricalChange,
  Project,
} from "@reweird/shared-types";
import { ApiError, physicalGitApi } from "@/lib/api";

function errorMessage(error: unknown): string {
  return error instanceof ApiError ? error.message : "The request failed. Check the backend and try again.";
}

function formatTimestamp(createdAtMS: number): string {
  return new Date(createdAtMS).toLocaleString();
}

const statusLabel: Record<EvidenceState, string> = {
  UNCHANGED: "No change",
  CHANGED: "Changed",
  ADDED: "Added",
  REMOVED: "Removed",
  NOT_CAPTURED: "Not captured",
  UNAVAILABLE: "Insufficient captured evidence to compare",
};

const statusColor: Record<EvidenceState, string> = {
  UNCHANGED: "var(--muted)",
  CHANGED: "var(--cyan)",
  ADDED: "var(--green)",
  REMOVED: "var(--red)",
  NOT_CAPTURED: "var(--muted)",
  UNAVAILABLE: "var(--amber)",
};

function formatFieldValue(value: unknown): string {
  if (value === null || value === undefined) return "—";
  if (typeof value === "boolean") return value ? "Yes" : "No";
  return String(value);
}

function FieldChangeRow({ change }: { change: FieldChange }) {
  return (
    <div className="pin-map">
      <div>
        <span>{change.field.replaceAll("_", " ")}</span>
        <b>{formatFieldValue(change.before)} &rarr; {formatFieldValue(change.after)}</b>
      </div>
    </div>
  );
}

function DiffSection({ title, status, children }: { title: string; status: EvidenceState; children?: React.ReactNode }) {
  return (
    <div style={{ marginBottom: 16 }}>
      <div className="panel-heading">
        <div><span className="eyebrow">{title}</span></div>
        <b style={{ color: statusColor[status], fontSize: 9, textTransform: "uppercase", letterSpacing: 0.6 }}>{statusLabel[status]}</b>
      </div>
      {children}
    </div>
  );
}

function ComponentDiffRow({ change }: { change: ComponentChange }) {
  return (
    <div style={{ padding: "6px 0", borderTop: "1px solid var(--line-soft)" }}>
      <div style={{ display: "flex", justifyContent: "space-between", fontSize: 9 }}>
        <span style={{ color: statusColor[change.status] }}>{change.status === "ADDED" ? "+" : change.status === "REMOVED" ? "-" : "~"} {change.name || change.component_id}</span>
      </div>
      {change.fields?.map((field) => <FieldChangeRow key={field.field} change={field} />)}
    </div>
  );
}

function ConnectionDiffRow({ change }: { change: ConnectionChange }) {
  return (
    <div style={{ padding: "6px 0", borderTop: "1px solid var(--line-soft)" }}>
      <div style={{ display: "flex", justifyContent: "space-between", fontSize: 9 }}>
        <span style={{ color: statusColor[change.status] }}>{change.status === "ADDED" ? "+" : change.status === "REMOVED" ? "-" : "~"} {change.summary || change.connection_id}</span>
      </div>
      {change.fields?.map((field) => <FieldChangeRow key={field.field} change={field} />)}
    </div>
  );
}

function ProbeDiffRow({ change }: { change: ProbeElectricalChange }) {
  return (
    <div style={{ padding: "6px 0", borderTop: "1px solid var(--line-soft)" }}>
      <div style={{ display: "flex", justifyContent: "space-between", fontSize: 9 }}>
        <span style={{ color: statusColor[change.status] }}>{change.status === "ADDED" ? "+" : change.status === "REMOVED" ? "-" : "~"} {change.probe}</span>
      </div>
      {change.fields?.map((field) => <FieldChangeRow key={field.field} change={field} />)}
    </div>
  );
}

function CommitPhysicalStateModal({
  projectID,
  onClose,
  onCommitted,
}: {
  projectID: string;
  onClose: () => void;
  onCommitted: (commit: PhysicalCommit) => void;
}) {
  const [note, setNote] = useState("");
  const [image, setImage] = useState<File | null>(null);
  const [busy, setBusy] = useState(false);
  const [error, setError] = useState("");
  const formRef = useRef<HTMLFormElement>(null);

  useEffect(() => {
    const previousFocus = document.activeElement as HTMLElement | null;
    const previousOverflow = document.body.style.overflow;
    document.body.style.overflow = "hidden";
    formRef.current?.querySelector<HTMLElement>("textarea, input")?.focus();
    return () => {
      document.body.style.overflow = previousOverflow;
      previousFocus?.focus();
    };
  }, []);

  function handleDialogKey(event: React.KeyboardEvent<HTMLFormElement>) {
    if (event.key === "Escape" && !busy) { event.preventDefault(); onClose(); }
    if (event.key !== "Tab") return;
    const controls = formRef.current?.querySelectorAll<HTMLElement>("button:not(:disabled), input:not(:disabled), textarea:not(:disabled)");
    if (!controls?.length) return;
    const first = controls[0];
    const last = controls[controls.length - 1];
    if (event.shiftKey && document.activeElement === first) { event.preventDefault(); last.focus(); }
    else if (!event.shiftKey && document.activeElement === last) { event.preventDefault(); first.focus(); }
  }

  async function submit(event: React.FormEvent<HTMLFormElement>) {
    event.preventDefault();
    setError("");
    if (image && image.size > 5 * 1024 * 1024) {
      setError("The photo exceeds the 5 MB limit.");
      return;
    }
    setBusy(true);
    try {
      const commit = await physicalGitApi.create(projectID, { note: note.trim() || undefined, file: image ?? undefined });
      onCommitted(commit);
    } catch (caught) {
      setError(errorMessage(caught));
    } finally {
      setBusy(false);
    }
  }

  return (
    <div className="modal-backdrop" role="presentation" onMouseDown={busy ? undefined : onClose}>
      <form ref={formRef} className="modal" role="dialog" aria-modal="true" aria-labelledby="commit-physical-state-title" onKeyDown={handleDialogKey} onSubmit={submit} onMouseDown={(event) => event.stopPropagation()}>
        <div className="modal-head">
          <div><span className="eyebrow">Physical Git</span><h2 id="commit-physical-state-title">Commit physical state</h2></div>
          <button type="button" className="icon-button" onClick={onClose} disabled={busy} aria-label="Close"><X size={18} /></button>
        </div>
        <label>Note <small className="label-hint">optional</small><textarea rows={3} maxLength={2000} value={note} disabled={busy} onChange={(event) => setNote(event.target.value)} placeholder="What changed on the bench?" /></label>
        <div className="upload-grid">
          <label className={image ? "has-file" : ""}><ImageIcon size={20} /><span>{image?.name ?? "Photo"}</span><small>PNG/JPG · optional · max 5 MB</small><input type="file" accept="image/png,image/jpeg,.png,.jpg,.jpeg" disabled={busy} onChange={(event) => setImage(event.target.files?.[0] ?? null)} /></label>
        </div>
        {error && <div className="form-error" role="alert"><AlertTriangle size={15} />{error}</div>}
        <div className="modal-actions">
          <button type="button" className="secondary" onClick={onClose} disabled={busy}>Cancel</button>
          <button className="primary" type="submit" disabled={busy}>{busy ? <RefreshCw className="spin" size={16} /> : <GitCommitHorizontal size={16} />} Commit physical state</button>
        </div>
      </form>
    </div>
  );
}

function CommitDetailPanel({ detail }: { detail: PhysicalCommitDetail }) {
  const { commit, measurement } = detail;
  const components = commit.profile_snapshot?.components ?? null;
  const connections = commit.profile_snapshot?.connections ?? null;
  const hasSoftware = Boolean(commit.software_provider || commit.software_repository || commit.software_revision);

  return (
    <section className="panel history-detail">
      <span className="eyebrow">Physical Commit</span>
      <h2>{commit.display_id}</h2>
      <div className="spec-chips"><span>{formatTimestamp(commit.created_at_ms)}</span></div>
      {commit.note && <p>{commit.note}</p>}

      <h3>Visual</h3>
      {commit.image ? (
        <div className="analysis-source-row"><ImageIcon size={17} /><div><strong>{commit.image.original_filename}</strong><span>{commit.image.content_type} &middot; {(commit.image.size_bytes / 1024).toFixed(0)} KB</span></div></div>
      ) : <p className="inline-empty">Not captured</p>}

      <h3>Hardware</h3>
      {components === null ? (
        <p className="inline-empty">Not captured</p>
      ) : components.length === 0 ? (
        <p className="inline-empty">No components captured</p>
      ) : (
        <div className="git-files">{components.map((component) => <div key={component.id}><code>{component.name}</code><small>{component.interface_type || "Interface unconfirmed"}</small></div>)}</div>
      )}

      <h3>Circuit</h3>
      {connections === null ? (
        <p className="inline-empty">Not captured</p>
      ) : connections.length === 0 ? (
        <p className="inline-empty">No connections captured</p>
      ) : (
        <div className="git-files">{connections.map((connection) => <div key={connection.id}><code>{connection.gpio != null ? `GPIO${connection.gpio}` : connection.role} &rarr; {connection.target}</code><small>{connection.behavior}</small></div>)}</div>
      )}

      <h3>Electrical</h3>
      {measurement ? (
        <div className="git-files">
          {measurement.analysis.probes.map((probe) => (
            <div key={probe.probe}>
              <code>{probe.probe}</code>
              <small>
                {probe.average_voltage != null ? `${probe.average_voltage.toFixed(2)} V · ` : ""}
                {probe.missing_expected_activity ? "activity missing" : "activity present"}
                {" · "}{probe.stable ? "stable" : "unstable"}
              </small>
            </div>
          ))}
        </div>
      ) : <p className="inline-empty">Not captured</p>}

      <h3>Software</h3>
      {hasSoftware ? (
        <div className="spec-chips">
          {commit.software_provider && <span>{commit.software_provider}</span>}
          {commit.software_repository && <span>{commit.software_repository}</span>}
          {commit.software_revision && <span>{commit.software_revision}</span>}
        </div>
      ) : <p className="inline-empty">Not captured</p>}

      <h3>Metadata</h3>
      <div className="spec-chips">
        <span>{commit.id}</span>
        <span>Project {commit.project_id}</span>
        <span>{commit.passport_baseline_ids?.length ? `Baseline #${commit.passport_baseline_ids.join(", #")}` : "No passport baseline captured"}</span>
      </div>
    </section>
  );
}

function CommitDiffPanel({ diff, commits, fromID, toID, onChangeFrom, onChangeTo }: {
  diff: PhysicalCommitDiff;
  commits: PhysicalCommit[];
  fromID: string;
  toID: string;
  onChangeFrom: (id: string) => void;
  onChangeTo: (id: string) => void;
}) {
  const fromLabel = commits.find((commit) => commit.id === fromID)?.display_id ?? fromID;
  const toLabel = commits.find((commit) => commit.id === toID)?.display_id ?? toID;

  return (
    <section className="panel history-detail">
      <span className="eyebrow">Physical Diff</span>
      <h2>{fromLabel} &rarr; {toLabel}</h2>
      <div className="form-row" style={{ marginBottom: 16 }}>
        <label>From<select value={fromID} onChange={(event) => onChangeFrom(event.target.value)}>{commits.map((commit) => <option key={commit.id} value={commit.id}>{commit.display_id}</option>)}</select></label>
        <label>To<select value={toID} onChange={(event) => onChangeTo(event.target.value)}>{commits.map((commit) => <option key={commit.id} value={commit.id}>{commit.display_id}</option>)}</select></label>
      </div>

      <DiffSection title="Visual" status={diff.visual.status} />

      <DiffSection title="Hardware" status={diff.components.status}>
        {diff.components.changes?.map((change) => <ComponentDiffRow key={change.component_id} change={change} />)}
      </DiffSection>

      <DiffSection title="Circuit" status={diff.circuit.status}>
        {diff.circuit.changes?.map((change) => <ConnectionDiffRow key={change.connection_id} change={change} />)}
      </DiffSection>

      <DiffSection title="Electrical" status={diff.electrical.status}>
        {diff.electrical.probes?.map((change) => <ProbeDiffRow key={change.probe} change={change} />)}
      </DiffSection>

      <DiffSection title="Software" status={diff.software.status}>
        {diff.software.fields?.map((field) => <FieldChangeRow key={field.field} change={field} />)}
      </DiffSection>
    </section>
  );
}

type Selection =
  | { kind: "view"; commitID: string }
  | { kind: "compare"; fromID: string; toID: string }
  | null;

export function PhysicalHistoryView({ project }: { project: Project | null }) {
  const [modalOpen, setModalOpen] = useState(false);
  const [lastCommitted, setLastCommitted] = useState<PhysicalCommit | null>(null);
  const [commits, setCommits] = useState<PhysicalCommit[]>([]);
  const [loadError, setLoadError] = useState("");

  const [selection, setSelection] = useState<Selection>(null);
  const [detail, setDetail] = useState<PhysicalCommitDetail | null>(null);
  const [diff, setDiff] = useState<PhysicalCommitDiff | null>(null);
  const [paneBusy, setPaneBusy] = useState(false);
  const [paneError, setPaneError] = useState("");

  useEffect(() => {
    if (!project) return;
    let active = true;
    physicalGitApi.list(project.id).then(({ items }) => { if (active) setCommits(items); }).catch((caught) => { if (active) setLoadError(errorMessage(caught)); });
    return () => { active = false; };
  }, [project]);

  useEffect(() => {
    if (!project || !selection) { setDetail(null); setDiff(null); return; }
    let active = true;
    setPaneBusy(true);
    setPaneError("");
    const request = selection.kind === "view"
      ? physicalGitApi.getDetail(project.id, selection.commitID).then((result) => { if (active) { setDetail(result); setDiff(null); } })
      : physicalGitApi.diff(project.id, selection.fromID, selection.toID).then((result) => { if (active) { setDiff(result); setDetail(null); } });
    request.catch((caught) => { if (active) setPaneError(errorMessage(caught)); }).finally(() => { if (active) setPaneBusy(false); });
    return () => { active = false; };
  }, [project, selection]);

  if (!project) {
    return (
      <section className="empty-state panel">
        <GitCommitHorizontal size={32} />
        <h2>Physical Git needs a real project</h2>
        <p>Create and analyze a project to start committing its physical state. The built-in demo does not use Physical Git.</p>
      </section>
    );
  }

  return (
    <>
      <section className="page-heading">
        <div><p className="kicker">Physical Git · Commit</p><h1>Physical History</h1><p>Capture what ReWeird currently knows about this project&apos;s hardware &mdash; the Circuit Map, the latest measurement, and an optional photo &mdash; as a point-in-time commit.</p></div>
        <div className="heading-actions">
          <button className="primary" onClick={() => setModalOpen(true)}><GitCommitHorizontal size={16} /> Commit Physical State</button>
        </div>
      </section>

      {lastCommitted && (
        <div className="input-security" role="status">
          <CheckCircle2 size={15} />
          <span>Physical state committed &middot; {lastCommitted.display_id} &middot; {formatTimestamp(lastCommitted.created_at_ms)}</span>
        </div>
      )}

      {loadError && <div className="form-error" role="alert"><AlertTriangle size={15} />{loadError}</div>}

      {commits.length === 0 ? (
        <section className="panel git-sync-panel">
          <h3>Commits</h3>
          <p>No physical commits yet. Commit the current state to start this project&apos;s Physical Git history.</p>
        </section>
      ) : (
        <section className="history-layout">
          <div className="panel history-list">
            {commits.map((commit, index) => {
              const previous = commits[index + 1];
              const selected = selection?.kind === "view" && selection.commitID === commit.id;
              return (
                <div key={commit.id} className={`history-item ${selected ? "selected" : ""}`}>
                  <button
                    type="button"
                    onClick={() => setSelection({ kind: "view", commitID: commit.id })}
                    style={{ flex: 1, textAlign: "left", background: "none", border: 0, padding: 0, color: "inherit", cursor: "pointer" }}
                  >
                    <span><strong>{commit.display_id}</strong><small>{formatTimestamp(commit.created_at_ms)}</small>{commit.note && <em>{commit.note}</em>}</span>
                  </button>
                  {previous && (
                    <button type="button" className="text-button" onClick={() => setSelection({ kind: "compare", fromID: previous.id, toID: commit.id })}>
                      <Zap size={12} /> Compare
                    </button>
                  )}
                </div>
              );
            })}
          </div>

          <div className="history-main">
            {paneBusy && <div className="analysis-progress"><RefreshCw className="spin" size={15} /><span>Loading&hellip;</span></div>}
            {paneError && <div className="form-error" role="alert"><AlertTriangle size={15} />{paneError}</div>}
            {!paneBusy && detail && <CommitDetailPanel detail={detail} />}
            {!paneBusy && diff && selection?.kind === "compare" && (
              <CommitDiffPanel
                diff={diff}
                commits={commits}
                fromID={selection.fromID}
                toID={selection.toID}
                onChangeFrom={(id) => setSelection({ kind: "compare", fromID: id, toID: selection.toID })}
                onChangeTo={(id) => setSelection({ kind: "compare", fromID: selection.fromID, toID: id })}
              />
            )}
            {!paneBusy && !detail && !diff && !paneError && (
              <section className="panel history-detail">
                <Cpu size={24} />
                <p>Select a commit to view its captured evidence, or Compare it against the previous one.</p>
              </section>
            )}
          </div>
        </section>
      )}

      {modalOpen && (
        <CommitPhysicalStateModal
          projectID={project.id}
          onClose={() => setModalOpen(false)}
          onCommitted={(commit) => {
            setModalOpen(false);
            setLastCommitted(commit);
            setCommits((current) => [commit, ...current]);
          }}
        />
      )}
    </>
  );
}
