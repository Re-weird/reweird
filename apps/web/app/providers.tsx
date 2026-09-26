"use client";

import { useEffect } from "react";
import { SessionProvider } from "next-auth/react";
import { AUTH_ENABLED, useAuth } from "@/lib/auth";
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
  if (!AUTH_ENABLED) return <>{children}</>;
  return (
    <SessionProvider>
      <TokenBridge />
      {children}
    </SessionProvider>
  );
}
