// Where to send the user after the GitHub App install round trip. GitHub
// only knows the fixed setup URL, so the page they started from is kept in
// sessionStorage (per tab) and read back by /settings/github/callback.
const KEY = "reweird:github-return";

export interface GitHubReturn {
  path: string;
  /** Reopen the new-project dialog so the user can pick a repo straight away. */
  reopenNewProject: boolean;
}

export function rememberGitHubReturn(value: GitHubReturn) {
  try { sessionStorage.setItem(KEY, JSON.stringify(value)); } catch { /* storage blocked: fall back to Settings */ }
}

export function takeGitHubReturn(): GitHubReturn {
  const fallback = { path: "/settings", reopenNewProject: false };
  try {
    const raw = sessionStorage.getItem(KEY);
    sessionStorage.removeItem(KEY);
    if (!raw) return fallback;
    const parsed = JSON.parse(raw) as Partial<GitHubReturn>;
    // Only same-site paths, so a stored value can't redirect off ReWeird.
    const path = typeof parsed.path === "string" && parsed.path.startsWith("/") && !parsed.path.startsWith("//") ? parsed.path : fallback.path;
    return { path, reopenNewProject: parsed.reopenNewProject === true };
  } catch {
    return fallback;
  }
}
