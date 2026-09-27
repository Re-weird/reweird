import type { Metadata } from "next";
import Link from "next/link";
import { ArrowRight, Cable, FlaskConical } from "lucide-react";

export const metadata: Metadata = {
  title: "ReWeird demo",
  description: "Watch the live hardware project demo, or try ReWeird yourself with a simulated circuit.",
};

// "Project demo" starts from the landing page, where the team presents the
// live hardware walkthrough.
const PROJECT_DEMO_HREF = "/";

export default function DemoChooserPage() {
  return <div className="jd-page">
    <main className="jd-main jd-chooser">
      <span className="bench-label">ReWeird demo</span>
      <h1>How do you want to see it?</h1>
      <p className="jd-catchphrase">When hardware gets weird, <em>ReWeird it.</em></p>
      <div className="jd-chooser-grid">
        <Link href={PROJECT_DEMO_HREF} className="bench-panel jd-chooser-card">
          <Cable size={22} />
          <strong>Project demo</strong>
          <p>Our team walks you through ReWeird with the real ESP32 and HC-SR04 connected live. Every reading comes from the hardware on the table.</p>
          <span className="jd-chooser-tag live">LIVE HARDWARE</span>
          <span className="jd-chooser-go">Start the project demo <ArrowRight size={15} /></span>
        </Link>
        <Link href="/try" className="bench-panel jd-chooser-card">
          <FlaskConical size={22} />
          <strong>Try it yourself</strong>
          <p>No hardware needed. Break a simulated circuit, run tests, fix it, and prove the fix.</p>
          <span className="jd-chooser-tag">SIMULATION</span>
          <span className="jd-chooser-go">Start the challenge <ArrowRight size={15} /></span>
        </Link>
      </div>
    </main>
  </div>;
}
