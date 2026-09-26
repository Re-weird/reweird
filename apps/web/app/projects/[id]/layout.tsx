"use client";

import { useEffect, useState } from "react";
import Link from "next/link";
import { useParams } from "next/navigation";
import { ArrowLeft, FolderX } from "lucide-react";
import { Button } from "@/components/ui/button";
import { Skeleton } from "@/components/ui/skeleton";
import { useAppState } from "@/lib/app-state";
import { DEMO_PROJECT_ID } from "@/lib/project-routes";

// Loads the project named in the URL and holds the page until both the API
// session and that project are in state. Without this, the local demo
// fixture (fake probe readings) rendered for a moment before real data
// replaced it.
export default function ProjectLayout({ children }: { children: React.ReactNode }) {
  const { id } = useParams<{ id: string }>();
  const { loadProject, project, sessionReady } = useAppState();
  const [status, setStatus] = useState<"loading" | "ready" | "missing">("loading");

  useEffect(() => {
    let live = true;
    setStatus("loading");
    loadProject(id).then((result) => { if (live) setStatus(result); });
    return () => { live = false; };
    // Only re-run when the URL's project id changes.
    // eslint-disable-next-line react-hooks/exhaustive-deps
  }, [id]);

  if (status === "missing") {
    return (
      <div data-tw className="mx-auto flex max-w-md flex-col items-start gap-4 py-24">
        <span className="grid size-11 place-items-center rounded-lg bg-surface-2 text-muted-foreground ring-1 ring-border"><FolderX className="size-5" strokeWidth={1.5} /></span>
        <div>
          <h1 className="text-lg font-semibold text-foreground">Project not found</h1>
          <p className="mt-1 text-sm leading-relaxed text-muted-foreground">No project with the id <span className="font-mono text-foreground">{id}</span> exists on this instance.</p>
        </div>
        <Button asChild variant="outline" size="sm"><Link href="/projects"><ArrowLeft /> Back to projects</Link></Button>
      </div>
    );
  }

  const projectInState = id === DEMO_PROJECT_ID ? project === null : project?.id === id;
  if (status === "loading" || !sessionReady || !projectInState) {
    return (
      <div data-tw aria-busy="true" aria-label="Loading project" className="flex flex-col gap-6">
        <Skeleton className="h-44 w-full bg-surface-2" />
        <div className="grid grid-cols-2 gap-4 md:grid-cols-4">
          {[0, 1, 2, 3].map((cell) => <Skeleton key={cell} className="h-20 bg-surface-2" />)}
        </div>
        <div className="grid grid-cols-1 gap-6 lg:grid-cols-[minmax(0,1.6fr)_minmax(0,1fr)]">
          <Skeleton className="h-72 bg-surface-2" />
          <Skeleton className="h-72 bg-surface-2" />
        </div>
      </div>
    );
  }

  return <>{children}</>;
}
