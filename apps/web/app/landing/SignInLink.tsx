"use client";

import { LogIn } from "lucide-react";
import { SignInButton } from "@clerk/nextjs";
import { CLERK_ENABLED } from "@/lib/clerk";

export function SignInLink({ className }: { className?: string }) {
  const content = (
    <>
      <span><LogIn size={14} /></span>
      <div><strong>Sign in</strong><small>Save your own projects</small></div>
    </>
  );

  if (!CLERK_ENABLED) {
    return (
      <a href="/" className={className} title="Google sign-in isn't configured on this deployment yet.">
        {content}
      </a>
    );
  }

  return (
    <SignInButton mode="modal">
      <button className={className} type="button" style={{ width: "100%", textAlign: "left" }}>{content}</button>
    </SignInButton>
  );
}
