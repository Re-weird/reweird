import { SignJWT } from "jose";
import { auth } from "@/auth";

/**
 * Mints a short-lived bearer token the browser attaches to apps/api
 * requests. apps/api never sees Google or NextAuth directly - it verifies
 * this token with the shared AUTH_TOKEN_SECRET and trusts its `sub` (the
 * Google account id) as the caller's identity. Five minutes is long enough
 * to cover a request round trip while keeping a leaked token short-lived.
 */
export async function GET() {
  const session = await auth();
  if (!session?.user) return Response.json({ token: null });

  const secret = process.env.AUTH_TOKEN_SECRET;
  if (!secret) return Response.json({ token: null });

  const userId = (session.user as { id?: string }).id;
  if (!userId) return Response.json({ token: null });

  const token = await new SignJWT({ email: session.user.email, name: session.user.name })
    .setProtectedHeader({ alg: "HS256" })
    .setSubject(userId)
    .setIssuedAt()
    .setExpirationTime("5m")
    .sign(new TextEncoder().encode(secret));

  return Response.json({ token });
}
