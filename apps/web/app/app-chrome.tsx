"use client";

import { CheckCircle2 } from "lucide-react";
import { useAppState } from "@/lib/app-state";
import { NewProjectModal } from "./project-workflow";
import { ProfileRail } from "./profile-rail";
import { SectionNav } from "./section-nav";

// Persistent shell: profile rail fixed on the left; the tab row and the
// current page render beside it. The rail lives in the root layout, so it
// never remounts on navigation. No data-tw on these wrappers - legacy
// (non-Tailwind) pages render in the content column and the scoped Tailwind
// reset must not reach them.
export function AppChrome({ children }: { children: React.ReactNode }) {
  const { showNewProject, setShowNewProject, completeProjectAnalysis, loadDemoProject, toast } = useAppState();
  return (
    <main>
      <div className="content">
        <div className="grid grid-cols-1 gap-10 lg:grid-cols-[232px_minmax(0,1fr)] lg:gap-12">
          <ProfileRail />
          <div className="min-w-0">
            <SectionNav />
            <div className="pt-8">{children}</div>
          </div>
        </div>
      </div>
      {showNewProject && <NewProjectModal onClose={() => setShowNewProject(false)} onComplete={completeProjectAnalysis} onLoadDemo={loadDemoProject} />}
      {toast && <div className="toast" role="status"><CheckCircle2 size={18} />{toast}</div>}
    </main>
  );
}
