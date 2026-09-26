"use client";

import Link from "next/link";
import { SignInButton } from "@clerk/nextjs";
import { CLERK_ENABLED } from "@/lib/clerk";
import { GoogleMark } from "./GoogleMark";

export function ContinueWithGoogle({ className }: { className?: string }) {
  const content = (
    <>
      <GoogleMark /> Continue with Google
    </>
  );

  if (!CLERK_ENABLED) {
    return (
      <Link href="/app" className={className} title="Google sign-in isn't configured on this deployment yet — see design/design.md for setup.">
        {content}
      </Link>
    );
  }

  return (
    <SignInButton mode="modal" forceRedirectUrl="/app">
      <button className={className} type="button">{content}</button>
    </SignInButton>
  );
}
