"use client";

import { useEffect, useRef, useState } from "react";
import Link from "next/link";
import { useParams, usePathname } from "next/navigation";
import { AnimatePresence, LayoutGroup, motion, useReducedMotion } from "framer-motion";
import {
  BarChart3, Box, Cable, CheckCircle2, ChevronDown, Cpu, FileBarChart, FolderGit2, LayoutDashboard,
  Microscope, Moon, Plus, RefreshCw, Settings, Sun, TestTube2, Waves,
} from "lucide-react";
import { Button } from "@/components/ui/button";
import { Tooltip, TooltipContent, TooltipTrigger } from "@/components/ui/tooltip";
import { useAppState } from "@/lib/app-state";
import { DEMO_PROJECT_ID, projectTabs, type ProjectTabID } from "@/lib/project-routes";
import { cn } from "@/lib/utils";

type Theme = "light" | "dark";
type NavItem = { key: string; label: string; href: string; icon: typeof Box; active: boolean; dot?: boolean };

const spring = { type: "spring", stiffness: 380, damping: 32 } as const;

const tabIcons: Record<ProjectTabID, typeof Box> = {
  workbench: LayoutDashboard,
  overview: Box,
  "probe-setup": Cable,
  simulator: TestTube2,
  diagnosis: Microscope,
  "next-test": TestTube2,
  verify: CheckCircle2,
  history: RefreshCw,
  reports: FileBarChart,
  computer: Cpu,
};

function useTheme() {
  const [theme, setTheme] = useState<Theme>("dark");
  useEffect(() => {
    let stored: string | null = null;
    try { stored = window.localStorage.getItem("reweird-theme"); } catch { /* storage unavailable */ }
    const next: Theme = stored === "light" || stored === "dark"
      ? stored
      : window.matchMedia("(prefers-color-scheme: light)").matches ? "light" : "dark";
    setTheme(next);
    document.documentElement.dataset.theme = next;
  }, []);
  const toggle = () => {
    const next: Theme = theme === "dark" ? "light" : "dark";
    setTheme(next);
    document.documentElement.dataset.theme = next;
    try { window.localStorage.setItem("reweird-theme", next); } catch { /* keep in-session */ }
  };
  return { theme, toggle };
}

function AccountMenu({ onLoadDemo }: { onLoadDemo: () => void }) {
  const [open, setOpen] = useState(false);
  const ref = useRef<HTMLDivElement>(null);
  const pathname = usePathname();

  useEffect(() => { setOpen(false); }, [pathname]);
  useEffect(() => {
    if (!open) return;
    const close = (event: MouseEvent) => { if (!ref.current?.contains(event.target as Node)) setOpen(false); };
    const escape = (event: KeyboardEvent) => { if (event.key === "Escape") setOpen(false); };
    document.addEventListener("mousedown", close);
    document.addEventListener("keydown", escape);
    return () => { document.removeEventListener("mousedown", close); document.removeEventListener("keydown", escape); };
  }, [open]);

  const itemClass = "flex items-center gap-2.5 rounded-md px-2.5 py-2 text-[13px] text-foreground transition-colors hover:bg-accent [&_svg]:size-4 [&_svg]:text-muted-foreground";

  return (
    <div ref={ref} className="relative">
      <button
        onClick={() => setOpen((value) => !value)}
        aria-haspopup="menu"
        aria-expanded={open}
        aria-label="Account menu"
        className="grid size-8 place-items-center rounded-full border border-border bg-surface-3 font-mono text-[11px] font-medium text-foreground transition-transform active:scale-95"
      >
        RW
      </button>
      <AnimatePresence>
        {open && (
          <motion.div
            role="menu"
            initial={{ opacity: 0, y: -6, scale: 0.97 }}
            animate={{ opacity: 1, y: 0, scale: 1 }}
            exit={{ opacity: 0, y: -4, scale: 0.98 }}
            transition={spring}
            className="absolute top-[calc(100%+8px)] right-0 z-50 flex w-60 origin-top-right flex-col rounded-lg border border-border bg-popover p-1.5 shadow-[0_18px_40px_-16px_rgba(0,0,0,0.6)]"
          >
            <div className="mb-1 border-b border-line-soft px-2.5 pt-1.5 pb-2.5">
              <p className="text-[13px] font-semibold text-foreground">Local session</p>
              <p className="text-xs text-muted-foreground">No user sign-in yet</p>
            </div>
            <Link href="/" role="menuitem" className={itemClass}><BarChart3 /> Your dashboard</Link>
            <Link href="/projects" role="menuitem" className={itemClass}><FolderGit2 /> Your projects</Link>
            <Link href="/settings" role="menuitem" className={itemClass}><Settings /> Settings</Link>
            <div className="my-1 h-px bg-line-soft" />
            <button role="menuitem" onClick={() => { setOpen(false); onLoadDemo(); }} className={cn(itemClass, "text-left")}><Waves /> Load demo project</button>
          </motion.div>
        )}
      </AnimatePresence>
    </div>
  );
}

export function GlobalNav() {
  const pathname = usePathname() ?? "/";
  const params = useParams<{ id?: string }>();
  const reduce = useReducedMotion();
  const { project, session, setShowNewProject, loadDemoProject } = useAppState();
  const { theme, toggle } = useTheme();

  const projectID = params?.id;
  const projectName = projectID === DEMO_PROJECT_ID ? "Built-in demo" : project?.id === projectID ? project?.name : undefined;
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

  const mode = projectID ? `project:${projectID}` : "global";

  return (
    <header data-tw className="sticky top-0 z-40 border-b border-border bg-chrome/95 backdrop-blur supports-[backdrop-filter]:bg-chrome/80">
      <div className="flex h-14 items-center gap-3 px-4 md:px-6">
        <Link href="/" aria-label="ReWeird dashboard" className="grid size-9 place-items-center rounded-lg text-signal transition-colors hover:bg-accent">
          <Waves className="size-[22px]" strokeWidth={1.75} />
        </Link>

        <nav aria-label="Breadcrumb" className="flex min-w-0 flex-1 items-center gap-2 text-sm">
          <AnimatePresence mode="popLayout" initial={false}>
            {projectID ? (
              <motion.div key="crumbs-project" className="flex min-w-0 items-center gap-2" initial={{ opacity: 0, x: -6 }} animate={{ opacity: 1, x: 0 }} exit={{ opacity: 0, x: -6 }} transition={spring}>
                <Link href="/projects" className="whitespace-nowrap text-muted-foreground transition-colors hover:text-foreground">Projects</Link>
                <span className="text-subtle">/</span>
                <Link href={base} className="flex min-w-0 items-center gap-1.5 font-semibold text-foreground">
                  <span className="truncate">{projectName ?? "Loading…"}</span>
                  <ChevronDown className="size-3.5 shrink-0 text-subtle" />
                </Link>
              </motion.div>
            ) : (
              <motion.span key="crumbs-global" className="font-semibold tracking-tight text-foreground" initial={{ opacity: 0, x: 6 }} animate={{ opacity: 1, x: 0 }} exit={{ opacity: 0, x: 6 }} transition={spring}>
                ReWeird
              </motion.span>
            )}
          </AnimatePresence>
        </nav>

        <div className="flex items-center gap-1.5">
          <Tooltip>
            <TooltipTrigger asChild>
              <Button variant="outline" size="icon-sm" onClick={() => setShowNewProject(true)} aria-label="New project" className="active:scale-95"><Plus /></Button>
            </TooltipTrigger>
            <TooltipContent>New project</TooltipContent>
          </Tooltip>
          <Tooltip>
            <TooltipTrigger asChild>
              <Button variant="outline" size="icon-sm" onClick={toggle} aria-label={`Switch to ${theme === "dark" ? "light" : "dark"} mode`} className="active:scale-95">
                {theme === "dark" ? <Sun /> : <Moon />}
              </Button>
            </TooltipTrigger>
            <TooltipContent>{theme === "dark" ? "Light mode" : "Dark mode"}</TooltipContent>
          </Tooltip>
          <AccountMenu onLoadDemo={loadDemoProject} />
        </div>
      </div>

      <LayoutGroup id="primary-nav">
        <nav aria-label={projectID ? "Project navigation" : "Primary navigation"} className="relative h-11 overflow-x-auto overflow-y-hidden px-2 md:px-4 [scrollbar-width:none]">
          <AnimatePresence mode="wait" initial={false}>
            <motion.div
              key={mode}
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
                  <motion.div
                    key={item.key}
                    className="relative flex"
                    variants={{ hidden: { opacity: 0, y: reduce ? 0 : 6 }, show: { opacity: 1, y: 0, transition: spring } }}
                  >
                    <Link
                      href={item.href}
                      aria-current={item.active ? "page" : undefined}
                      className={cn(
                        "group my-1.5 flex items-center gap-2 rounded-md px-2.5 text-[13px] whitespace-nowrap transition-colors",
                        item.active ? "font-semibold text-foreground" : "font-medium text-muted-foreground hover:bg-accent hover:text-foreground",
                      )}
                    >
                      <Icon className={cn("size-4", item.active ? "text-foreground" : "text-subtle group-hover:text-muted-foreground")} strokeWidth={1.5} />
                      {item.label}
                      {item.dot && <span className="size-1.5 rounded-full bg-signal" aria-label="Awaiting review" />}
                    </Link>
                    {item.active && (
                      <motion.span layoutId="nav-active" transition={spring} className="absolute inset-x-2 -bottom-px h-0.5 rounded-full bg-signal" />
                    )}
                  </motion.div>
                );
              })}
            </motion.div>
          </AnimatePresence>
        </nav>
      </LayoutGroup>
    </header>
  );
}
