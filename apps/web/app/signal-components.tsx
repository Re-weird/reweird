"use client";

import { Check, CircleDot, TriangleAlert } from "lucide-react";
import {
  Area,
  AreaChart,
  CartesianGrid,
  ResponsiveContainer,
  Tooltip,
  XAxis,
  YAxis,
} from "recharts";
import type { DemoSession, ProbeReading, RuleResult } from "@reweird/shared-types";

export function StatusDot({ status }: { status: ProbeReading["status"] }) {
  return <span className={`status-dot ${status}`} aria-label={status} />;
}

export function MiniChart({ values, danger = false }: { values: number[] | null; danger?: boolean }) {
  if (!values?.length) return <div className="mini-empty">No probe assigned</div>;
  const max = Math.max(...values, 1);
  const points = values
    .map((value, index) => `${values.length === 1 ? 60 : (index / (values.length - 1)) * 120},${35 - (value / max) * 29}`)
    .join(" ");
  return (
    <svg className="mini-chart" viewBox="0 0 120 38" preserveAspectRatio="none" role="img" aria-label="Recent signal trend">
      <polyline className={danger ? "danger-line" : "signal-line"} points={points} fill="none" />
    </svg>
  );
}

export function ProbeCard({ reading }: { reading: ProbeReading }) {
  const danger = reading.status === "intermittent";
  return (
    <article className={`probe-card ${danger ? "probe-alert" : ""} ${reading.status === "idle" ? "muted-card" : ""}`}>
      <div className="probe-top">
        <div>
          <span className="eyebrow">{reading.probe}</span>
          <h3>{reading.role}</h3>
        </div>
        <StatusDot status={reading.status} />
      </div>
      <div className="probe-reading">
        <strong>{reading.value === null ? "—" : reading.value.toFixed(reading.unit === "V" ? 2 : 1)}</strong>
        <span>{reading.unit}</span>
      </div>
      <MiniChart values={reading.samples} danger={danger} />
      <div className="probe-foot">
        <span className={`status-label ${reading.status}`}>{reading.status}</span>
        {reading.dropouts > 0 && <span>{reading.dropouts} dropouts/min</span>}
      </div>
    </article>
  );
}

export function RuleRow({ rule }: { rule: RuleResult }) {
  const Icon = rule.status === "pass" ? Check : rule.status === "warn" ? CircleDot : TriangleAlert;
  return (
    <div className={`rule-row ${rule.status}`}>
      <span className="rule-icon"><Icon size={15} /></span>
      <span>{rule.message}</span>
    </div>
  );
}

export function ConfidenceRing({ value }: { value: number }) {
  const degrees = Math.round(value * 360);
  return (
    <div className="confidence-ring" style={{ background: `conic-gradient(var(--cyan) ${degrees}deg, var(--line) 0deg)` }}>
      <div><strong>{Math.round(value * 100)}%</strong><span>confidence</span></div>
    </div>
  );
}

export function SignalChart({ session }: { session: DemoSession }) {
  const charted = session.probes.filter((probe) => probe.samples?.length).slice(0, 2);
  const rows = (charted[0]?.samples ?? []).map((_, index) => ({
    time: `${index * 5}s`,
    primary: charted[0]?.samples?.[index],
    secondary: charted[1]?.samples?.[index],
  }));
  return (
    <div className="chart-wrap">
      <ResponsiveContainer width="100%" height={240}>
        <AreaChart data={rows} margin={{ top: 12, right: 8, left: -25, bottom: 0 }}>
          <defs>
            <linearGradient id="echoGradient" x1="0" y1="0" x2="0" y2="1">
              <stop offset="0%" stopColor="var(--cyan)" stopOpacity={0.12} />
              <stop offset="100%" stopColor="var(--cyan)" stopOpacity={0} />
            </linearGradient>
          </defs>
          <CartesianGrid stroke="var(--line)" vertical={false} />
          <XAxis dataKey="time" stroke="var(--muted)" fontSize={11} tickLine={false} axisLine={false} />
          <YAxis stroke="var(--muted)" fontSize={11} tickLine={false} axisLine={false} />
          <Tooltip contentStyle={{ background: "var(--surface)", color: "var(--text)", border: "1px solid var(--line)", borderRadius: 10, fontSize: 12 }} />
          <Area type="linear" dataKey="primary" stroke="var(--cyan)" strokeWidth={1.5} fill="url(#echoGradient)" name={charted[0] ? `${charted[0].probe} ${charted[0].role}` : "Signal"} />
          {charted[1] && <Area type="linear" dataKey="secondary" stroke="var(--green)" strokeWidth={1.5} fill="transparent" name={`${charted[1].probe} ${charted[1].role}`} />}
        </AreaChart>
      </ResponsiveContainer>
    </div>
  );
}
