import type { NextConfig } from "next";

const hardwareConfig: NextConfig = {
  output: "standalone",
  transpilePackages: ["@reweird/shared-types"],
  // Hosts (e.g. an ngrok tunnel) allowed to use the dev server's live-reload
  // and dev resources. Comma-separated; dev only.
  allowedDevOrigins: process.env.NEXT_DEV_ALLOWED_ORIGINS?.split(",").map((host) => host.trim()).filter(Boolean),
  async rewrites() {
    const api = process.env.API_INTERNAL_URL ?? "http://127.0.0.1:8080";
    // Excludes /api/auth/* so NextAuth's own route handlers (app/api/auth/**)
    // handle those requests instead of being proxied to the Go backend, which
    // has no routes for them.
    return [
      { source: "/api/:path((?!auth/).*)", destination: `${api}/api/:path` },
      // GitHub App webhooks. The API checks each delivery's HMAC signature, so
      // exposing this path through the web app's public URL is safe.
      { source: "/webhooks/github", destination: `${api}/webhooks/github` },
    ];
  },
};

// Judge builds contain only the explicitly named demo routes. Auth handlers,
// project uploads, API rewrites, and live hardware pages are not deployed.
const judgeConfig: NextConfig = {
  output: "export",
  pageExtensions: ["judge.tsx", "judge.ts"],
  trailingSlash: true,
  basePath: process.env.REWEIRD_BASE_PATH ?? "",
  env: { NEXT_PUBLIC_DEMO_BASE_PATH: process.env.REWEIRD_BASE_PATH ?? "", NEXT_PUBLIC_JUDGE_MODE: "true" },
  images: { unoptimized: true },
};

export default process.env.REWEIRD_APP_MODE === "hardware" ? hardwareConfig : judgeConfig;
