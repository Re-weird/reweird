"use client";

import { useEffect } from "react";
import { useParams } from "next/navigation";
import { useAppState } from "@/lib/app-state";

// Loads the project named in the URL. The breadcrumb and tab row live in the
// global header (global-nav.tsx), GitHub-style, so this layout adds no chrome.
export default function ProjectLayout({ children }: { children: React.ReactNode }) {
  const { id } = useParams<{ id: string }>();
  const { loadProject } = useAppState();

  useEffect(() => {
    loadProject(id);
    // Only re-run when the URL's project id changes.
    // eslint-disable-next-line react-hooks/exhaustive-deps
  }, [id]);

  return <>{children}</>;
}
