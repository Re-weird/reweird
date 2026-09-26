import type { Metadata } from "next";
import "./globals.css";
import "./workbench.css";
import "./circuit-map.css";

export const metadata: Metadata = {
  title: "ReWeird — Hardware Diagnostics",
  description: "Evidence-first diagnostics for physical electronics projects.",
  icons: { icon: "/images/reweird-logo.png" },
};

export default function RootLayout({ children }: Readonly<{ children: React.ReactNode }>) {
  return (
    <html lang="en">
      <body>{children}</body>
    </html>
  );
}
