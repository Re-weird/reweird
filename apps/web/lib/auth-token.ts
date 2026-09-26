type TokenGetter = () => Promise<string | null>;

let getter: TokenGetter | null = null;

export function setTokenGetter(fn: TokenGetter | null) {
  getter = fn;
}

/** Used by requestJSON() to attach a verified Clerk bearer token when a session exists. */
export async function getAuthToken(): Promise<string | null> {
  if (!getter) return null;
  try {
    return await getter();
  } catch {
    return null;
  }
}
