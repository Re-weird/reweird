export interface BeatPose {
  /** Board rotation in radians. */
  rotY: number;
  rotX: number;
  /** Board position offset in scene units. */
  posX: number;
  posY: number;
  scale: number;
  /** 0..1 opacity-style blend values, interpolated between beats. */
  sensor: number;
  probes: number;
  fault: number;
  verified: number;
  ecosystem: number;
  cta: number;
  /** 0 = elements laid out linearly, 1 = fully arranged into the evidence orbit. */
  orbit: number;
}

export interface Beat extends BeatPose {
  id: string;
  kicker?: string;
  heading: string;
  body?: string;
}

const deg = (value: number) => (value * Math.PI) / 180;

export const beats: Beat[] = [
  {
    id: "hero",
    kicker: "Hardware diagnostics, guided by evidence",
    heading: "Understand your project.\nMeasure what is actually happening.\nFind the fault. Verify the fix.",
    rotY: deg(-18), rotX: deg(4), posX: 1.1, posY: 0, scale: 1,
    sensor: 0, probes: 0, fault: 0, verified: 0, ecosystem: 0, cta: 1, orbit: 0,
  },
  {
    id: "understand-signals",
    kicker: "First look",
    heading: "Your project already tells us a lot.",
    body: "ReWeird reads pin assignments and signal roles straight from your code.",
    rotY: deg(14), rotX: deg(2), posX: 0.6, posY: 0, scale: 1.05,
    sensor: 0, probes: 0, fault: 0, verified: 0, ecosystem: 0, cta: 0, orbit: 0,
  },
  {
    id: "understand",
    kicker: "Understand",
    heading: "ReWeird reads the project before diagnosing the hardware.",
    body: "Code. Components. Expected behavior.",
    rotY: deg(28), rotX: deg(0), posX: -0.4, posY: 0, scale: 1.05,
    sensor: 0, probes: 0, fault: 0, verified: 0, ecosystem: 0, cta: 0, orbit: 0,
  },
  {
    id: "connect",
    kicker: "Connect",
    heading: "Measure the signals that matter.",
    body: "P1 Power · P2 Trigger · P3 Echo",
    rotY: deg(46), rotX: deg(-2), posX: 0, posY: 0, scale: 1.05,
    sensor: 1, probes: 1, fault: 0, verified: 0, ecosystem: 0, cta: 0, orbit: 0,
  },
  {
    id: "orbit",
    kicker: "Evidence",
    heading: "One project. Multiple sources of evidence.",
    body: "Software. Measurements. Specifications. Baseline.",
    rotY: deg(70), rotX: deg(-2), posX: 0, posY: 0.1, scale: 0.92,
    sensor: 1, probes: 1, fault: 0, verified: 0, ecosystem: 0, cta: 0, orbit: 1,
  },
  {
    id: "fault",
    kicker: "Something changed",
    heading: "Something isn't behaving as expected.",
    body: "ReWeird doesn't guess. It follows the evidence.",
    rotY: deg(92), rotX: deg(0), posX: 0, posY: 0, scale: 0.98,
    sensor: 1, probes: 1, fault: 1, verified: 0, ecosystem: 0, cta: 0, orbit: 0.4,
  },
  {
    id: "diagnose",
    kicker: "Diagnose",
    heading: "AI reasons about the evidence.\nIt does not create the evidence.",
    body: "POWER 3.31 V — PASS · TRIG pulse — PASS · ECHO no response — FAIL",
    rotY: deg(112), rotX: deg(2), posX: -0.5, posY: 0, scale: 1,
    sensor: 1, probes: 1, fault: 1, verified: 0, ecosystem: 0, cta: 0, orbit: 0,
  },
  {
    id: "loop",
    kicker: "Closed loop",
    heading: "Diagnosis is a loop, not a guess.",
    body: "Understand → Measure → Diagnose → Test → Verify",
    rotY: deg(138), rotX: deg(0), posX: 0, posY: 0, scale: 1,
    sensor: 1, probes: 1, fault: 1, verified: 0, ecosystem: 0, cta: 0, orbit: 0.15,
  },
  {
    id: "verify",
    kicker: "Verify",
    heading: "Don't stop at finding the problem.\nProve the fix worked.",
    body: "ECHO — restored",
    rotY: deg(160), rotX: deg(0), posX: 0.4, posY: 0, scale: 1.02,
    sensor: 1, probes: 1, fault: 0, verified: 1, ecosystem: 0, cta: 0, orbit: 0,
  },
  {
    id: "ecosystem",
    kicker: "Beyond one board",
    heading: "Built for the hardware you build with.",
    body: "ESP32 · Arduino · Raspberry Pi",
    rotY: deg(184), rotX: deg(2), posX: 0, posY: 0, scale: 0.94,
    sensor: 0.4, probes: 0.4, fault: 0, verified: 1, ecosystem: 1, cta: 0, orbit: 0,
  },
  {
    id: "final",
    kicker: "Hardware fails.",
    heading: "Finding out why shouldn't be guesswork.",
    body: "Demo uses simulated hardware. No account required.",
    rotY: deg(198), rotX: deg(3), posX: 1.1, posY: 0, scale: 1,
    sensor: 0, probes: 0, fault: 0, verified: 0, ecosystem: 0, cta: 1, orbit: 0,
  },
];

const lerp = (a: number, b: number, t: number) => a + (b - a) * t;

export function poseAt(progress: number): BeatPose & { index: number; t: number } {
  const clamped = Math.min(1, Math.max(0, progress));
  const scaled = clamped * (beats.length - 1);
  const index = Math.min(beats.length - 2, Math.floor(scaled));
  const t = scaled - index;
  const a = beats[index];
  const b = beats[index + 1];
  return {
    index,
    t,
    rotY: lerp(a.rotY, b.rotY, t),
    rotX: lerp(a.rotX, b.rotX, t),
    posX: lerp(a.posX, b.posX, t),
    posY: lerp(a.posY, b.posY, t),
    scale: lerp(a.scale, b.scale, t),
    sensor: lerp(a.sensor, b.sensor, t),
    probes: lerp(a.probes, b.probes, t),
    fault: lerp(a.fault, b.fault, t),
    verified: lerp(a.verified, b.verified, t),
    ecosystem: lerp(a.ecosystem, b.ecosystem, t),
    cta: lerp(a.cta, b.cta, t),
    orbit: lerp(a.orbit, b.orbit, t),
  };
}

export function activeBeatIndex(progress: number): number {
  const clamped = Math.min(1, Math.max(0, progress));
  return Math.round(clamped * (beats.length - 1));
}
