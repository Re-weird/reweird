import type { DemoSession } from "@reweird/shared-types";

const API_URL = process.env.NEXT_PUBLIC_API_URL ?? "http://localhost:8080";

async function request(path: string, init?: RequestInit): Promise<DemoSession | null> {
  const controller = new AbortController();
  const timer = setTimeout(() => controller.abort(), 900);
  try {
    const response = await fetch(`${API_URL}${path}`, {
      ...init,
      headers: { "Content-Type": "application/json", ...init?.headers },
      signal: controller.signal,
      cache: "no-store",
    });
    if (!response.ok) return null;
    return (await response.json()) as DemoSession;
  } catch {
    return null;
  } finally {
    clearTimeout(timer);
  }
}

export const demoApi = {
  load: () => request("/api/v1/demo/session"),
  wiggle: () => request("/api/v1/demo/wiggle", { method: "POST" }),
  repair: () => request("/api/v1/demo/repair", { method: "POST" }),
  reset: () => request("/api/v1/demo/reset", { method: "POST" }),
};
