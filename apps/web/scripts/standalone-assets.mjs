// Preserve standalone deployment assets on both Windows and Linux.
import {mkdir, cp} from "node:fs/promises";
await mkdir(".next/standalone/apps/web/.next", {recursive: true});
await cp("public", ".next/standalone/apps/web/public", {recursive: true});
await cp(".next/static", ".next/standalone/apps/web/.next/static", {recursive: true});
