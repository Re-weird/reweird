"use client";

import Link from "next/link";
import { signIn } from "next-auth/react";
import { AUTH_ENABLED } from "@/lib/auth";
import { GoogleMark } from "./GoogleMark";

export function ContinueWithGoogle({ className }: { className?: string }) {
  const content = (
    <>
      <GoogleMark /> Continue with Google
    </>
  );

  if (!AUTH_ENABLED) {
    return (
      <Link href="/app" className={className} title="Google sign-in isn't configured on this deployment yet — see design/design.md for setup.">
        {content}
      </Link>
    );
  }

  return (
    <button className={className} type="button" onClick={() => signIn("google", { callbackUrl: "/app" })}>{content}</button>
  );
}
