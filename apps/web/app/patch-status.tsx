"use client";

import { useEffect, useState } from "react";
import { patchApi } from "@/lib/api";

export function PatchStatus() {
  const [detail, setDetail] = useState("Physical output disabled. Checking controller status…");
  useEffect(() => {
    let current = true;
    patchApi.status().then((status) => {
      if (current) setDetail(status.detail);
    }).catch(() => {
      if (current) setDetail("Controller unavailable. Physical output remains locked; no execution is claimed.");
    });
    return () => { current = false; };
  }, []);
  return <section className="panel guided-panel" aria-label="PATCH hardware safety">
    <p className="kicker">PATCH LOCKED · Master enable OFF</p>
    <h2>Active hardware tests are unavailable</h2>
    <p>{detail}</p>
    <p>No dedicated protected PATCH output is configured. P1–P6 remain measurement inputs. Serial connection does not unlock PATCH.</p>
    <button className="primary" disabled title="Requires a verified dedicated protection interface and firmware arming handshake">Approve active test — unavailable</button>
  </section>;
}
