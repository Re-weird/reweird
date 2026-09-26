"use client";

import { useEffect, useRef, useState } from "react";
import { Canvas } from "@react-three/fiber";
import { Board } from "./Board";
import { isWebGLAvailable } from "./webgl";
import { StaticIllustration } from "./StaticIllustration";
import styles from "./scroll-story.module.css";

export function Scene({ progressRef, reducedMotion }: { progressRef: React.MutableRefObject<number>; reducedMotion: boolean }) {
  const wrapperRef = useRef<HTMLDivElement>(null);
  const [webglOk, setWebglOk] = useState<boolean | null>(null);
  const [visible, setVisible] = useState(true);
  const [isMobile, setIsMobile] = useState(false);

  useEffect(() => {
    setWebglOk(isWebGLAvailable());
    const query = window.matchMedia("(max-width: 760px)");
    setIsMobile(query.matches);
    const handler = (event: MediaQueryListEvent) => setIsMobile(event.matches);
    query.addEventListener("change", handler);
    return () => query.removeEventListener("change", handler);
  }, []);

  useEffect(() => {
    const node = wrapperRef.current;
    if (!node) return;
    const observer = new IntersectionObserver(([entry]) => setVisible(entry.isIntersecting), { threshold: 0.01 });
    observer.observe(node);
    return () => observer.disconnect();
  }, []);

  return (
    <div ref={wrapperRef} className={styles.sceneWrapper}>
      {webglOk === false ? (
        <StaticIllustration />
      ) : webglOk === null ? null : (
        <Canvas
          dpr={isMobile ? 1 : [1, 1.5]}
          frameloop={visible ? "always" : "never"}
          camera={{ position: [0, 0.6, isMobile ? 5.6 : 4.4], fov: 34 }}
          gl={{ antialias: !isMobile, alpha: true }}
        >
          <ambientLight intensity={0.55} />
          <directionalLight position={[3, 4, 2]} intensity={1.1} castShadow={false} />
          <directionalLight position={[-3, -1, -2]} intensity={0.25} color="#8fb0e6" />
          <Board progressRef={progressRef} reducedMotion={reducedMotion} isMobile={isMobile} />
        </Canvas>
      )}
    </div>
  );
}
