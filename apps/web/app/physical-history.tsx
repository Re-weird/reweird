"use client";

import { useEffect, useRef, useState } from "react";
import { AlertTriangle, Camera, CheckCircle2, Circle, Cpu, GitCommitHorizontal, Image as ImageIcon, RefreshCw, Save, ShieldCheck, Upload, Zap, X } from "lucide-react";
import type {
  CameraConfig,
  CameraStatus,
  CameraTestFrame,
  CameraTestResult,
  ComponentChange,
  ConnectionChange,
  EvidenceState,
  FieldChange,
  PhysicalCommit,
  PhysicalCommitDetail,
  PhysicalCommitDiff,
  PhysicalCommitVisionAnalysis,
  PhysicalRestorePlan,
  PhysicalVerifyResult,
  ProbeElectricalChange,
  Project,
  RestoreSection,
  SemanticVisionComponentChange,
  VerifyCategoryResult,
} from "@reweird/shared-types";
import { ApiError, cameraApi, PHYSICAL_GIT_DEMO_PROJECT_ID, physicalGitApi, physicalGitDemoApi } from "@/lib/api";

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

const restoreStatusLabel: Record<PhysicalRestorePlan["components"]["status"], string> = {
  MATCH: "Match",
  ACTION_REQUIRED: "Action required",
  VERIFY_REQUIRED: "Verification required",
  NOT_CAPTURED: "Not captured",
  UNAVAILABLE: "Insufficient evidence to compare",
};

const restoreStatusColor: Record<PhysicalRestorePlan["components"]["status"], string> = {
  MATCH: "var(--green)",
  ACTION_REQUIRED: "var(--red)",
  VERIFY_REQUIRED: "var(--amber)",
  NOT_CAPTURED: "var(--muted)",
  UNAVAILABLE: "var(--amber)",
};

const verifyStatusLabel: Record<PhysicalVerifyResult["overall"], string> = {
  SUPPORTED: "Supported by available evidence",
  NOT_SUPPORTED: "Not supported by available evidence",
  INCONCLUSIVE: "Inconclusive",
  NOT_CAPTURED: "Not captured",
  UNAVAILABLE: "Unavailable",
};

const verifyStatusColor: Record<PhysicalVerifyResult["overall"], string> = {
  SUPPORTED: "var(--green)",
  NOT_SUPPORTED: "var(--red)",
  INCONCLUSIVE: "var(--amber)",
  NOT_CAPTURED: "var(--muted)",
  UNAVAILABLE: "var(--amber)",
};

function RestoreActionIcon({ status }: { status: PhysicalRestorePlan["components"]["status"] }) {
  if (status === "MATCH") return <CheckCircle2 size={14} color="var(--green)" />;
  if (status === "ACTION_REQUIRED") return <AlertTriangle size={14} color="var(--red)" />;
  if (status === "VERIFY_REQUIRED") return <Circle size={14} color="var(--amber)" />;
  return <Circle size={14} color="var(--muted)" />;
}

function RestoreSectionView({ title, section }: { title: string; section: RestoreSection }) {
  return (
    <div style={{ marginBottom: 16 }}>
      <div className="panel-heading">
        <div><span className="eyebrow">{title}</span></div>
        <b style={{ color: restoreStatusColor[section.status], fontSize: 9, textTransform: "uppercase", letterSpacing: 0.6 }}>{restoreStatusLabel[section.status]}</b>
      </div>
      {section.actions?.map((action, index) => (
        <div key={`${action.title}-${index}`} style={{ display: "flex", gap: 8, padding: "6px 0", borderTop: "1px solid var(--line-soft)" }}>
          <RestoreActionIcon status={action.status} />
          <div style={{ flex: 1 }}>
            <div style={{ fontSize: 9, display: "flex", justifyContent: "space-between" }}>
              <span>{action.title}{action.ai_interpreted && <em style={{ marginLeft: 6, color: "var(--muted)" }}>AI-detected, unconfirmed</em>}</span>
              {(action.target_value || action.current_value) && <span style={{ color: "var(--muted)" }}>{action.current_value ?? "—"} &rarr; {action.target_value ?? "—"}</span>}
            </div>
            {action.description && <p className="inline-empty" style={{ textAlign: "left", padding: "2px 0 0", fontSize: 9 }}>{action.description}</p>}
          </div>
        </div>
      ))}
    </div>
  );
}

function VerifyCategoryView({ title, result }: { title: string; result: VerifyCategoryResult }) {
  return (
    <div style={{ display: "flex", justifyContent: "space-between", padding: "8px 0", borderTop: "1px solid var(--line-soft)" }}>
      <div>
        <span className="eyebrow">{title}</span>
        {result.detail && <p className="inline-empty" style={{ textAlign: "left", padding: "2px 0 0", fontSize: 9 }}>{result.detail}</p>}
      </div>
      <b style={{ color: verifyStatusColor[result.status], fontSize: 9, textTransform: "uppercase", letterSpacing: 0.6, whiteSpace: "nowrap" }}>{verifyStatusLabel[result.status]}</b>
    </div>
  );
}

function RestorePanel({ plan, targetLabel, sourceLabel, onVerify }: { plan: PhysicalRestorePlan; targetLabel: string; sourceLabel: string; onVerify: () => void }) {
  return (
    <section className="panel history-detail">
      <span className="eyebrow">Physical Restore</span>
      <h2>Restore {targetLabel}</h2>
      {plan.has_source ? <p className="inline-empty" style={{ textAlign: "left" }}>Reference state: {sourceLabel}</p> : <p className="inline-empty" style={{ textAlign: "left" }}>No reference commit is available -- restoration guidance is limited.</p>}

      <RestoreSectionView title="Components" section={plan.components} />
      <RestoreSectionView title="Circuit" section={plan.circuit} />
      <RestoreSectionView title="Electrical" section={plan.electrical} />
      <RestoreSectionView title="Visual" section={plan.visual} />
      <RestoreSectionView title="Software" section={plan.software} />

      <div className="modal-actions" style={{ justifyContent: "flex-start", marginTop: 8 }}>
        <button type="button" className="primary" onClick={onVerify}><ShieldCheck size={16} /> Verify restoration</button>
      </div>
    </section>
  );
}

function VerifyPanel({ result, targetLabel, onCommitRestored }: { result: PhysicalVerifyResult; targetLabel: string; onCommitRestored: () => void }) {
  return (
    <section className="panel history-detail">
      <span className="eyebrow">Physical Verify</span>
      <h2>Verify restoration &rarr; {targetLabel}</h2>
      <div className="spec-chips"><b style={{ color: verifyStatusColor[result.overall] }}>{result.summary}</b></div>

      <div style={{ marginTop: 12 }}>
        <VerifyCategoryView title="Components" result={result.components} />
        <VerifyCategoryView title="Circuit" result={result.circuit} />
        <VerifyCategoryView title="Electrical" result={result.electrical} />
        <VerifyCategoryView title="Visual" result={result.visual} />
        <VerifyCategoryView title="Software" result={result.software} />
      </div>

      <div className="modal-actions" style={{ justifyContent: "flex-start", marginTop: 8 }}>
        <button type="button" className="primary" onClick={onCommitRestored}><GitCommitHorizontal size={16} /> Commit restored state</button>
      </div>
    </section>
  );
}

// DemoStateControls is only ever rendered for the canonical Physical Git
// demo project. It flips ONLY that project's simulated live state between
// broken and working so a judge can watch the real Verify engine react --
// the backend hard-gates both actions to this one project id, and there is
// no equivalent action for a real project.
function DemoStateControls() {
  const [busy, setBusy] = useState(false);
  async function run(action: () => Promise<unknown>) {
    setBusy(true);
    try { await action(); window.location.reload(); } finally { setBusy(false); }
  }
  return (
    <div style={{ display: "flex", gap: 8 }}>
      <button type="button" className="secondary" disabled={busy} onClick={() => run(physicalGitDemoApi.applyRestoration)}>{busy ? <RefreshCw className="spin" size={14} /> : <ShieldCheck size={14} />} Apply simulated restoration</button>
      <button type="button" className="text-button" disabled={busy} onClick={() => run(physicalGitDemoApi.applyBreak)}><RefreshCw size={14} /> Reset demo</button>
    </div>
  );
}

const cameraStatusLabel: Record<CameraStatus, string> = {
  CONNECTED: "Connected",
  UNREACHABLE: "Unreachable",
  INVALID_STREAM: "Invalid stream",
  TIMEOUT: "Timed out",
  NOT_CONFIGURED: "Not configured",
};

// VisionCameraPanel configures a project's real MJPEG camera source. It
// never creates a Physical Commit and never calls Gemini -- Test Connection
// and Capture Test Frame only confirm connectivity/positioning. The camera
// itself is used automatically the next time COMMIT PHYSICAL STATE runs
// with no manually attached photo (see the backend's createPhysicalCommit).
function VisionCameraPanel({ projectID, cameraConfig, onChange }: { projectID: string; cameraConfig: CameraConfig | null; onChange: (config: CameraConfig | null) => void }) {
  const [url, setUrl] = useState(cameraConfig?.url ?? "");
  const [busy, setBusy] = useState<"save" | "test" | "capture" | "clear" | null>(null);
  const [error, setError] = useState("");
  const [testResult, setTestResult] = useState<CameraTestResult | null>(null);
  const [frame, setFrame] = useState<CameraTestFrame | null>(null);

  useEffect(() => { setUrl(cameraConfig?.url ?? ""); }, [cameraConfig?.url]);

  const trimmedURL = url.trim();
  const override = trimmedURL ? { source_type: "mjpeg" as const, url: trimmedURL } : undefined;

  async function save() {
    setBusy("save"); setError("");
    try {
      const updated = await cameraApi.saveConfig(projectID, { source_type: "mjpeg", url: trimmedURL });
      onChange(updated.camera_config ?? null);
    } catch (caught) { setError(errorMessage(caught)); }
    finally { setBusy(null); }
  }
  async function clear() {
    setBusy("clear"); setError(""); setTestResult(null); setFrame(null);
    try {
      const updated = await cameraApi.clearConfig(projectID);
      onChange(updated.camera_config ?? null);
      setUrl("");
    } catch (caught) { setError(errorMessage(caught)); }
    finally { setBusy(null); }
  }
  async function test() {
    setBusy("test"); setError(""); setFrame(null);
    try { setTestResult(await cameraApi.test(projectID, override)); }
    catch (caught) { setError(errorMessage(caught)); }
    finally { setBusy(null); }
  }
  async function captureFrame() {
    setBusy("capture"); setError(""); setTestResult(null);
    try { setFrame(await cameraApi.captureTestFrame(projectID, override)); }
    catch (caught) { setError(errorMessage(caught)); }
    finally { setBusy(null); }
  }

  return (
    <section className="panel" style={{ marginBottom: 16 }}>
      <div className="panel-heading"><div><span className="eyebrow">Real hardware evidence</span><h2 style={{ fontSize: 13, margin: "3px 0" }}><Camera size={14} style={{ verticalAlign: "-2px", marginRight: 4 }} />Vision camera</h2></div></div>
      <p className="inline-empty" style={{ textAlign: "left" }}>
        Configure an MJPEG camera (e.g. an Android phone running an &quot;IP Webcam&quot;-style app on the same Wi-Fi as this backend) to capture a real raw frame automatically each time you commit physical state with no photo attached.
      </p>
      <div className="upload-grid" style={{ gridTemplateColumns: "minmax(0,1fr) auto auto auto", alignItems: "center", gap: 8 }}>
        <input
          aria-label="Camera MJPEG URL"
          placeholder="http://10.110.194.207:4444"
          value={url}
          disabled={busy !== null}
          onChange={(event) => setUrl(event.target.value)}
          style={{ height: 36, borderRadius: 8, background: "var(--surface-2)", border: "1px solid var(--line)", color: "var(--text)", padding: "0 10px", fontSize: 12 }}
        />
        <button type="button" className="secondary" disabled={busy !== null || !trimmedURL} onClick={() => void save()}>{busy === "save" ? <RefreshCw className="spin" size={14} /> : <Save size={14} />} Save</button>
        <button type="button" className="secondary" disabled={busy !== null} onClick={() => void test()}>{busy === "test" ? <RefreshCw className="spin" size={14} /> : <Zap size={14} />} Test Connection</button>
        <button type="button" className="text-button" disabled={busy !== null} onClick={() => void captureFrame()}>{busy === "capture" ? <RefreshCw className="spin" size={14} /> : <ImageIcon size={14} />} Capture Test Frame</button>
      </div>
      {cameraConfig && <button type="button" className="text-button" disabled={busy !== null} onClick={() => void clear()} style={{ marginTop: 6 }}>Clear camera</button>}
      {error && <div className="form-error" role="alert"><AlertTriangle size={15} />{error}</div>}
      {testResult && (
        <p className="inline-empty" style={{ textAlign: "left" }}>
          Status: <strong>{cameraStatusLabel[testResult.status]}</strong>
          {testResult.frame_available && ` · frame available · ${testResult.content_type} · ${testResult.latency_ms}ms`}
          {testResult.message && ` · ${testResult.message}`}
        </p>
      )}
      {frame && (
        <div style={{ marginTop: 8 }}>
          {/* eslint-disable-next-line @next/next/no-img-element */}
          <img src={`data:${frame.content_type};base64,${frame.image_base64}`} alt="Captured test frame" style={{ maxWidth: 240, borderRadius: 8, border: "1px solid var(--line-soft)" }} />
          <p className="inline-empty" style={{ textAlign: "left" }}>{frame.width}&times;{frame.height} &middot; {(frame.size_bytes / 1024).toFixed(0)} KB &middot; this test frame was not saved or sent to Gemini.</p>
        </div>
      )}
    </section>
  );
}

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

function SemanticVisionComponentDiffRow({ change }: { change: SemanticVisionComponentChange }) {
  const prefix = change.status === "ADDED" ? "+" : change.status === "REMOVED" ? "-" : "~";
  return (
    <div style={{ padding: "6px 0", borderTop: "1px solid var(--line-soft)" }}>
      <div style={{ display: "flex", justifyContent: "space-between", fontSize: 9 }}>
        <span style={{ color: statusColor[change.status] }}>{prefix} {change.name}</span>
        <span style={{ color: "var(--muted)" }}>{change.before_count} &rarr; {change.after_count}</span>
      </div>
    </div>
  );
}

// SemanticVisualSection renders the AI interpretation layer of the Visual
// diff -- a deterministic comparison of two already-stored Gemini Vision
// analyses, kept visually and semantically separate from the raw image
// evidence comparison above it. It never triggers Analyze Hardware itself:
// analysis only ever happens from the commit detail view, on request.
function SemanticVisualSection({ diff, fromLabel, toLabel }: { diff: PhysicalCommitDiff; fromLabel: string; toLabel: string }) {
  const semantic = diff.semantic_visual;
  return (
    <div style={{ border: "1px solid var(--line-soft)", borderRadius: 10, padding: 12, marginTop: 4, marginBottom: 16 }}>
      <div className="panel-heading">
        <div><span className="eyebrow">AI interpretation, not evidence</span><h2 style={{ fontSize: 12, margin: "3px 0" }}>Detected Hardware Comparison</h2></div>
        <b style={{ color: statusColor[semantic.status], fontSize: 9, textTransform: "uppercase", letterSpacing: 0.6 }}>{statusLabel[semantic.status]}</b>
      </div>

      {semantic.status === "NOT_CAPTURED" && (
        <p className="inline-empty">Neither commit has been analyzed. Analyze both from their commit detail views to compare detected hardware.</p>
      )}

      {semantic.status === "UNAVAILABLE" && (
        <>
          <p className="inline-empty">Insufficient analyzed evidence to compare.</p>
          <div className="spec-chips" style={{ marginTop: 4 }}>
            <span>{fromLabel}: {semantic.from_analyzed ? "analyzed" : "not analyzed"}</span>
            <span>{toLabel}: {semantic.to_analyzed ? "analyzed" : "not analyzed"}</span>
          </div>
          <p className="inline-empty" style={{ marginTop: 4 }}>Analyze both commits from their commit detail views to enable semantic comparison.</p>
        </>
      )}

      {semantic.status === "UNCHANGED" && (
        <p className="inline-empty">Detected components are the same on both sides.</p>
      )}

      {semantic.status === "CHANGED" && semantic.changes?.map((change) => <SemanticVisionComponentDiffRow key={change.key} change={change} />)}
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
  defaultNote,
  cameraConfigured,
}: {
  projectID: string;
  onClose: () => void;
  onCommitted: (commit: PhysicalCommit) => void;
  defaultNote?: string;
  cameraConfigured?: boolean;
}) {
  const [note, setNote] = useState(defaultNote ?? "");
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
        {cameraConfigured && !image && (
          <p className="inline-empty" style={{ textAlign: "left" }}>
            A frame will be captured automatically from your configured camera since no photo is attached above.
          </p>
        )}
        {error && <div className="form-error" role="alert"><AlertTriangle size={15} />{error}</div>}
        <div className="modal-actions">
          <button type="button" className="secondary" onClick={onClose} disabled={busy}>Cancel</button>
          <button className="primary" type="submit" disabled={busy}>{busy ? <RefreshCw className="spin" size={16} /> : <GitCommitHorizontal size={16} />} Commit physical state</button>
        </div>
      </form>
    </div>
  );
}

// Confidence below this threshold is flagged "Needs confirmation" in the UI.
// This is purely a display grouping, not a new backend fact -- every
// component already carries its real confidence value regardless.
const NEEDS_CONFIRMATION_THRESHOLD = 0.9;

function VisualEvidenceSection({ commit }: { commit: PhysicalCommit }) {
  const [visionAnalysis, setVisionAnalysis] = useState<PhysicalCommitVisionAnalysis | null>(null);
  const [busy, setBusy] = useState(false);
  const [loaded, setLoaded] = useState(false);
  const [error, setError] = useState("");

  useEffect(() => {
    setVisionAnalysis(null);
    setLoaded(false);
    setError("");
    if (!commit.image) return;
    let active = true;
    physicalGitApi.getVisionAnalysis(commit.project_id, commit.id)
      .then((result) => { if (active) { setVisionAnalysis(result); setLoaded(true); } })
      .catch((caught) => { if (active) { setError(errorMessage(caught)); setLoaded(true); } });
    return () => { active = false; };
  }, [commit.id, commit.project_id, commit.image]);

  async function analyze() {
    setBusy(true);
    setError("");
    try {
      setVisionAnalysis(await physicalGitApi.analyzeHardware(commit.project_id, commit.id));
    } catch (caught) {
      setError(errorMessage(caught));
    } finally {
      setBusy(false);
    }
  }

  if (!commit.image) {
    return (
      <>
        <p className="inline-empty">Not captured</p>
        <p className="inline-empty">Hardware image analysis unavailable because this commit has no captured image.</p>
      </>
    );
  }

  const needsConfirmation = visionAnalysis?.analysis.components.filter((component) => component.confidence < NEEDS_CONFIRMATION_THRESHOLD) ?? [];

  return (
    <>
      <div className="analysis-source-row">
        <ImageIcon size={17} />
        <div><strong>{commit.image.original_filename}</strong><span>{commit.image.content_type} &middot; {(commit.image.size_bytes / 1024).toFixed(0)} KB</span></div>
      </div>
      <p className="inline-empty" style={{ textAlign: "left", padding: "4px 0" }}>Raw image evidence &mdash; captured {formatTimestamp(commit.created_at_ms)}.</p>

      <div style={{ border: "1px solid var(--line-soft)", borderRadius: 10, padding: 12, marginTop: 8 }}>
        <div className="panel-heading"><div><span className="eyebrow">AI interpretation, not evidence</span><h2 style={{ fontSize: 12, margin: "3px 0" }}>AI Analysis</h2></div></div>

        {!loaded && !error && <div className="analysis-progress"><RefreshCw className="spin" size={15} /><span>Loading&hellip;</span></div>}
        {error && <div className="form-error" role="alert"><AlertTriangle size={15} />{error}</div>}

        {loaded && !visionAnalysis && (
          <>
            <p className="inline-empty">Not analyzed</p>
            <button type="button" className="secondary" disabled={busy} onClick={analyze}>{busy ? <RefreshCw className="spin" size={16} /> : <Zap size={16} />} Analyze Hardware</button>
          </>
        )}

        {visionAnalysis && (
          <>
            {visionAnalysis.analysis.status === "VISION_COMPLETE" && visionAnalysis.analysis.components.length === 0 && (
              <p className="inline-empty">Gemini Vision did not identify any components in this photo.</p>
            )}
            <div className="git-files">
              {visionAnalysis.analysis.components.map((component, index) => (
                <div key={`${component.catalog_id || component.name}-${index}`}>
                  <code>{component.name}</code>
                  <small>confidence: {Math.round(component.confidence * 100)}%</small>
                </div>
              ))}
            </div>
            {needsConfirmation.length > 0 && (
              <div style={{ marginTop: 8 }}>
                <span className="eyebrow">Needs confirmation</span>
                <ul style={{ margin: "4px 0 0", paddingLeft: 16, fontSize: 9, color: "var(--muted)" }}>
                  {needsConfirmation.map((component, index) => <li key={index}>{component.name} &mdash; confidence below {Math.round(NEEDS_CONFIRMATION_THRESHOLD * 100)}%</li>)}
                </ul>
              </div>
            )}
            <div className="spec-chips" style={{ marginTop: 8 }}><span>Analyzed {formatTimestamp(visionAnalysis.created_at_ms)}</span></div>
            <button type="button" className="text-button" disabled={busy} onClick={analyze} style={{ marginTop: 8 }}>{busy ? <RefreshCw className="spin" size={14} /> : <Upload size={14} />} Re-analyze</button>
          </>
        )}
      </div>
    </>
  );
}

function CommitDetailPanel({ detail, onRestore }: { detail: PhysicalCommitDetail; onRestore: () => void }) {
  const { commit, measurement } = detail;
  const components = commit.profile_snapshot?.components ?? null;
  const connections = commit.profile_snapshot?.connections ?? null;
  const hasSoftware = Boolean(commit.software_provider || commit.software_repository || commit.software_revision);

  return (
    <section className="panel history-detail">
      <div className="panel-heading">
        <div><span className="eyebrow">Physical Commit</span><h2 style={{ margin: "3px 0" }}>{commit.display_id}</h2></div>
        <button type="button" className="secondary" onClick={onRestore}><ShieldCheck size={15} /> Restore this state</button>
      </div>
      <div className="spec-chips"><span>{formatTimestamp(commit.created_at_ms)}</span></div>
      {commit.note && <p>{commit.note}</p>}

      <h3>Visual</h3>
      <VisualEvidenceSection commit={commit} />

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
      <SemanticVisualSection diff={diff} fromLabel={fromLabel} toLabel={toLabel} />

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
  | { kind: "restore"; commitID: string }
  | { kind: "verify"; commitID: string }
  | null;

export function PhysicalHistoryView({ project }: { project: Project | null }) {
  const [modalOpen, setModalOpen] = useState(false);
  const [modalDefaultNote, setModalDefaultNote] = useState<string | undefined>(undefined);
  const [lastCommitted, setLastCommitted] = useState<PhysicalCommit | null>(null);
  const [commits, setCommits] = useState<PhysicalCommit[]>([]);
  const [loadError, setLoadError] = useState("");

  const [selection, setSelection] = useState<Selection>(null);
  const [detail, setDetail] = useState<PhysicalCommitDetail | null>(null);
  const [diff, setDiff] = useState<PhysicalCommitDiff | null>(null);
  const [restorePlan, setRestorePlan] = useState<PhysicalRestorePlan | null>(null);
  const [verifyResult, setVerifyResult] = useState<PhysicalVerifyResult | null>(null);
  const [paneBusy, setPaneBusy] = useState(false);
  const [paneError, setPaneError] = useState("");
  const [cameraConfig, setCameraConfig] = useState<CameraConfig | null>(project?.camera_config ?? null);

  useEffect(() => { setCameraConfig(project?.camera_config ?? null); }, [project?.id, project?.camera_config]);

  useEffect(() => {
    if (!project) return;
    let active = true;
    physicalGitApi.list(project.id).then(({ items }) => { if (active) setCommits(items); }).catch((caught) => { if (active) setLoadError(errorMessage(caught)); });
    return () => { active = false; };
  }, [project]);

  useEffect(() => {
    if (!project || !selection) { setDetail(null); setDiff(null); setRestorePlan(null); setVerifyResult(null); return; }
    let active = true;
    setPaneBusy(true);
    setPaneError("");
    const request = selection.kind === "view"
      ? physicalGitApi.getDetail(project.id, selection.commitID).then((result) => { if (active) { setDetail(result); setDiff(null); setRestorePlan(null); setVerifyResult(null); } })
      : selection.kind === "compare"
      ? physicalGitApi.diff(project.id, selection.fromID, selection.toID).then((result) => { if (active) { setDiff(result); setDetail(null); setRestorePlan(null); setVerifyResult(null); } })
      : selection.kind === "restore"
      ? physicalGitApi.restore(project.id, selection.commitID).then((result) => { if (active) { setRestorePlan(result); setDetail(null); setDiff(null); setVerifyResult(null); } })
      : physicalGitApi.verify(project.id, selection.commitID).then((result) => { if (active) { setVerifyResult(result); setDetail(null); setDiff(null); setRestorePlan(null); } });
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

  const isDemoProject = project.id === PHYSICAL_GIT_DEMO_PROJECT_ID;

  return (
    <>
      <section className="page-heading">
        <div>
          <p className="kicker">Physical Git · Commit</p>
          <h1>Physical History{isDemoProject && <span className="confirmed" style={{ marginLeft: 10, background: "var(--amber-soft, rgba(230,160,40,.18))", color: "var(--amber)" }}>DEMO MODE · SIMULATED</span>}</h1>
          <p>{isDemoProject ? "Simulated evidence, real Physical Git behavior. No ESP32, camera, GitHub, or Gemini call is required to see this history." : "Capture what ReWeird currently knows about this project's hardware — the Circuit Map, the latest measurement, and an optional photo — as a point-in-time commit."}</p>
        </div>
        <div className="heading-actions">
          {isDemoProject && <DemoStateControls />}
          <button className="primary" onClick={() => setModalOpen(true)}><GitCommitHorizontal size={16} /> Commit Physical State</button>
        </div>
      </section>

      {!isDemoProject && <VisionCameraPanel projectID={project.id} cameraConfig={cameraConfig} onChange={setCameraConfig} />}

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
                    <span className="spec-chips" style={{ marginTop: 4 }}>
                      {commit.image && <span>PHOTO</span>}
                      {(commit.profile_snapshot?.components?.length || commit.profile_snapshot?.connections?.length) ? <span>CIRCUIT</span> : null}
                      {commit.measurement_id != null && <span>MEASUREMENT</span>}
                      {commit.software_revision && <span>SOFTWARE</span>}
                    </span>
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
            {!paneBusy && detail && <CommitDetailPanel detail={detail} onRestore={() => setSelection({ kind: "restore", commitID: detail.commit.id })} />}
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
            {!paneBusy && restorePlan && selection?.kind === "restore" && (
              <RestorePanel
                plan={restorePlan}
                targetLabel={commits.find((commit) => commit.id === selection.commitID)?.display_id ?? selection.commitID}
                sourceLabel={commits.find((commit) => commit.id === restorePlan.source_commit)?.display_id ?? restorePlan.source_commit ?? "—"}
                onVerify={() => setSelection({ kind: "verify", commitID: selection.commitID })}
              />
            )}
            {!paneBusy && verifyResult && selection?.kind === "verify" && (
              <VerifyPanel
                result={verifyResult}
                targetLabel={commits.find((commit) => commit.id === selection.commitID)?.display_id ?? selection.commitID}
                onCommitRestored={() => {
                  const target = commits.find((commit) => commit.id === selection.commitID);
                  setModalDefaultNote(target ? `Restored toward ${target.display_id}` : undefined);
                  setModalOpen(true);
                }}
              />
            )}
            {!paneBusy && !detail && !diff && !restorePlan && !verifyResult && !paneError && (
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
          defaultNote={modalDefaultNote}
          cameraConfigured={cameraConfig != null}
          onClose={() => { setModalOpen(false); setModalDefaultNote(undefined); }}
          onCommitted={(commit) => {
            setModalOpen(false);
            setModalDefaultNote(undefined);
            setLastCommitted(commit);
            setCommits((current) => [commit, ...current]);
          }}
        />
      )}
    </>
  );
}
