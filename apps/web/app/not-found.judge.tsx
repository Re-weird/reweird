import Link from "next/link";

export default function NotFound() {
  return <main className="jd-main jd-chooser"><h1>Open the ReWeird demo</h1>
    <p>This link belongs to the live hardware app. The judge demo runs entirely in your browser.</p>
    <Link href="/" className="primary">Start the demo</Link>
  </main>;
}
