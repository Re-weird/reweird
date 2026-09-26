"use client";

import { Suspense, useEffect, useMemo, useRef, useState } from "react";
import { Canvas, useFrame } from "@react-three/fiber";
import { Box3, Mesh, MeshStandardMaterial, SRGBColorSpace, type Group, Vector3 } from "three";
import { useReducedMotion } from "./scroll-story/useReducedMotion";
import { OrbitControls, useGLTF, useTexture } from "@react-three/drei";
import { isWebGLAvailable } from "./scroll-story/webgl";
import { StaticIllustration } from "./scroll-story/StaticIllustration";

function Assembly({ selected, rotation }: { selected: number | null; rotation: number }) {
  const group = useRef<Group>(null);
  const reduced = useReducedMotion();
  useFrame((state, delta) => {
    if (!group.current) return;
    const target = (selected ?? 0) * Math.PI / 2 + rotation;
    group.current.rotation.y += (target - group.current.rotation.y) * (reduced ? 1 : 1 - Math.exp(-delta * 5));
    group.current.position.y = .3 + (reduced || selected !== null ? 0 : Math.sin(state.clock.elapsedTime * .6) * .035);
  });
  return <group ref={group}><RodinEsp32 /></group>;
}

export function RodinEsp32() {
  const { scene } = useGLTF("/models/reweird-esp32.glb");
  const [diffuse, normal, pbr] = useTexture([
    "/models/reweird-esp32-diffuse.jpg",
    "/models/reweird-esp32-normal.jpg",
    "/models/reweird-esp32-pbr.jpg",
  ]);
  const model = useMemo(() => {
    const clone = scene.clone(true);
    diffuse.colorSpace = SRGBColorSpace;
    diffuse.flipY = normal.flipY = pbr.flipY = false;
    const bounds = new Box3().setFromObject(clone);
    const size = bounds.getSize(new Vector3());
    const center = bounds.getCenter(new Vector3());
    const scale = 2.25 / Math.max(size.x, size.y, size.z);
    clone.position.set(-center.x * scale, -center.y * scale, -center.z * scale);
    clone.scale.setScalar(scale);
    clone.traverse(object => {
      if (object instanceof Mesh) {
        object.castShadow = true;
        object.receiveShadow = true;
        object.material = new MeshStandardMaterial({
          map: diffuse,
          normalMap: normal,
          roughnessMap: pbr,
          metalnessMap: pbr,
          roughness: .82,
          metalness: .55,
        });
      }
    });
    return clone;
  }, [diffuse, normal, pbr, scene]);
  return <primitive object={model} rotation={[0, 0, 0]} />;
}

useGLTF.preload("/models/reweird-esp32.glb");

export function InteractiveHardware({ selected, theme }: { selected: number | null; theme: "light" | "dark" }) {
  const [available, setAvailable] = useState(false);
  const [rotation, setRotation] = useState(0);
  const [visible, setVisible] = useState(true);
  const wrapper = useRef<HTMLDivElement>(null);
  const reduced = useReducedMotion();
  useEffect(() => setAvailable(isWebGLAvailable()), []);
  useEffect(() => {
    const observer = new IntersectionObserver(([entry]) => setVisible(entry.isIntersecting));
    if (wrapper.current) observer.observe(wrapper.current);
    return () => observer.disconnect();
  }, [available]);
  if (!available) return <StaticIllustration />;
  return <div ref={wrapper} style={{width: "100%", height: "100%"}} tabIndex={0} role="group" aria-label="3D ESP32. Drag to rotate, or use left and right arrow keys." onKeyDown={event => { if (event.key === "ArrowLeft" || event.key === "ArrowRight") { event.preventDefault(); setRotation(value => value + (event.key === "ArrowLeft" ? -.4 : .4)); } }}><Canvas dpr={[1, 1.5]} frameloop={visible ? reduced ? "demand" : "always" : "never"} camera={{ position: [2.5, 3.8, 3.8], fov: 36 }} gl={{ alpha: true, antialias: true }}>
    <ambientLight intensity={1.6} />
    <directionalLight position={[3, 6, 4]} intensity={3} />
    <directionalLight position={[-4, 2, -3]} intensity={1.2} color="#b5c9e6" />
    <Suspense fallback={null}><Assembly selected={selected} rotation={rotation} /></Suspense>
    <mesh position={[0, -.38, 0]}><cylinderGeometry args={[1.3, 1.33, .12, 64]} /><meshStandardMaterial color={theme === "light" ? "#cdd2d6" : "#20252b"} metalness={.35} roughness={.6} /></mesh>
    <OrbitControls makeDefault enablePan={false} enableZoom={false} minPolarAngle={.25} maxPolarAngle={Math.PI / 2.1} />
  </Canvas></div>;
}
