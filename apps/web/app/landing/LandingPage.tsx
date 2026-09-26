"use client";

import dynamic from "next/dynamic";

const ScrollStory = dynamic(() => import("./scroll-story/ScrollStory").then((mod) => mod.ScrollStory), {
  ssr: false,
});

export function LandingPage() {
  return <ScrollStory />;
}
