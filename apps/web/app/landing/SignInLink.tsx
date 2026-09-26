"use client";

import { LogIn } from "lucide-react";
import { signIn } from "next-auth/react";
import { AUTH_ENABLED } from "@/lib/auth";

export function SignInLink({ className }: { className?: string }) {
  const content = (
    <>
      <span><LogIn size={14} /></span>
      <div><strong>Sign in</strong><small>Save your own projects</small></div>
    </>
  );

  if (!AUTH_ENABLED) {
    return (
      <a href="/" className={className} title="Google sign-in isn't configured on this deployment yet.">
        {content}
      </a>
    );
  }

  return (
    <button className={className} type="button" style={{ width: "100%", textAlign: "left" }} onClick={() => signIn("google")}>{content}</button>
  );
}
