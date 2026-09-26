"use client";

import { Component, Suspense, useEffect, useRef, useState, type ReactNode, type MutableRefObject } from "react";
import { Canvas, useFrame } from "@react-three/fiber";
import { Environment, Lightformer } from "@react-three/drei";
import { Group, MathUtils, Vector3 } from "three";
import { RodinEsp32 } from "./InteractiveHardware";
import { isWebGLAvailable } from "./scroll-story/webgl";

class SceneBoundary extends Component<{ children: ReactNode; fallback: ReactNode }, { failed: boolean }> {
  state = { failed: false };
  static getDerivedStateFromError() { return { failed: true }; }
  render() { return this.state.failed ? this.props.fallback : this.props.children; }
}

function Model({ progress, reduced, turn, mobile }: { progress: MutableRefObject<number>; reduced: boolean; turn: number; mobile: boolean }) {
  const group = useRef<Group>(null);
  const look = useRef(new Vector3());
  useFrame(({ camera, clock }, delta) => {
    if (!group.current) return;
    const p = reduced ? 0 : progress.current;
    const dive = MathUtils.smoothstep(p, .46, .96);
    const ease = 1 - Math.exp(-Math.min(delta, .05) * 7);
    const idle = reduced ? 0 : Math.sin(clock.elapsedTime * .24) * .1;
    group.current.rotation.y = MathUtils.lerp(group.current.rotation.y, (1 - dive) * (-.32 + idle + turn + p * .9), ease);
    group.current.rotation.z = MathUtils.lerp(group.current.rotation.z, (1 - dive) * -.18, ease);
    group.current.position.y = reduced ? 0 : Math.sin(clock.elapsedTime * .7) * .025 * (1 - dive);
    // Move from an elevated product view toward the USB edge, then through it.
    camera.position.lerp(new Vector3(
      MathUtils.lerp(mobile ? 2.7 : 2.4, 0, dive),
      MathUtils.lerp(2.5, .035, dive),
      MathUtils.lerp(mobile ? 4.6 : 3.9, .72, dive),
    ), ease);
    look.current.lerp(new Vector3(0, 0, MathUtils.lerp(0, .4, dive)), ease);
    camera.lookAt(look.current);
  });
  return <group ref={group}><RodinEsp32 /></group>;
}

export function StoryHardware({ progress, reduced, light }: { progress: MutableRefObject<number>; reduced: boolean; light: boolean }) {
  const [webgl, setWebgl] = useState(false);
  const [visible, setVisible] = useState(true);
  const [mobile, setMobile] = useState(false);
  const [turn, setTurn] = useState(0);
  const wrapper = useRef<HTMLDivElement>(null);
  const drag = useRef<number | null>(null);
  useEffect(() => {
    setWebgl(isWebGLAvailable());
    const query = window.matchMedia("(max-width: 760px)");
    const resize = () => setMobile(query.matches);
    resize(); query.addEventListener("change", resize);
    const observer = new IntersectionObserver(([entry]) => setVisible(entry.isIntersecting));
    if (wrapper.current) observer.observe(wrapper.current);
    return () => { observer.disconnect(); query.removeEventListener("change", resize); };
  }, []);
  const fallback = <div style={{ display: "grid", placeItems: "center", height: "100%", color: "var(--s-muted)", fontSize: 13 }}>ESP32 · Explore the story below</div>;
  return <div ref={wrapper} role="group" tabIndex={0} aria-label="Interactive ESP32. Drag horizontally or use left and right arrows to rotate." style={{ height: "100%", width: "100%", touchAction: "pan-y", cursor: "grab" }}
    onPointerDown={event => { drag.current = event.clientX; event.currentTarget.setPointerCapture(event.pointerId); }}
    onPointerMove={event => { if (drag.current !== null) { setTurn(value => value + (event.clientX - drag.current!) * .009); drag.current = event.clientX; } }}
    onPointerUp={() => { drag.current = null; }} onPointerCancel={() => { drag.current = null; }}
    onKeyDown={event => { if (["ArrowLeft", "ArrowRight"].includes(event.key)) { event.preventDefault(); setTurn(value => value + (event.key === "ArrowLeft" ? -.3 : .3)); } }}>
    {webgl ? <SceneBoundary fallback={fallback}><Canvas dpr={mobile ? 1 : [1, 1.5]} frameloop={visible ? reduced ? "demand" : "always" : "never"} camera={{ position: [2.4, 2.5, 3.9], fov: 32, near: .015 }} gl={{ antialias: !mobile, alpha: true }}>
      <ambientLight intensity={light ? .85 : .5} />
      <directionalLight position={[3, 5, 3]} intensity={light ? 2.1 : 2.6} />
      <directionalLight position={[-3, 1, -2]} intensity={1.5} color="#a0b8d1" />
      <Environment resolution={128} frames={1}>
        <Lightformer intensity={3} position={[0, 4, 0]} rotation={[Math.PI / 2, 0, 0]} scale={[5, 2, 1]} />
        <Lightformer intensity={2} position={[-4, 1, 0]} rotation={[0, Math.PI / 2, 0]} scale={[2, 5, 1]} color="#b5c6d5" />
      </Environment>
      <Suspense fallback={null}><Model progress={progress} reduced={reduced} turn={turn} mobile={mobile} /></Suspense>
    </Canvas></SceneBoundary> : fallback}
  </div>;
}
