"use client";

import { useSession } from "next-auth/react";

/**
 * Whether Google sign-in is configured for this build. Read once at module
 * load from the inlined NEXT_PUBLIC_ env var - this never changes at
 * runtime, so picking a hook implementation based on it below does not
 * violate the rules of hooks (every render of a given component calls the
 * same fixed function reference for the lifetime of the app).
 */
export const AUTH_ENABLED = !!process.env.NEXT_PUBLIC_GOOGLE_CLIENT_ID;

export interface AuthUser {
  isLoaded: boolean;
  isSignedIn: boolean;
  name: string | null;
  email: string | null;
  imageUrl: string | null;
}

export interface AuthState {
  isLoaded: boolean;
  isSignedIn: boolean;
  getToken: () => Promise<string | null>;
}

async function fetchToken(): Promise<string | null> {
  try {
    const response = await fetch("/api/auth/token");
    if (!response.ok) return null;
    const data = (await response.json()) as { token: string | null };
    return data.token;
  } catch {
    return null;
  }
}

function useUserReal(): AuthUser {
  const { status, data } = useSession();
  return {
    isLoaded: status !== "loading",
    isSignedIn: status === "authenticated",
    name: data?.user?.name ?? null,
    email: data?.user?.email ?? null,
    imageUrl: data?.user?.image ?? null,
  };
}

function useUserStub(): AuthUser {
  return { isLoaded: true, isSignedIn: false, name: null, email: null, imageUrl: null };
}

function useAuthReal(): AuthState {
  const { status } = useSession();
  return { isLoaded: status !== "loading", isSignedIn: status === "authenticated", getToken: fetchToken };
}

function useAuthStub(): AuthState {
  return { isLoaded: true, isSignedIn: false, getToken: async () => null };
}

export const useUser: () => AuthUser = AUTH_ENABLED ? useUserReal : useUserStub;
export const useAuth: () => AuthState = AUTH_ENABLED ? useAuthReal : useAuthStub;
