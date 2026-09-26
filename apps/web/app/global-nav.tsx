"use client";

import { useEffect, useRef, useState } from "react";
import Link from "next/link";
import { useParams, usePathname } from "next/navigation";
import { BarChart3, Box, Cable, CheckCircle2, Cpu, FileBarChart, FolderGit2, LayoutDashboard, Microscope, Moon, Plus, RefreshCw, Settings, Sun, TestTube2, Waves } from "lucide-react";
import { useAppState } from "@/lib/app-state";
import { DEMO_PROJECT_ID, projectTabs, type ProjectTabID } from "@/lib/project-routes";

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

type Theme = "light" | "dark";

const pageTitles: Record<string, string> = { "/": "Dashboard", "/projects": "Projects", "/settings": "Settings" };

export function GlobalNav() {
  const pathname = usePathname() ?? "/";
  const params = useParams<{ id?: string }>();
  const { project, session, setShowNewProject, loadDemoProject } = useAppState();
  const [theme, setTheme] = useState<Theme>("dark");
  const [menuOpen, setMenuOpen] = useState(false);
  const menuRef = useRef<HTMLDivElement>(null);

  useEffect(() => {
    let stored: string | null = null;
    try { stored = window.localStorage.getItem("reweird-theme"); } catch { /* Theme still works when browser storage is unavailable. */ }
    const nextTheme: Theme = stored === "light" || stored === "dark"
      ? stored
      : window.matchMedia("(prefers-color-scheme: light)").matches ? "light" : "dark";
    setTheme(nextTheme);
    document.documentElement.dataset.theme = nextTheme;
  }, []);

  useEffect(() => {
    if (!menuOpen) return;
    const close = (event: MouseEvent) => { if (!menuRef.current?.contains(event.target as Node)) setMenuOpen(false); };
    const escape = (event: KeyboardEvent) => { if (event.key === "Escape") setMenuOpen(false); };
    document.addEventListener("mousedown", close);
    document.addEventListener("keydown", escape);
    return () => { document.removeEventListener("mousedown", close); document.removeEventListener("keydown", escape); };
  }, [menuOpen]);

  useEffect(() => { setMenuOpen(false); }, [pathname]);

  const toggleTheme = () => {
    const nextTheme: Theme = theme === "dark" ? "light" : "dark";
    setTheme(nextTheme);
    document.documentElement.dataset.theme = nextTheme;
    try { window.localStorage.setItem("reweird-theme", nextTheme); } catch { /* Keep the in-session preference. */ }
  };

  const projectID = params?.id;
  const projectName = projectID === DEMO_PROJECT_ID ? "Built-in demo" : project?.id === projectID ? project?.name : undefined;

  const base = projectID ? `/projects/${projectID}` : "";

  return (
    <header className="global-header">
    <div className="global-nav">
      <Link href="/" className="global-nav-brand" aria-label="ReWeird dashboard"><Waves size={22} /></Link>
      <nav className="global-nav-crumbs" aria-label="Breadcrumb">
        {projectID ? <>
          <Link href="/projects">Projects</Link>
          <span className="crumb-sep">/</span>
          <Link href={`/projects/${projectID}`} className="crumb-current">{projectName ?? "Loading…"}</Link>
        </> : <span className="crumb-current">{pageTitles[pathname] ?? "ReWeird"}</span>}
      </nav>
      <div className="global-nav-actions">
        <button className="icon-button" onClick={() => setShowNewProject(true)} aria-label="New project" title="New project"><Plus size={16} /></button>
        <button className="icon-button theme-toggle" onClick={toggleTheme} aria-label={`Switch to ${theme === "dark" ? "light" : "dark"} mode`} title={`Switch to ${theme === "dark" ? "light" : "dark"} mode`}>
          {theme === "dark" ? <Sun size={16} /> : <Moon size={16} />}
        </button>
        <div className="avatar-menu" ref={menuRef}>
          <button className="avatar-button" onClick={() => setMenuOpen((open) => !open)} aria-haspopup="menu" aria-expanded={menuOpen} aria-label="Account menu">RW</button>
          {menuOpen && <div className="avatar-dropdown" role="menu">
            <div className="avatar-dropdown-head"><strong>Local session</strong><small>No user sign-in</small></div>
            <Link href="/" role="menuitem"><BarChart3 size={15} /> Your dashboard</Link>
            <Link href="/projects" role="menuitem"><FolderGit2 size={15} /> Your projects</Link>
            <Link href="/settings" role="menuitem"><Settings size={15} /> Settings</Link>
            <button role="menuitem" onClick={() => { setMenuOpen(false); loadDemoProject(); }}><Waves size={15} /> Load demo project</button>
          </div>}
        </div>
      </div>
    </div>
    {projectID && <nav className="project-tabs" aria-label="Project navigation">
      {projectTabs.map((tab) => {
        const Icon = tabIcons[tab.id];
        const href = tab.segment ? `${base}/${tab.segment}` : base;
        const active = pathname === href;
        return <Link key={tab.id} href={href} className={active ? "active" : ""} aria-current={active ? "page" : undefined}>
          <Icon size={15} /><span>{tab.label}</span>
          {tab.id === "diagnosis" && session.stage !== "verify" && <i />}
        </Link>;
      })}
    </nav>}
    </header>
  );
}
