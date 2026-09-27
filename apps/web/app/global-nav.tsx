"use client";

import { useEffect, useRef, useState } from "react";
import Image from "next/image";
import Link from "next/link";
import { useParams, usePathname } from "next/navigation";
import { AnimatePresence, motion } from "framer-motion";
import { signOut } from "next-auth/react";
import { BarChart3, ChevronDown, FolderGit2, LogOut, Moon, Plus, Settings, Sun, Waves } from "lucide-react";
import { Button } from "@/components/ui/button";
import { Tooltip, TooltipContent, TooltipTrigger } from "@/components/ui/tooltip";
import { useAccountActivity } from "@/lib/account-activity";
import { useAppState } from "@/lib/app-state";
import { DEMO_PROJECT_ID } from "@/lib/project-routes";
import { useUser } from "@/lib/auth";
import { cn } from "@/lib/utils";

type Theme = "light" | "dark";

const spring = { type: "spring", stiffness: 380, damping: 32 } as const;


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
  const { isSignedIn, name, email } = useUser();

  useEffect(() => { setOpen(false); }, [pathname]);
  useEffect(() => {
    if (!open) return;
    const close = (event: MouseEvent) => { if (!ref.current?.contains(event.target as Node)) setOpen(false); };
    const escape = (event: KeyboardEvent) => { if (event.key === "Escape") setOpen(false); };
    document.addEventListener("mousedown", close);
    document.addEventListener("keydown", escape);
    return () => { document.removeEventListener("mousedown", close); document.removeEventListener("keydown", escape); };
  }, [open]);

  const activity = useAccountActivity();
  const github = activity.status === "ready" ? activity.data.github : null;
  // Same picture as the profile rail: GitHub's, never the Google photo.
  const githubLogin = github?.connected ? github.github_user || github.account_login : undefined;
  const avatar = githubLogin ? `https://github.com/${encodeURIComponent(githubLogin)}.png?size=64` : null;

  const itemClass = "flex items-center gap-2.5 rounded-md px-2.5 py-2 text-[13px] text-foreground transition-colors hover:bg-accent [&_svg]:size-4 [&_svg]:text-muted-foreground";

  return (
    <div ref={ref} className="relative">
      <button
        onClick={() => setOpen((value) => !value)}
        aria-haspopup="menu"
        aria-expanded={open}
        aria-label="Account menu"
        className="grid size-8 place-items-center overflow-hidden rounded-full border border-border bg-surface-3 font-mono text-[11px] font-medium text-foreground transition-transform active:scale-95"
      >
        {/* eslint-disable-next-line @next/next/no-img-element */}
        {avatar ? <img src={avatar} alt="" className="size-full object-cover" /> : "RW"}
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
              <p className="text-[13px] font-semibold text-foreground">{isSignedIn ? (name ?? "Signed in") : "Local session"}</p>
              <p className="text-xs text-muted-foreground">{isSignedIn ? (email ?? "ReWeird account") : "No user sign-in yet"}</p>
            </div>
            <Link href="/dashboard" role="menuitem" className={itemClass}><BarChart3 /> Your dashboard</Link>
            <Link href="/projects" role="menuitem" className={itemClass}><FolderGit2 /> Your projects</Link>
            <Link href="/settings" role="menuitem" className={itemClass}><Settings /> Settings</Link>
            <div className="my-1 h-px bg-line-soft" />
            <button role="menuitem" onClick={() => { setOpen(false); onLoadDemo(); }} className={cn(itemClass, "text-left")}><Waves /> Load demo project</button>
            {isSignedIn && (
              <button role="menuitem" onClick={() => signOut({ callbackUrl: "/" })} className={cn(itemClass, "text-left")}><LogOut /> Sign out</button>
            )}
          </motion.div>
        )}
      </AnimatePresence>
    </div>
  );
}

export function GlobalNav() {
  const pathname = usePathname() ?? "/";
  const params = useParams<{ id?: string }>();
  const { project, setShowNewProject, loadDemoProject } = useAppState();
  const { theme, toggle } = useTheme();

  const projectID = params?.id;
  const projectName = projectID === DEMO_PROJECT_ID ? "Built-in demo" : project?.id === projectID ? project?.name : undefined;
  const base = projectID ? `/projects/${projectID}` : "";

  // The public landing page (/) and the legacy single-page workbench (/app)
  // own their own header/chrome; this global nav is only for the routed
  // dashboard/projects app. The /demo chooser and /try judge demo own their own shell too.
  if (pathname === "/" || pathname.startsWith("/app") || pathname.startsWith("/try") || pathname === "/demo") return null;

  return (
    <header data-tw className="sticky top-0 z-40 border-b border-border bg-chrome/95 backdrop-blur supports-[backdrop-filter]:bg-chrome/80">
      <div className="flex h-14 items-center gap-3 px-4 md:px-6">
        {/* Logo from main; crop and dark-mode lift match main's .brand-logo-image. */}
        <Link href="/dashboard" aria-label="ReWeird dashboard" className="relative block h-[34px] w-[66px] shrink-0 overflow-hidden rounded-md transition-opacity hover:opacity-85">
          <Image src="/images/reweird-logo.png" alt="" width={72} height={72} priority className="absolute top-[-22px] left-[-3px] size-[72px] max-w-none dark:[filter:contrast(.55)_brightness(1.3)]" />
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

    </header>
  );
}
