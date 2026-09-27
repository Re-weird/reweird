// Scope only live/test APIs, never practice simulator requests.
export function scopeLiveRequest(path, pathname) {
  if (!/^\/api\/v1\/(session(?:\?|$)|telemetry\/status(?:\?|$)|tests(?:\/|\?|$))/.test(path)) return path;
  const match = /^\/projects\/([^/]+)/.exec(pathname);
  if (!match || match[1] === "demo" || pathname.endsWith("/simulator")) return path;
  const url = new URL(path, "https://reweird.invalid");
  url.searchParams.set("project_id", decodeURIComponent(match[1]));
  return url.pathname + url.search;
}
