"use client";

import Link from "next/link";
import { useParams, usePathname } from "next/navigation";
import { AnimatePresence, LayoutGroup, motion, useReducedMotion } from "framer-motion";
import { BarChart3, Box, Cable, CheckCircle2, Cpu, Fingerprint, FileBarChart, FolderGit2, GitCommitHorizontal, LayoutDashboard, Microscope, RefreshCw, Settings, TestTube2 } from "lucide-react";
import { useAppState } from "@/lib/app-state";
import { projectTabs, type ProjectTabID } from "@/lib/project-routes";
import { cn } from "@/lib/utils";

type NavItem = { key: string; label: string; href: string; icon: typeof Box; active: boolean; dot?: boolean };

const spring = { type: "spring", stiffness: 380, damping: 32 } as const;

const tabIcons: Record<ProjectTabID, typeof Box> = {
  workbench: LayoutDashboard,
  overview: Box,
  "probe-setup": Cable,
  passport: Fingerprint,
  simulator: TestTube2,
  diagnosis: Microscope,
  "physical-history": GitCommitHorizontal,
  "next-test": TestTube2,
  verify: CheckCircle2,
  history: RefreshCw,
  reports: FileBarChart,
  computer: Cpu,
};

// The tab row sits beside the profile rail, above the page content. Outside a
// project it lists Dashboard / Projects / Settings; inside one it morphs into
// the project's tabs in the same spot. The underline is a shared layoutId.
export function SectionNav() {
  const pathname = usePathname() ?? "/";
  const params = useParams<{ id?: string }>();
  const reduce = useReducedMotion();
  const { session } = useAppState();

  const projectID = params?.id;
  const base = projectID ? `/projects/${projectID}` : "";
  const items: NavItem[] = projectID
    ? projectTabs.map((tab) => {
      const href = tab.segment ? `${base}/${tab.segment}` : base;
      return { key: tab.id, label: tab.label, href, icon: tabIcons[tab.id], active: pathname === href, dot: tab.id === "diagnosis" && session.stage !== "verify" };
    })
    : [
      { key: "dashboard", label: "Dashboard", href: "/", icon: BarChart3, active: pathname === "/" },
      { key: "projects", label: "Projects", href: "/projects", icon: FolderGit2, active: pathname.startsWith("/projects") },
      { key: "settings", label: "Settings", href: "/settings", icon: Settings, active: pathname === "/settings" },
    ];

  return (
    <LayoutGroup id="section-nav">
      <nav
        data-tw
        aria-label={projectID ? "Project navigation" : "Primary navigation"}
        className="sticky top-14 z-30 h-12 overflow-x-auto overflow-y-hidden border-b border-border bg-background/90 backdrop-blur supports-[backdrop-filter]:bg-background/75 [scrollbar-width:none]"
      >
        <AnimatePresence mode="wait" initial={false}>
          <motion.div
            key={projectID ? `project:${projectID}` : "global"}
            className="flex h-full items-stretch gap-0.5"
            initial="hidden"
            animate="show"
            exit="exit"
            variants={{
              hidden: {},
              show: { transition: { staggerChildren: reduce ? 0 : 0.025 } },
              exit: { opacity: 0, transition: { duration: 0.12 } },
            }}
          >
            {items.map((item) => {
              const Icon = item.icon;
              return (
                <motion.div key={item.key} className="relative flex" variants={{ hidden: { opacity: 0, y: reduce ? 0 : 6 }, show: { opacity: 1, y: 0, transition: spring } }}>
                  <Link
                    href={item.href}
                    aria-current={item.active ? "page" : undefined}
                    className={cn(
                      "group my-2 flex items-center gap-2 rounded-md px-2.5 text-[13px] whitespace-nowrap transition-colors",
                      item.active ? "font-semibold text-foreground" : "font-medium text-muted-foreground hover:bg-accent hover:text-foreground",
                    )}
                  >
                    <Icon className={cn("size-4", item.active ? "text-foreground" : "text-subtle group-hover:text-muted-foreground")} strokeWidth={1.5} />
                    {item.label}
                    {item.dot && <span className="size-1.5 rounded-full bg-signal" aria-label="Awaiting review" />}
                  </Link>
                  {item.active && <motion.span layoutId="nav-active" transition={spring} className="absolute inset-x-2 -bottom-px h-0.5 rounded-full bg-signal" />}
                </motion.div>
              );
            })}
          </motion.div>
        </AnimatePresence>
      </nav>
    </LayoutGroup>
  );
}
