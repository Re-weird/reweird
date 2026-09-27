import type { Metadata } from "next";
import { JudgeDemo } from "../judge-demo";

export const metadata: Metadata = {
  title: "Try ReWeird — simulated judge demo",
  description: "Break a simulated circuit, follow the evidence, verify the fix. No hardware needed.",
};

export default function TryPage() {
  return <JudgeDemo />;
}
