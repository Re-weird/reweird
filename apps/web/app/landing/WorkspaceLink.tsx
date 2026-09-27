"use client";

import Link from "next/link";
import { signIn } from "next-auth/react";
import { AUTH_ENABLED, useUser } from "@/lib/auth";

// Opens the workspace. Signed-out visitors can't reach /dashboard (proxy.ts
// sends them back here), so for them this starts Google sign-in and lands
// on the dashboard afterwards.
export function WorkspaceLink({ className, children }: { className?: string; children: React.ReactNode }) {
  const { isSignedIn } = useUser();
  if (process.env.NEXT_PUBLIC_JUDGE_MODE === "true") return <Link href="/software" className={className}>{children}</Link>;
  if (!AUTH_ENABLED || isSignedIn) return <Link href="/dashboard" className={className}>{children}</Link>;
  return (
    <button type="button" className={className} style={className ? { cursor: "pointer", fontFamily: "inherit" } : { cursor: "pointer", font: "inherit", color: "inherit", background: "none", border: 0, padding: 0, display: "inline-flex", alignItems: "center", gap: 6 }} onClick={() => signIn("google", { callbackUrl: "/dashboard" })}>
      {children}
    </button>
  );
}
