import { NextResponse } from "next/server";
import { auth } from "@/auth";

// Signed-out visitors to app pages go to the landing page, which has the
// sign-in button. This is an optimistic check on the session cookie; the
// API still verifies every request itself. The guard is off when Google
// sign-in isn't configured, so a local setup without accounts keeps working.
const signInConfigured = Boolean(process.env.GOOGLE_CLIENT_ID && process.env.AUTH_SECRET);

function isPublicPath(pathname: string) {
  // The built-in demo is what the landing page's "Try demo" opens. The
  // Physical Git demo project (apps/api/internal/demodata) is a second,
  // separate anonymous demo -- a judge must reach it without signing in.
  return (
    pathname === "/projects/demo" || pathname.startsWith("/projects/demo/") ||
    pathname === "/projects/physical-git-demo" || pathname.startsWith("/projects/physical-git-demo/")
  );
}

export default auth((request) => {
  if (!signInConfigured || request.auth || isPublicPath(request.nextUrl.pathname)) return NextResponse.next();
  return NextResponse.redirect(new URL("/", request.nextUrl));
});

export const config = {
  matcher: ["/dashboard/:path*", "/projects/:path*", "/settings/:path*"],
};
