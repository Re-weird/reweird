"use client";

import { useAuth as useClerkAuth, useUser as useClerkUser } from "@clerk/nextjs";

/**
 * Whether Clerk is configured for this build. Read once at module load from
 * the inlined NEXT_PUBLIC_ env var — this never changes at runtime, so
 * picking a hook implementation based on it below does not violate the
 * rules of hooks (every render of a given component calls the same fixed
 * function reference for the lifetime of the app).
 */
export const CLERK_ENABLED = !!process.env.NEXT_PUBLIC_CLERK_PUBLISHABLE_KEY;

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

function useUserReal(): AuthUser {
  const { isLoaded, isSignedIn, user } = useClerkUser();
  return {
    isLoaded,
    isSignedIn: !!isSignedIn,
    name: user?.fullName ?? null,
    email: user?.primaryEmailAddress?.emailAddress ?? null,
    imageUrl: user?.imageUrl ?? null,
  };
}

function useUserStub(): AuthUser {
  return { isLoaded: true, isSignedIn: false, name: null, email: null, imageUrl: null };
}

function useAuthReal(): AuthState {
  const { isLoaded, isSignedIn, getToken } = useClerkAuth();
  return { isLoaded, isSignedIn: !!isSignedIn, getToken: async () => getToken() };
}

function useAuthStub(): AuthState {
  return { isLoaded: true, isSignedIn: false, getToken: async () => null };
}

export const useUser: () => AuthUser = CLERK_ENABLED ? useUserReal : useUserStub;
export const useAuth: () => AuthState = CLERK_ENABLED ? useAuthReal : useAuthStub;
