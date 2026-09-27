"use client";

import { useState } from "react";
import { bridgeApi } from "@/lib/api";

export function USBBridgePanel({projectID}: {projectID: string}) {
  const [device, setDevice] = useState("");
  const [wireProfile, setWireProfile] = useState("");
  const [confirmed, setConfirmed] = useState(false);
  const [busy, setBusy] = useState(false);
  const [error, setError] = useState("");
  const [pair, setPair] = useState<{token: string; share_token: string; expires_at_ms: number} | null>(null);
  const [origin, setOrigin] = useState("");
  async function connect() {
    setBusy(true); setError(""); setPair(null);
    try { const result = await bridgeApi.pair(projectID, device.trim(), wireProfile.trim()); setOrigin(window.location.origin); setPair(result); }
    catch (e) { setError(e instanceof Error ? e.message : "Pairing failed"); }
    finally { setBusy(false); }
  }
  async function revoke() {
    setBusy(true); setError("");
    try { await bridgeApi.revoke(projectID); setPair(null); }
    catch (e) { setError(e instanceof Error ? e.message : "Revocation failed"); }
    finally { setBusy(false); }
  }
  return <details className="panel" style={{margin: "16px 0", padding: 20}}>
    <summary style={{cursor: "pointer", fontWeight: 600}}>Connect USB hardware to this hosted project / Share with judges</summary>
    <p>Keep the ESP32 plugged into your laptop. This read-only bridge forwards REAL SERIAL measurements to this project. PATCH stays locked.</p>
    <p>First confirm this project’s physical profile and probe placement. Pairing expires after 12 hours; pairing again revokes previous bridge and judge credentials.</p>
    <div style={{display: "grid", gap: 12, maxWidth: 650}}>
      <label>Device ID (from real telemetry)<input value={device} onChange={e => setDevice(e.target.value)} placeholder="reweird-3428B5AD4F7C" maxLength={128} /></label>
      <label>Firmware profile ID<input value={wireProfile} onChange={e => setWireProfile(e.target.value)} placeholder="ultrasonic-servo-bench-0e43097d23" maxLength={128} /></label>
      <label><input type="checkbox" checked={confirmed} onChange={e => setConfirmed(e.target.checked)} /> I confirm this device and firmware profile describe this project’s exact probe mapping. Original firmware IDs remain recorded.</label>
      <div><button type="button" className="primary" disabled={busy || !confirmed || !device.trim() || !wireProfile.trim()} onClick={() => void connect()}>Pair USB bridge</button>{" "}<button type="button" className="secondary" disabled={busy} onClick={() => void revoke()}>Revoke bridge and judge link</button></div>
    </div>
    {error && <p role="alert">{error}</p>}
    {pair && <div style={{overflowWrap: "anywhere"}}>
      <p>Stop the local API/serial monitor to free COM5. In PowerShell, from <code>apps/api</code>, run:</p>
      <pre style={{whiteSpace: "pre-wrap"}}>{`$secret = Read-Host 'Paste bridge token' -AsSecureString\n$env:REWEIRD_BRIDGE_TOKEN = [System.Net.NetworkCredential]::new('', $secret).Password\ngo run ./cmd/usb-bridge --url "${origin}" --project "${projectID}" --port COM5\nRemove-Item Env:REWEIRD_BRIDGE_TOKEN`}</pre>
      <p>Private bridge token—copy only into the prompt above, never send to judges:</p>
      <input type="password" readOnly value={pair.token} aria-label="Private bridge token" onFocus={e => e.target.select()} />{" "}<button type="button" className="secondary" onClick={() => void navigator.clipboard.writeText(pair.token).catch(() => setError("Clipboard unavailable; select and copy the token field."))}>Copy private token</button>
      <p>Read-only judge link (anyone with this link can view live probe readings until expiry or revocation):</p>
      <a href={`/live/${encodeURIComponent(projectID)}#${pair.share_token}`} target="_blank" rel="noreferrer">{origin}/live/{projectID}#{pair.share_token}</a>
      <p>Expires: {new Date(pair.expires_at_ms).toLocaleString()}. A device reboot requires pairing again. Closing this panel does not stop the bridge.</p>
    </div>}
  </details>;
}
