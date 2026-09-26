"use client";

import { UserButton } from "@clerk/nextjs";
import { useUser } from "@/lib/clerk";
import { SignInLink } from "./SignInLink";

/** Rendered in the app sidebar: account controls when signed in, a sign-in prompt otherwise. */
export function AccountSlot() {
  const { isSignedIn, name, email } = useUser();

  if (!isSignedIn) {
    return <SignInLink className="operator signin-link" />;
  }

  return (
    <div className="operator">
      <UserButton
        appearance={{
          elements: {
            userButtonAvatarBox: { width: 31, height: 31 },
            userButtonPopoverCard: { fontFamily: "Manrope, sans-serif" },
          },
        }}
      />
      <div><strong>{name ?? "Signed in"}</strong><small>{email ?? "ReWeird account"}</small></div>
    </div>
  );
}
