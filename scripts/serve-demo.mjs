// Serve the exported demo without a backend, credentials, or extra dependencies.
import { createServer } from "node:http";
import { readFile, stat } from "node:fs/promises";
import { resolve, extname, sep } from "node:path";
import { fileURLToPath } from "node:url";

const root = fileURLToPath(new URL("../apps/web/out/", import.meta.url));
const basePath = process.env.REWEIRD_BASE_PATH ?? "";
const types = { ".html": "text/html", ".js": "text/javascript", ".css": "text/css", ".json": "application/json", ".txt": "text/plain", ".png": "image/png", ".svg": "image/svg+xml", ".ico": "image/x-icon", ".woff2": "font/woff2" };
try { await stat(resolve(root, "index.html")); }
catch { console.error("Build the demo first: npm run build"); process.exit(1); }
createServer(async (req, res) => {
  res.setHeader("X-Content-Type-Options", "nosniff");
  if (req.method !== "GET" && req.method !== "HEAD") { res.writeHead(405); res.end(); return; }
  try {
    let pathname = decodeURIComponent(new URL(req.url, "http://localhost").pathname);
    if (basePath && pathname !== basePath && !pathname.startsWith(basePath + "/")) throw new Error("Not found");
    pathname = pathname.slice(basePath.length);
    let file = resolve(root, "." + (pathname || "/"));
    if (file !== resolve(root) && !file.startsWith(resolve(root) + sep)) throw new Error("Not found");
    if ((await stat(file)).isDirectory()) file = resolve(file, "index.html");
    const body = await readFile(file);
    res.writeHead(200, { "Content-Type": types[extname(file)] ?? "application/octet-stream" });
    res.end(req.method === "HEAD" ? undefined : body);
  } catch {
    res.writeHead(404, { "Content-Type": "text/html" });
    const body = await readFile(resolve(root, "404.html")).catch(() => "Not found");
    res.end(req.method === "HEAD" ? undefined : body);
  }
}).listen(Number(process.env.PORT ?? 3000), process.env.HOST ?? "127.0.0.1", () => console.log(`ReWeird demo: http://${process.env.HOST ?? "127.0.0.1"}:${process.env.PORT ?? 3000}${basePath}/`));
