import type { Metadata } from "next";
import { LandingPage } from "./landing/LandingPage";

export const metadata: Metadata = {
  title: "ReWeird — Hardware diagnostics, guided by evidence",
  description: "Understand your project, measure what's actually happening, find the fault, verify the fix.",
};

export default function Page() {
  return <LandingPage />;
}
