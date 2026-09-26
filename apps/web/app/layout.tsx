import type { Metadata } from "next";
import "./globals.css";
import "./workbench.css";
import { AppStateProvider } from "@/lib/app-state";
import { GlobalNav } from "./global-nav";
import { AppChrome } from "./app-chrome";

export const metadata: Metadata = {
  title: "ReWeird — Hardware Diagnostics",
  description: "Evidence-first diagnostics for physical electronics projects.",
};

export default function RootLayout({ children }: Readonly<{ children: React.ReactNode }>) {
  return (
    <html lang="en">
      <body>
        <AppStateProvider>
          <GlobalNav />
          <AppChrome>{children}</AppChrome>
        </AppStateProvider>
      </body>
    </html>
  );
}
