"use client";

import { useMemo, useRef } from "react";
import { useFrame } from "@react-three/fiber";
import { RoundedBox, Html, Line, Instances, Instance } from "@react-three/drei";
import * as THREE from "three";
import { poseAt, activeBeatIndex, type BeatPose } from "./beats";

const PCB_GREEN = "#2f5d46";
const PCB_GREEN_LIGHT = "#3a7057";
const CHIP_DARK = "#20242a";
const PIN_METAL = "#c9b98a";

function PinHeader({ side, count }: { side: -1 | 1; count: number }) {
  const positions = useMemo(() => {
    const out: [number, number, number][] = [];
    const span = 1.5;
    for (let i = 0; i < count; i += 1) {
      const z = -span / 2 + (span * i) / (count - 1);
      out.push([side * 0.62, 0.06, z]);
    }
    return out;
  }, [side, count]);

  return (
    <Instances limit={count}>
      <cylinderGeometry args={[0.014, 0.014, 0.09, 8]} />
      <meshStandardMaterial color={PIN_METAL} metalness={0.6} roughness={0.35} />
      {positions.map((position, index) => (
        <Instance key={index} position={position} />
      ))}
    </Instances>
  );
}

function Esp32Board({ pinCount }: { pinCount: number }) {
  return (
    <group>
      <RoundedBox args={[1.3, 0.06, 1.8]} radius={0.03} smoothness={2} castShadow receiveShadow>
        <meshStandardMaterial color={PCB_GREEN} roughness={0.55} metalness={0.12} />
      </RoundedBox>
      {/* RF shield / module can */}
      <RoundedBox args={[0.55, 0.09, 0.75]} radius={0.02} position={[0.1, 0.07, -0.35]} castShadow>
        <meshStandardMaterial color={CHIP_DARK} roughness={0.4} metalness={0.5} />
      </RoundedBox>
      {/* USB connector */}
      <RoundedBox args={[0.28, 0.1, 0.16]} radius={0.02} position={[0, 0.06, 0.97]} castShadow>
        <meshStandardMaterial color="#8b8f94" roughness={0.4} metalness={0.6} />
      </RoundedBox>
      <PinHeader side={-1} count={pinCount} />
      <PinHeader side={1} count={pinCount} />
    </group>
  );
}

function Hcsr04({ visibility }: { visibility: number }) {
  if (visibility <= 0.01) return null;
  return (
    <group position={[1.35, 0.05, 0.2]} scale={Math.max(0.001, visibility)}>
      <RoundedBox args={[0.7, 0.32, 0.28]} radius={0.03} castShadow>
        <meshStandardMaterial color="#e7e7e2" roughness={0.6} />
      </RoundedBox>
      <mesh position={[-0.16, 0.05, 0.16]}>
        <cylinderGeometry args={[0.13, 0.13, 0.12, 20]} />
        <meshStandardMaterial color="#c7c7bf" metalness={0.5} roughness={0.3} />
      </mesh>
      <mesh position={[0.16, 0.05, 0.16]}>
        <cylinderGeometry args={[0.13, 0.13, 0.12, 20]} />
        <meshStandardMaterial color="#c7c7bf" metalness={0.5} roughness={0.3} />
      </mesh>
      {visibility > 0.6 && (
        <Html center position={[0, 0.35, 0.2]} className="story-label" occlude={false}>
          HC-SR04
        </Html>
      )}
    </group>
  );
}

function ProbeMarker({ position, label, color, show }: { position: [number, number, number]; label: string; color: string; show: number }) {
  if (show <= 0.05) return null;
  return (
    <group position={position} scale={Math.max(0.001, show)}>
      <mesh>
        <sphereGeometry args={[0.05, 16, 16]} />
        <meshStandardMaterial color={color} emissive={color} emissiveIntensity={0.4} />
      </mesh>
      {show > 0.6 && (
        <Html center className="story-label story-label-sm" occlude={false}>
          {label}
        </Html>
      )}
    </group>
  );
}

function SignalLine({ start, end, fault, verified }: { start: [number, number, number]; end: [number, number, number]; fault: number; verified: number }) {
  const color = fault > 0.5 ? "#c96a63" : verified > 0.5 ? "#5f9c7e" : "#8fa6c9";
  const points = useMemo(() => [new THREE.Vector3(...start), new THREE.Vector3(...end)], [start, end]);
  return <Line points={points} color={color} lineWidth={1.4} dashed={fault > 0.5} dashSize={0.05} gapSize={0.04} transparent opacity={0.85} />;
}

function OrbitCard({ label, baseAngle, orbit }: { label: string; baseAngle: number; orbit: number }) {
  const radius = 2.1;
  const restPosition: [number, number, number] = [0.9, 0.4, -0.2];
  const orbitPosition: [number, number, number] = [Math.cos(baseAngle) * radius, 0.3, Math.sin(baseAngle) * radius];
  const x = restPosition[0] + (orbitPosition[0] - restPosition[0]) * orbit;
  const y = restPosition[1] + (orbitPosition[1] - restPosition[1]) * orbit;
  const z = restPosition[2] + (orbitPosition[2] - restPosition[2]) * orbit;
  if (orbit <= 0.05) return null;
  return (
    <Html center position={[x, y, z]} className="story-label story-orbit-card" style={{ opacity: Math.min(1, orbit * 1.6) }} occlude={false}>
      {label}
    </Html>
  );
}

function EcosystemBoard({ position, visibility }: { position: [number, number, number]; visibility: number }) {
  if (visibility <= 0.03) return null;
  return (
    <RoundedBox args={[0.55, 0.03, 0.75]} radius={0.02} position={position} scale={Math.max(0.001, visibility) * 0.7}>
      <meshStandardMaterial color="#4a5b52" roughness={0.8} transparent opacity={0.35} />
    </RoundedBox>
  );
}

export function Board({ progressRef, reducedMotion, isMobile = false }: { progressRef: React.MutableRefObject<number>; reducedMotion: boolean; isMobile?: boolean }) {
  const group = useRef<THREE.Group>(null);
  const pose = useRef<BeatPose>(poseAt(0));
  const posXScale = isMobile ? 0.45 : 1;

  useFrame(() => {
    if (!group.current) return;
    const raw = progressRef.current;
    const nextPose = reducedMotion
      ? { ...poseAt(activeBeatIndex(raw) / 10), index: 0, t: 0 }
      : poseAt(raw);
    pose.current = nextPose;
    const damp = reducedMotion ? 1 : 0.12;
    group.current.rotation.y += (nextPose.rotY - group.current.rotation.y) * damp;
    group.current.rotation.x += (nextPose.rotX - group.current.rotation.x) * damp;
    group.current.position.x += (nextPose.posX * posXScale - group.current.position.x) * damp;
    group.current.position.y += (nextPose.posY - group.current.position.y) * damp;
    const targetScale = nextPose.scale;
    group.current.scale.x += (targetScale - group.current.scale.x) * damp;
    group.current.scale.y += (targetScale - group.current.scale.y) * damp;
    group.current.scale.z += (targetScale - group.current.scale.z) * damp;
  });

  const p = pose.current;

  return (
    <group ref={group}>
      <Esp32Board pinCount={isMobile ? 7 : 13} />
      <Hcsr04 visibility={p.sensor} />
      <ProbeMarker position={[-0.62, 0.15, -0.45]} label="P1 · POWER" color="#7fa3d6" show={p.probes} />
      <ProbeMarker position={[0.62, 0.15, 0.1]} label="P2 · TRIG" color="#8fbf9e" show={p.probes} />
      <ProbeMarker position={[1.05, 0.2, 0.4]} label="P3 · ECHO" color={p.fault > 0.5 ? "#c96a63" : "#8fbf9e"} show={p.probes} />
      {p.probes > 0.4 && (
        <SignalLine start={[0.62, 0.15, 0.1]} end={[1.2, 0.15, 0.25]} fault={p.fault} verified={p.verified} />
      )}
      <OrbitCard label="CODE" baseAngle={Math.PI * 0.1} orbit={p.orbit} />
      <OrbitCard label="SPECIFICATIONS" baseAngle={Math.PI * 0.55} orbit={p.orbit} />
      <OrbitCard label="MEASUREMENTS" baseAngle={Math.PI * 1.0} orbit={p.orbit} />
      <OrbitCard label="BASELINE" baseAngle={Math.PI * 1.5} orbit={p.orbit} />
      <EcosystemBoard position={[-2.1, -0.3, -2.4]} visibility={p.ecosystem} />
      <EcosystemBoard position={[2.2, -0.35, -2.7]} visibility={p.ecosystem} />
    </group>
  );
}
