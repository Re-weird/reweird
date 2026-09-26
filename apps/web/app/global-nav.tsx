"use client";

import { useEffect, useState } from "react";
import Link from "next/link";
import { usePathname } from "next/navigation";
import { BarChart3, FolderGit2, Moon, Plus, Settings, Sun, Waves } from "lucide-react";
import { useAppState } from "@/lib/app-state";

type Theme = "light" | "dark";

export function GlobalNav() {
  const pathname = usePathname();
  const { setShowNewProject, loadDemoProject } = useAppState();
  const [theme, setTheme] = useState<Theme>("dark");

  useEffect(() => {
    let stored: string | null = null;
    try { stored = window.localStorage.getItem("reweird-theme"); } catch { /* Theme still works when browser storage is unavailable. */ }
    const nextTheme: Theme = stored === "light" || stored === "dark"
      ? stored
      : window.matchMedia("(prefers-color-scheme: light)").matches ? "light" : "dark";
    setTheme(nextTheme);
    document.documentElement.dataset.theme = nextTheme;
  }, []);

  const toggleTheme = () => {
    const nextTheme: Theme = theme === "dark" ? "light" : "dark";
    setTheme(nextTheme);
    document.documentElement.dataset.theme = nextTheme;
    try { window.localStorage.setItem("reweird-theme", nextTheme); } catch { /* Keep the in-session preference. */ }
  };

  return (
    <header className="global-nav">
      <Link href="/" className="global-nav-brand"><Waves size={20} /><span>Re<span>Weird</span></span></Link>
      <nav aria-label="Primary navigation" className="global-nav-links">
        <Link href="/projects" aria-current={pathname?.startsWith("/projects") ? "page" : undefined} className={pathname?.startsWith("/projects") ? "active" : ""}><FolderGit2 size={15} /> Projects</Link>
        <Link href="/" aria-current={pathname === "/" ? "page" : undefined} className={pathname === "/" ? "active" : ""}><BarChart3 size={15} /> Account</Link>
        <Link href="/settings" aria-current={pathname === "/settings" ? "page" : undefined} className={pathname === "/settings" ? "active" : ""}><Settings size={15} /> Settings</Link>
      </nav>
      <div className="global-nav-actions">
        <button className="text-button" onClick={loadDemoProject}>Load demo</button>
        <button className="primary compact" onClick={() => setShowNewProject(true)}><Plus size={15} /> New project</button>
        <button className="icon-button theme-toggle" onClick={toggleTheme} aria-label={`Switch to ${theme === "dark" ? "light" : "dark"} mode`} title={`Switch to ${theme === "dark" ? "light" : "dark"} mode`}>
          {theme === "dark" ? <Sun size={16} /> : <Moon size={16} />}
        </button>
      </div>
    </header>
  );
}
