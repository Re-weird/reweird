"use client";

import { useEffect, useState } from "react";
import { GitBranch, ShieldCheck, TriangleAlert } from "lucide-react";
import { gitApi, type GitPreview } from "@/lib/api";

export function GitSyncPanel({ workflowID }: { workflowID: string }) {
  const [open, setOpen] = useState(false);
  const [preview, setPreview] = useState<GitPreview | null>(null);
  const [error, setError] = useState("");
  const [commit, setCommit] = useState("");
  const [busy, setBusy] = useState(false);
  useEffect(() => { setOpen(false); setPreview(null); setCommit(""); setError(""); }, [workflowID]);
  async function showPreview() {
    setOpen(true); setBusy(true); setError("");
    try { setPreview(await gitApi.preview(workflowID)); }
    catch (cause) { setError(cause instanceof Error ? cause.message : "Git preview unavailable."); }
    finally { setBusy(false); }
  }
  async function approve(push: boolean) {
    setBusy(true); setError("");
    try { const result = await gitApi.commit(workflowID, push); setCommit(`${result.commit.slice(0, 12)}${result.pushed ? " · pushed" : " · local commit"}`); setPreview(await gitApi.preview(workflowID)); }
    catch (cause) { setError(cause instanceof Error ? cause.message : "Git commit blocked."); }
    finally { setBusy(false); }
  }
  return <div className="panel git-sync-panel">
    {!open && <button className="secondary" onClick={showPreview}><GitBranch size={15} /> Preview Git sync</button>}
    {open && <><h3>Sync diagnostic report</h3>{busy && <p>Checking generated artifacts…</p>}{error && <p className="computer-error"><TriangleAlert size={15} /> {error}</p>}{commit && <p><ShieldCheck size={14} /> Commit {commit}</p>}
      {preview && <><div className="git-files">{preview.files.map((file) => <div key={file.path}><code>{file.path}</code><small>{file.bytes} bytes</small></div>)}</div><p>Security scan: {preview.secret_scan === "clear" ? "No known secret pattern found" : "Potential secret detected — commit blocked"}</p><p>Destination: {preview.repo_available ? "configured local project repository" : "no repository configured"}</p>{preview.detail && <p>{preview.detail}</p>}<div className="git-actions"><button className="secondary" onClick={() => setOpen(false)}>Keep local</button>{preview.commit_allowed && !commit && <button className="primary" disabled={busy} onClick={() => approve(false)}>Approve commit</button>}{preview.commit_allowed && preview.push_configured && !commit && <button className="secondary" disabled={busy} onClick={() => approve(true)}>Approve commit &amp; push</button>}</div></>}
    </>}
  </div>;
}
