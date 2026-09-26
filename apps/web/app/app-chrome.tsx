"use client";

import { CheckCircle2 } from "lucide-react";
import { useAppState } from "@/lib/app-state";
import { NewProjectModal } from "./project-workflow";

export function AppChrome({ children }: { children: React.ReactNode }) {
  const { showNewProject, setShowNewProject, completeProjectAnalysis, loadDemoProject, toast } = useAppState();
  return (
    <main>
      <div className="content">{children}</div>
      {showNewProject && <NewProjectModal onClose={() => setShowNewProject(false)} onComplete={completeProjectAnalysis} onLoadDemo={loadDemoProject} />}
      {toast && <div className="toast" role="status"><CheckCircle2 size={18} />{toast}</div>}
    </main>
  );
}
