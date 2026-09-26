"use client";

import { useEffect } from "react";
import { ClerkProvider } from "@clerk/nextjs";
import { CLERK_ENABLED, useAuth } from "@/lib/clerk";
import { setTokenGetter } from "@/lib/auth-token";

function TokenBridge() {
  const { getToken } = useAuth();
  useEffect(() => {
    setTokenGetter(getToken);
    return () => setTokenGetter(null);
  }, [getToken]);
  return null;
}

export function Providers({ children }: { children: React.ReactNode }) {
  if (!CLERK_ENABLED) return <>{children}</>;
  return (
    <ClerkProvider>
      <TokenBridge />
      {children}
    </ClerkProvider>
  );
}
