import type { Metadata } from "next";
import Link from "next/link";
import { ArrowRight, Cable, FlaskConical } from "lucide-react";

export const metadata: Metadata = {
  title: "ReWeird demo",
  description: "Explore the actual ReWeird app with simulated values, or try the circuit challenge.",
};

// Open the real software interface with browser-generated measurements.
const PROJECT_DEMO_HREF = "/software";

export default function DemoChooserPage() {
  return <div className="jd-page">
    <main className="jd-main jd-chooser">
      <span className="bench-label">ReWeird demo</span>
      <h1>How do you want to see it?</h1>
      <div className="jd-chooser-grid">
        <Link href={PROJECT_DEMO_HREF} className="bench-panel jd-chooser-card">
          <Cable size={22} />
          <strong>Project demo</strong>
          <p>Explore the actual ReWeird workbench, circuit map, signal monitor, diagnosis, guided tests, and repair verification with simulated values. No sign-in or hardware needed.</p>
          <span className="jd-chooser-tag live">APP + SIMULATED VALUES</span>
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
