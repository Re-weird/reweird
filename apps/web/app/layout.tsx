import type { Metadata } from "next";
import "./globals.css";
import "./workbench.css";
import { Providers } from "./providers";

export const metadata: Metadata = {
  title: "ReWeird — Hardware Diagnostics",
  description: "Evidence-first diagnostics for physical electronics projects.",
};

export default function RootLayout({ children }: Readonly<{ children: React.ReactNode }>) {
  return (
    <html lang="en">
      <body>
        <Providers>{children}</Providers>
      </body>
    </html>
  );
}
