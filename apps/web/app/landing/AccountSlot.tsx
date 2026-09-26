"use client";

import { LogOut } from "lucide-react";
import { signOut } from "next-auth/react";
import { useUser } from "@/lib/auth";
import { SignInLink } from "./SignInLink";

/** Rendered in the app sidebar: account controls when signed in, a sign-in prompt otherwise. */
export function AccountSlot() {
  const { isSignedIn, name, email, imageUrl } = useUser();

  if (!isSignedIn) {
    return <SignInLink className="operator signin-link" />;
  }

  return (
    <div className="operator">
      {imageUrl ? (
        <img src={imageUrl} alt="" width={31} height={31} style={{ borderRadius: "50%" }} />
      ) : (
        <span className="device-icon" aria-hidden="true">{(name ?? email ?? "?").charAt(0).toUpperCase()}</span>
      )}
      <div><strong>{name ?? "Signed in"}</strong><small>{email ?? "ReWeird account"}</small></div>
      <button type="button" className="icon-button" aria-label="Sign out" title="Sign out" onClick={() => signOut({ callbackUrl: "/" })}>
        <LogOut size={15} />
      </button>
    </div>
  );
}
