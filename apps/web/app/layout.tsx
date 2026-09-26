import type { Metadata } from "next";
import "./tailwind.css";
import { Providers } from "./providers";
import { AppStateProvider } from "@/lib/app-state";
import { TooltipProvider } from "@/components/ui/tooltip";
import { GlobalNav } from "./global-nav";
import { AppChrome } from "./app-chrome";

export const metadata: Metadata = {
  title: "ReWeird — Hardware Diagnostics",
  description: "Evidence-first diagnostics for physical electronics projects.",
  icons: { icon: "/images/reweird-logo.png" },
};

export default function RootLayout({ children }: Readonly<{ children: React.ReactNode }>) {
  return (
    <html lang="en">
      <body>
        <Providers>
          <AppStateProvider>
            <TooltipProvider delayDuration={150}>
              <GlobalNav />
              <AppChrome>{children}</AppChrome>
            </TooltipProvider>
          </AppStateProvider>
        </Providers>
      </body>
    </html>
  );
}
