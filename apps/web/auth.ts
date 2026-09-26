import NextAuth from "next-auth";
import Google from "next-auth/providers/google";

/**
 * Google sign-in for the whole team. Session strategy is "jwt" (no
 * database) to match the rest of the stack, which is stateless. The Google
 * account id (`profile.sub`) is threaded through as the session's user id -
 * it's what apps/api's ownership scoping keys projects on, via the signed
 * token minted in app/api/auth/token/route.ts.
 */
export const { handlers, auth, signIn, signOut } = NextAuth({
  providers: [
    Google({
      clientId: process.env.GOOGLE_CLIENT_ID,
      clientSecret: process.env.GOOGLE_CLIENT_SECRET,
      // Without this, Google silently reuses whatever account is already
      // signed into the browser and skips its own picker/consent screen -
      // "click the button and you're just in" with no chance to choose or
      // confirm an account. Forcing the picker every time is the fix.
      authorization: { params: { prompt: "select_account" } },
    }),
  ],
  session: { strategy: "jwt" },
  callbacks: {
    jwt({ token, profile }) {
      if (profile?.sub) token.sub = profile.sub;
      return token;
    },
    session({ session, token }) {
      if (session.user && token.sub) (session.user as { id?: string }).id = token.sub;
      return session;
    },
  },
});
