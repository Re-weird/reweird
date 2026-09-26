"use client";

import { useEffect, useRef, useState } from "react";
import gsap from "gsap";
import { ScrollTrigger } from "gsap/ScrollTrigger";

let registered = false;

/**
 * Drives the whole scroll story from a single progress value (0..1) computed
 * from native scroll position over the story container. GSAP ScrollTrigger
 * only reads scroll position here (scrub) — it never takes control of the
 * scrollbar, so normal browser scrolling stays normal.
 */
export function useStoryProgress(containerRef: React.RefObject<HTMLElement | null>) {
  const progressRef = useRef(0);
  const [progress, setProgress] = useState(0);

  useEffect(() => {
    const container = containerRef.current;
    if (!container) return;
    if (!registered) {
      gsap.registerPlugin(ScrollTrigger);
      registered = true;
    }

    const trigger = ScrollTrigger.create({
      trigger: container,
      start: "top top",
      end: "bottom bottom",
      scrub: true,
      onUpdate: (self) => {
        progressRef.current = self.progress;
        setProgress(self.progress);
      },
    });

    return () => trigger.kill();
  }, [containerRef]);

  return { progress, progressRef };
}
