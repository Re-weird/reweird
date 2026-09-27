import type { Metadata } from "next";
import "./judge-entry.css";

export const metadata: Metadata = {
  title: "ReWeird — Interactive Demo",
  description: "Break a simulated circuit, investigate the evidence, and verify your fix. No sign-in, API keys, or hardware required.",
};

export default function JudgeLayout({ children }: Readonly<{ children: React.ReactNode }>) {
  return <html lang="en"><body>{children}</body></html>;
}
