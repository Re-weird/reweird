"use client";

import { useEffect, useRef, useState } from "react";
import { AlertTriangle, CheckCircle2, GitCommitHorizontal, Image as ImageIcon, RefreshCw, Upload, X } from "lucide-react";
import type { PhysicalCommit, Project } from "@reweird/shared-types";
import { ApiError, physicalGitApi } from "@/lib/api";

function errorMessage(error: unknown): string {
  return error instanceof ApiError ? error.message : "The request failed. Check the backend and try again.";
}

function formatTimestamp(createdAtMS: number): string {
  return new Date(createdAtMS).toLocaleString();
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

export function PhysicalHistoryView({ project }: { project: Project | null }) {
  const [modalOpen, setModalOpen] = useState(false);
  const [lastCommitted, setLastCommitted] = useState<PhysicalCommit | null>(null);
  const [commits, setCommits] = useState<PhysicalCommit[]>([]);
  const [loadError, setLoadError] = useState("");

  useEffect(() => {
    if (!project) return;
    let active = true;
    physicalGitApi.list(project.id).then(({ items }) => { if (active) setCommits(items); }).catch((caught) => { if (active) setLoadError(errorMessage(caught)); });
    return () => { active = false; };
  }, [project]);

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

      <section className="panel git-sync-panel">
        <h3>Commits</h3>
        {loadError && <div className="form-error" role="alert"><AlertTriangle size={15} />{loadError}</div>}
        {commits.length === 0 ? (
          <p>No physical commits yet. Commit the current state to start this project&apos;s Physical Git history.</p>
        ) : (
          <div className="git-files">
            {commits.map((commit) => (
              <div key={commit.id}>
                <code>{commit.display_id}</code>
                <small>{formatTimestamp(commit.created_at_ms)}{commit.note ? ` · ${commit.note}` : ""}</small>
              </div>
            ))}
          </div>
        )}
      </section>

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
