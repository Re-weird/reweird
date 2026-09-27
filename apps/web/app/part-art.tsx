"use client";

// Flat 2D drawings of the real hardware, used by the circuit-map schematic
// and as thumbnails in the component list. Drawn like product renders, so
// they keep fixed material colors (PCB, metal, plastic) in both themes.

export type PartKind = "ultrasonic" | "servo" | "led" | "button" | "generic";

export function partKind(...hints: (string | undefined)[]): PartKind {
  const text = hints.filter(Boolean).join(" ").toLowerCase();
  if (/hc-?sr04|ultrasonic|sonar|distance/.test(text)) return "ultrasonic";
  if (/servo|sg90|mg90/.test(text)) return "servo";
  if (/\bled\b|light-emitting|diode/.test(text)) return "led";
  if (/button|switch|push/.test(text)) return "button";
  return "generic";
}

const SILK = "#d7dde3";
const GOLD = "#b99b5c";

/** A header pin: gold pad plus silkscreen label, on the left or right edge. */
function HeaderPin({ x, y, label, side }: { x: number; y: number; label: string; side: "left" | "right" }) {
  return (
    <g>
      <rect x={x - 5} y={y - 5} width={10} height={10} rx={1.5} fill={GOLD} />
      <circle cx={x} cy={y} r={2.2} fill="#3a3222" />
      <text x={side === "left" ? x + 11 : x - 11} y={y + 3.5} textAnchor={side === "left" ? "start" : "end"} fill={SILK} fillOpacity={0.85} className="font-mono text-[9.5px] font-semibold">{label}</text>
    </g>
  );
}

/** ESP32 dev board: dark PCB, PCB antenna, the CPU package, USB, EN/BOOT buttons, header on the right. */
export function Esp32Board({ x, y, w, h, pins }: { x: number; y: number; w: number; h: number; pins: { y: number; label: string }[] }) {
  const cpuSize = Math.min(64, h - 40);
  const cpuX = x + 22;
  const cpuY = y + (h - cpuSize) / 2 + 4;
  const legs = 9;
  const step = cpuSize / (legs + 1);
  return (
    <g>
      <rect x={x} y={y} width={w} height={h} rx={7} fill="#172029" stroke="#2c3844" />
      {/* mounting holes */}
      {[[x + 9, y + 9], [x + 9, y + h - 9]].map(([cx, cy]) => <circle key={`${cx}-${cy}`} cx={cx} cy={cy} r={3.2} fill="none" stroke={GOLD} strokeOpacity={0.6} />)}
      {/* meander PCB antenna along the top */}
      <path d={`M ${x + 22} ${y + 12} h 8 v 8 h 8 v -8 h 8 v 8 h 8 v -8 h 8 v 8 h 8 v -8 h 8`} fill="none" stroke={GOLD} strokeOpacity={0.55} strokeWidth={1.4} />
      {/* CPU package with legs on all four sides */}
      <g>
        {Array.from({ length: legs }, (_, index) => {
          const offset = step * (index + 1);
          return (
            <g key={index} stroke="#aeb6bf" strokeWidth={1.2}>
              <line x1={cpuX + offset} y1={cpuY - 4} x2={cpuX + offset} y2={cpuY} />
              <line x1={cpuX + offset} y1={cpuY + cpuSize} x2={cpuX + offset} y2={cpuY + cpuSize + 4} />
              <line x1={cpuX - 4} y1={cpuY + offset} x2={cpuX} y2={cpuY + offset} />
              <line x1={cpuX + cpuSize} y1={cpuY + offset} x2={cpuX + cpuSize + 4} y2={cpuY + offset} />
            </g>
          );
        })}
        <rect x={cpuX} y={cpuY} width={cpuSize} height={cpuSize} rx={3} fill="#22272d" stroke="#3a424b" />
        <rect x={cpuX + 5} y={cpuY + 5} width={cpuSize - 10} height={cpuSize - 10} rx={2} fill="none" stroke="#343c45" />
        <circle cx={cpuX + 9} cy={cpuY + 9} r={1.8} fill="#56606b" />
        <text x={cpuX + cpuSize / 2} y={cpuY + cpuSize / 2 + 1} textAnchor="middle" fill={SILK} className="font-mono text-[11px] font-bold tracking-wider">ESP32</text>
        <text x={cpuX + cpuSize / 2} y={cpuY + cpuSize / 2 + 12} textAnchor="middle" fill={SILK} fillOpacity={0.5} className="font-mono text-[6.5px]">D0WD · 240MHz</text>
      </g>
      {/* USB connector and buttons at the bottom edge */}
      <rect x={x + w / 2 - 34} y={y + h - 10} width={22} height={12} rx={2} fill="#9aa3ac" stroke="#6f7881" />
      {[x + w / 2 + 2, x + w / 2 + 20].map((bx, index) => (
        <g key={bx}>
          <rect x={bx} y={y + h - 18} width={12} height={10} rx={1.5} fill="#2d333a" stroke="#48515b" />
          <circle cx={bx + 6} cy={y + h - 13} r={2.6} fill="#3d454e" />
          <text x={bx + 6} y={y + h - 21} textAnchor="middle" fill={SILK} fillOpacity={0.5} className="font-mono text-[5.5px]">{index ? "BOOT" : "EN"}</text>
        </g>
      ))}
      {/* power LED */}
      <circle cx={x + w - 22} cy={y + 14} r={2.2} fill="#c0504d" />
      {pins.map((pin) => <HeaderPin key={`${pin.label}-${pin.y}`} x={x + w - 12} y={pin.y} label={pin.label} side="right" />)}
    </g>
  );
}

function Transducer({ cx, cy, r, label }: { cx: number; cy: number; r: number; label: string }) {
  return (
    <g>
      <circle cx={cx} cy={cy} r={r} fill="#aab3bb" stroke="#7b848c" strokeWidth={1.2} />
      <circle cx={cx} cy={cy} r={r - 4} fill="#3b4249" />
      <circle cx={cx} cy={cy} r={r - 4} fill="url(#mesh)" />
      <circle cx={cx} cy={cy} r={r - 4} fill="none" stroke="#59626b" />
      <text x={cx} y={cy + r + 11} textAnchor="middle" fill={SILK} fillOpacity={0.7} className="font-mono text-[8px] font-bold">{label}</text>
    </g>
  );
}

/** One part drawn at its real shape, with header pins on its left edge at the given heights. */
export function PartArt({ kind, x, y, w, h, pins }: { kind: PartKind; x: number; y: number; w: number; h: number; pins: { y: number; label: string }[] }) {
  const pinX = x + 12;
  const bodyX = x + 82;
  const bodyW = w - 90;
  const cy = y + h / 2;
  const headers = pins.map((pin) => <HeaderPin key={`${pin.label}-${pin.y}`} x={pinX} y={pin.y} label={pin.label} side="left" />);
  // Wires leave from just past the pin labels, not through them.
  const wireX = pinX + 52;
  // Modules other than the HC-SR04 sit on a small breakout board.
  const carrier = (
    <g>
      <rect x={x} y={y} width={w} height={h} rx={6} fill="#1a2129" stroke="#2c3844" />
      <rect x={x + 4} y={y + 4} width={48} height={h - 8} rx={3} fill="none" stroke={SILK} strokeOpacity={0.08} />
      {[[x + 7, y + 7], [x + w - 7, y + 7], [x + 7, y + h - 7], [x + w - 7, y + h - 7]].map(([hx, hy]) => <circle key={`${hx}-${hy}`} cx={hx} cy={hy} r={2.2} fill="#10161c" stroke={GOLD} strokeOpacity={0.45} />)}
    </g>
  );

  if (kind === "ultrasonic") {
    const r = Math.min(26, (bodyW - 14) / 4, h / 2 - 16);
    return (
      <g>
        <rect x={x} y={y} width={w} height={h} rx={6} fill="#23466b" stroke="#35618f" />
        {[[x + 7, y + 7], [x + w - 7, y + 7], [x + 7, y + h - 7], [x + w - 7, y + h - 7]].map(([hx, hy]) => <circle key={`${hx}-${hy}`} cx={hx} cy={hy} r={2.4} fill="#16304b" stroke={GOLD} strokeOpacity={0.5} />)}
        <Transducer cx={bodyX + r + 4} cy={cy - 4} r={r} label="T" />
        <Transducer cx={bodyX + bodyW - r - 4} cy={cy - 4} r={r} label="R" />
        {/* crystal oscillator between the transducers */}
        <rect x={bodyX + bodyW / 2 - 7} y={cy - 12} width={14} height={20} rx={5} fill="#c3cad0" stroke="#8b949c" />
        <text x={bodyX + bodyW / 2} y={y + 13} textAnchor="middle" fill={SILK} fillOpacity={0.75} className="font-mono text-[8px] font-bold tracking-widest">HC-SR04</text>
        {headers}
      </g>
    );
  }

  if (kind === "servo") {
    const bodyH = Math.min(h - 24, 62);
    const top = cy - bodyH / 2;
    const hubX = bodyX + bodyW * 0.62;
    return (
      <g>
        {carrier}
        {/* ribbon cable from the header into the servo */}
        {pins.map((pin, index) => (
          <path key={pin.label} d={`M ${wireX} ${pin.y} C ${wireX + 14} ${pin.y}, ${bodyX - 18} ${cy - 6 + index * 5}, ${bodyX + 4} ${cy - 6 + index * 5}`} fill="none" stroke={["#b5483f", "#c9823a", "#6b4a36"][index % 3]} strokeWidth={2.4} strokeLinecap="round" />
        ))}
        <rect x={bodyX - 12} y={top + bodyH * 0.35} width={bodyW + 24} height={bodyH * 0.3} rx={3} fill="#2f5486" stroke="#446fa6" />
        {[bodyX - 6, bodyX + bodyW + 6].map((ex) => <circle key={ex} cx={ex} cy={cy} r={2.6} fill="#172a44" />)}
        <rect x={bodyX} y={top} width={bodyW} height={bodyH} rx={5} fill="#3a66a0" fillOpacity={0.92} stroke="#5582bd" />
        <circle cx={hubX} cy={top + bodyH * 0.45} r={bodyH * 0.26} fill="#2d5282" stroke="#5582bd" />
        {/* horn */}
        <g transform={`translate(${hubX} ${top + bodyH * 0.45})`}>
          <path d={`M ${-bodyH * 0.55} -4 L ${bodyH * 0.55} -2.5 L ${bodyH * 0.55} 2.5 L ${-bodyH * 0.55} 4 Z`} fill="#e7eaed" stroke="#b9bfc5" />
          <circle r={5} fill="#f2f4f6" stroke="#b9bfc5" />
          <circle r={1.6} fill="#8a9097" />
        </g>
        <text x={bodyX + 8} y={top + bodyH - 8} fill={SILK} fillOpacity={0.8} className="font-mono text-[7.5px] font-bold">SG90</text>
        {headers}
      </g>
    );
  }

  if (kind === "led") {
    const lx = bodyX + bodyW / 2;
    return (
      <g>
        {carrier}
        {pins.map((pin, index) => <path key={pin.label} d={`M ${wireX} ${pin.y} H ${lx - 5 + index * 10} V ${cy + 8}`} fill="none" stroke="#aeb6bf" strokeWidth={1.6} />)}
        <path d={`M ${lx - 13} ${cy + 10} V ${cy - 8} a 13 13 0 0 1 26 0 V ${cy + 10} Z`} fill="#c0504d" fillOpacity={0.9} stroke="#8f3a37" />
        <rect x={lx - 15} y={cy + 8} width={30} height={4} rx={1} fill="#a9433f" />
        <ellipse cx={lx - 5} cy={cy - 10} rx={3} ry={6} fill="#ffffff" fillOpacity={0.35} />
        {headers}
      </g>
    );
  }

  if (kind === "button") {
    const size = Math.min(bodyW, h - 20, 52);
    const bx = bodyX + (bodyW - size) / 2;
    const by = cy - size / 2;
    return (
      <g>
        {carrier}
        {pins.map((pin) => <path key={pin.label} d={`M ${wireX} ${pin.y} H ${bx}`} fill="none" stroke="#aeb6bf" strokeWidth={1.6} />)}
        <rect x={bx} y={by} width={size} height={size} rx={4} fill="#2a2f35" stroke="#4a525b" />
        {[[bx + 5, by + 5], [bx + size - 5, by + 5], [bx + 5, by + size - 5], [bx + size - 5, by + size - 5]].map(([dx, dy]) => <circle key={`${dx}-${dy}`} cx={dx} cy={dy} r={1.8} fill="#6d757e" />)}
        <circle cx={bx + size / 2} cy={cy} r={size * 0.3} fill="#3d444c" stroke="#59626c" />
        <circle cx={bx + size / 2} cy={cy} r={size * 0.3 - 3} fill="#454d56" />
        {headers}
      </g>
    );
  }

  // Generic part: a DIP package.
  const legs = Math.max(3, Math.min(7, Math.floor(bodyW / 16)));
  return (
    <g>
      {carrier}
      {pins.map((pin) => <path key={pin.label} d={`M ${wireX} ${pin.y} H ${bodyX}`} fill="none" stroke="#aeb6bf" strokeWidth={1.2} strokeOpacity={0.7} />)}
      {Array.from({ length: legs }, (_, index) => {
        const lx = bodyX + 10 + index * ((bodyW - 20) / (legs - 1));
        return <g key={index} fill="#aeb6bf"><rect x={lx - 2} y={cy - 26} width={4} height={6} /><rect x={lx - 2} y={cy + 20} width={4} height={6} /></g>;
      })}
      <rect x={bodyX} y={cy - 21} width={bodyW} height={42} rx={3} fill="#22272d" stroke="#3a424b" />
      <path d={`M ${bodyX} ${cy - 5} a 5 5 0 0 1 0 10`} fill="#161a1f" />
      <text x={bodyX + bodyW / 2} y={cy + 3} textAnchor="middle" fill={SILK} fillOpacity={0.7} className="font-mono text-[8px] font-bold">IC</text>
      {headers}
    </g>
  );
}

/** Shared SVG defs (the transducer mesh); render once inside each <svg> that draws parts. */
export function PartDefs() {
  return (
    <pattern id="mesh" width="4" height="4" patternUnits="userSpaceOnUse">
      <circle cx="2" cy="2" r="0.7" fill="#6b747d" />
    </pattern>
  );
}

/** Small standalone drawing for lists. */
export function PartThumb({ kind, className }: { kind: PartKind; className?: string }) {
  return (
    <svg viewBox="0 0 150 80" className={className} aria-hidden>
      <defs><PartDefs /></defs>
      <PartArt kind={kind} x={-44} y={4} w={190} h={72} pins={[]} />
    </svg>
  );
}
