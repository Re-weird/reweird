"use client";

import { useEffect, useRef, useState, type CSSProperties } from "react";
import dynamic from "next/dynamic";
import Link from "next/link";
import { ArrowDown, ArrowRight, ArrowUpRight, Check, ChevronRight, Code2, Cpu, Minus, Moon, Plus, RotateCcw, Sun } from "lucide-react";
import { useUser } from "@/lib/auth";
import { useTheme } from "@/lib/theme";
import { useReducedMotion } from "./scroll-story/useReducedMotion";
import { ContinueWithGoogle } from "./ContinueWithGoogle";
import { WorkspaceLink } from "./WorkspaceLink";
import s from "./product-story.module.css";

const Hardware = dynamic(() => import("./StoryHardware").then(mod => mod.StoryHardware), { ssr: false });
const clamp = (value: number) => Math.min(1, Math.max(0, value));
const chapters = ["Hardware", "Understand", "Map", "Measure", "Diagnose", "Verify"];
const chapterIds = ["hardware", "understand", "map", "measure", "diagnose", "verify"];
const codeLines = ["// A small program. A bigger picture.", "const int TRIG_PIN = 5;", "const int ECHO_PIN = 18;", "", "void setup() {", "  pinMode(TRIG_PIN, OUTPUT);", "  pinMode(ECHO_PIN, INPUT);", "}", "", "long duration = pulseIn(ECHO_PIN, HIGH);"];
const pins = [
  { name: "Power", pin: "VCC", value: "Supply", description: "Start with the supply. A signal only makes sense when its circuit has the power and reference it expects." },
  { name: "Trigger", pin: "GPIO 5", value: "Output", description: "Follow the trigger from the controller to the sensor. Compare the observed pulse with what the code asks for." },
  { name: "Echo", pin: "GPIO 18", value: "Input", description: "Trace the response back to the controller. A missing echo becomes evidence to investigate, not a conclusion by itself." },
];
const mapConnections = [
  { name: "Power", pin: "VCC · Controller", tone: "good", status: "Stable reading", detail: "Supply present and steady across the board — everything else on the map is checked against this baseline." },
  { name: "Trigger", pin: "GPIO 5 · Controller → Sensor", tone: "good", status: "Activity observed", detail: "The controller is issuing trigger pulses on schedule, matching what the profile expects." },
  { name: "Echo", pin: "GPIO 18 · Sensor → Controller", tone: "warn", status: "Awaiting a matching capture", detail: "The map still shows this connection from the profile — live tone appears once a stored capture matches it." },
  { name: "Status LED", pin: "GPIO 2 · Controller", tone: "muted", status: "No probe assigned", detail: "Not every connection needs a probe to appear on the map — this one is tracked without a live status." },
];

export function ProductStory() {
  const [theme, toggleTheme] = useTheme();
  const { isSignedIn } = useUser();
  const reduced = useReducedMotion();
  const root = useRef<HTMLDivElement>(null);
  const hero = useRef<HTMLElement>(null);
  const progress = useRef(0);
  const [active, setActive] = useState(0);
  const [pin, setPin] = useState(1);
  const [mapSelection, setMapSelection] = useState(0);
  const [fault, setFault] = useState(true);
  const [comparison, setComparison] = useState(55);
  const [open, setOpen] = useState<number | null>(0);

  useEffect(() => {
    let frame = 0;
    const update = () => {
      frame = 0;
      if (!root.current || !hero.current) return;
      const rect = hero.current.getBoundingClientRect();
      const p = clamp(-rect.top / Math.max(1, rect.height - window.innerHeight));
      progress.current = p;
      root.current.style.setProperty("--hero-p", String(p));
      root.current.style.setProperty("--dive", String(reduced ? 0 : clamp((p - .66) / .28)));
      root.current.style.setProperty("--hero-copy", String(reduced ? 1 : 1 - clamp((p - .08) / .22)));
      root.current.style.setProperty("--port-copy", String(reduced ? 0 : clamp((p - .25) / .15) * (1 - clamp((p - .63) / .16))));
      for (const section of root.current.querySelectorAll<HTMLElement>("[data-chapter]")) {
        const box = section.getBoundingClientRect();
        const reveal = clamp((window.innerHeight - box.top) / (window.innerHeight * .8));
        section.style.setProperty("--reveal", String(reveal));
        if (box.top < window.innerHeight * .5 && box.bottom > window.innerHeight * .5) setActive(Number(section.dataset.chapter));
      }
    };
    const onScroll = () => { if (!frame) frame = requestAnimationFrame(update); };
    update();
    window.addEventListener("scroll", onScroll, { passive: true });
    window.addEventListener("resize", onScroll);
    return () => { cancelAnimationFrame(frame); window.removeEventListener("scroll", onScroll); window.removeEventListener("resize", onScroll); };
  }, [reduced]);

  return <div ref={root} className={s.story}>
    <a href="#understand" className={s.skip}>Skip animation and explore ReWeird</a>
    <header className={s.header}>
      <Link href="/" className={s.brand} aria-label="ReWeird home"><img src="/images/reweird-logo-mark.png" alt="" width="52" height="26" /><span>ReWeird</span></Link>
      <nav aria-label="Main navigation"><a href="#understand">The experience</a><Link href="/projects/demo">Try demo <ArrowUpRight size={13} /></Link>
        {isSignedIn ? <Link href="/dashboard" className={s.navAction}>Workspace <ArrowRight size={14} /></Link> : <ContinueWithGoogle className={s.navAction} />}
        <button className={s.theme} onClick={toggleTheme} aria-label={`Switch to ${theme === "dark" ? "light" : "dark"} mode`}>{theme === "dark" ? <Sun size={16} /> : <Moon size={16} />}</button>
      </nav>
    </header>
    <aside className={s.chapterNav} aria-label="Story chapters">{chapters.map((chapter, index) => <a key={chapter} href={`#${chapterIds[index]}`} aria-label={chapter} aria-current={active === index ? "step" : undefined}><span>{String(index + 1).padStart(2, "0")}</span><i /><b>{chapter}</b></a>)}</aside>
    <main>
      <section ref={hero} id="hardware" data-chapter="0" className={`${s.hero} ${reduced ? s.reducedHero : ""}`}>
        <div className={s.heroStage}>
          <div className={s.stageGrid} aria-hidden="true" />
          <div className={s.heroCopy}><p className={s.eyebrow}><span /> HARDWARE INTELLIGENCE, MADE TANGIBLE</p><h1>Hardware speaks.<br /><em>Understand it.</em></h1><p className={s.lead}>Your code. Your circuit. The missing connection.<br />Follow the evidence from the first signal to the fix.</p><WorkspaceLink className={s.primary}>Enter the workspace <ArrowUpRight size={17} /></WorkspaceLink><small>Explore with simulated hardware. No setup needed.</small></div>
          <div className={s.heroModel}><Hardware progress={progress} reduced={reduced} light={theme === "light"} /></div>
          <div className={s.hardwareLabel}><span>01 / THE STARTING POINT</span><strong>ESP32</strong><p>A whole system.<br />Waiting to be understood.</p><div className={s.labelLine} /><small>DRAG TO EXPLORE ↔</small></div>
          <div className={s.portCopy}><span className={s.eyebrow}>LOOK A LITTLE CLOSER</span><h2>Every connection<br />has a story.</h2><p>Let’s follow one.</p></div>
          <div className={s.portTransition} aria-hidden="true"><div /><div /><div /><span>FROM HARDWARE TO UNDERSTANDING</span></div>
          <div className={s.heroFoot}><span>ESP32 / INTERACTIVE 3D STUDY</span><a href="#understand">Scroll to look inside <ArrowDown size={14} /></a><span>01 — 06</span></div>
        </div>
      </section>

      <section id="understand" data-chapter="1" className={`${s.chapter} ${s.understand}`}>
        <div className={s.sectionHeading}><p className={s.eyebrow}>01 / UNDERSTAND</p><h2>Before the first probe,<br /><em>know the project.</em></h2><p>Bring your code. ReWeird turns pin assignments, components, and expected behavior into a project you can reason about.</p></div>
        <div className={s.codeWorld}>
          <div className={s.source}><div className={s.instrumentHeader}><Code2 size={15} /><span>distance_sensor.ino</span><small>ILLUSTRATIVE PROJECT</small></div><pre>{codeLines.map((line, index) => <span key={index} className={(pin === 1 && [1, 5].includes(index)) || (pin === 2 && [2, 6, 9].includes(index)) ? s.codeActive : ""}><i>{String(index + 1).padStart(2,"0")}</i>{line || " "}</span>)}</pre><div className={s.sourceFoot}>SOURCE CODE <ArrowRight size={13} /> EXPECTED BEHAVIOR</div></div>
          <div className={s.interpretation}><div className={s.connectorLine} /><span className={s.eyebrow}>A PROJECT, MAPPED</span><h3>Connections.<br />With context.</h3><div className={s.pinList}>{pins.map((item, index) => <button key={item.name} aria-pressed={pin === index} onClick={() => setPin(index)}><span>{item.name}<small>{item.value}</small></span><b>{item.pin}</b><ChevronRight size={15} /></button>)}</div><p>Tap a signal to trace it through the code.</p></div>
        </div>
        <div className={s.sectionFoot}><span>UPLOAD → ANALYZE → REVIEW YOUR PROFILE</span><span>01 / 05</span></div>
      </section>

      <section id="map" data-chapter="2" className={s.chapter}>
        <div className={s.sectionHeading}><p className={s.eyebrow}>02 / MAP THE CIRCUIT</p><h2>Don&rsquo;t trace one wire.<br /><em>See the whole circuit.</em></h2><p>Every component and connection from your profile, laid out at once. A matching capture overlays a live status per connection — the map shows what the profile says should be there, not a verified photo of the wiring.</p></div>
        <div className={s.codeWorld}>
          <div className={s.source}><div className={s.instrumentHeader}><Cpu size={15} /><span>circuit_map.profile</span><small>PROFILE-DRIVEN TOPOLOGY</small></div><div className={s.mapList}>{mapConnections.map((item, index) => <button key={item.name} aria-pressed={mapSelection === index} onClick={() => setMapSelection(index)}><span className={`${s.mapDot} ${item.tone === "good" ? s.mapGood : item.tone === "warn" ? s.mapWarn : ""}`} aria-hidden="true" /><span>{item.name}<small>{item.pin}</small></span><ChevronRight size={14} /></button>)}</div><div className={s.sourceFoot}>COMPONENTS <ArrowRight size={13} /> LIVE TONE</div></div>
          <div className={s.interpretation}><div className={s.connectorLine} /><span className={s.eyebrow}>{mapConnections[mapSelection].status.toUpperCase()}</span><h3>{mapConnections[mapSelection].name}</h3><p>{mapConnections[mapSelection].detail}</p></div>
        </div>
        <div className={s.sectionFoot}><span>TOPOLOGY → LIVE TONE → SELECTED SIGNAL</span><span>02 / 05</span></div>
      </section>

      <section id="measure" data-chapter="3" className={`${s.chapter} ${s.measure}`}>
        <div className={s.sectionHeading}><p className={s.eyebrow}>03 / MEASURE</p><h2>Follow the wire.<br /><em>Find the signal.</em></h2><p>Expected behavior is a starting point. Measurements tell you what your circuit actually does.</p></div>
        <div className={s.wireWorld} style={{ "--selected-wire": pin } as CSSProperties}>
          <svg className={s.wires} viewBox="0 0 1100 380" role="img" aria-label={`${pins[pin].name} path from controller to sensor`}>
            <defs><pattern id="wire-grid" width="28" height="28" patternUnits="userSpaceOnUse"><circle cx="1" cy="1" r=".65" fill="currentColor" /></pattern></defs>
            <rect width="1100" height="380" fill="url(#wire-grid)" opacity=".2" />
            {[0, 1, 2].map(index => <g key={index} className={pin === index ? s.activeWire : s.inactiveWire}><path className={s.wireBase} d={`M205 ${112 + index * 78} H${330 + index * 55} Q${360 + index * 55} ${112 + index * 78} ${360 + index * 55} ${142 + index * 78} V${272 - index * 78} Q${360 + index * 55} ${302 - index * 78} ${390 + index * 55} ${302 - index * 78} H895`} /><path className={s.wireFlow} d={`M205 ${112 + index * 78} H${330 + index * 55} Q${360 + index * 55} ${112 + index * 78} ${360 + index * 55} ${142 + index * 78} V${272 - index * 78} Q${360 + index * 55} ${302 - index * 78} ${390 + index * 55} ${302 - index * 78} H895`} /> <circle cx="205" cy={112 + index * 78} r="5" /><circle cx="895" cy={302 - index * 78} r="5" /></g>)}
            <g className={s.wireNode}><rect x="30" y="50" width="170" height="280" rx="10" /><text x="115" y="84" textAnchor="middle">CONTROLLER</text>{pins.map((item,index) => <text key={item.pin} x="115" y={118+index*78} textAnchor="middle">{item.pin}</text>)}<rect x="900" y="50" width="170" height="280" rx="10" /><text x="985" y="84" textAnchor="middle">HC-SR04</text>{["ECHO", "TRIG", "VCC"].map((label,index) => <text key={label} x="985" y={152+index*78} textAnchor="middle">{label}</text>)}</g>
          </svg>
          <div className={s.wireControls} role="group" aria-label="Select a signal">{pins.map((item,index) => <button key={item.name} aria-pressed={pin===index} onClick={() => setPin(index)}><span>0{index+1}</span>{item.name}<ArrowUpRight size={14} /></button>)}</div>
          <div className={s.wireExplanation} aria-live="polite"><strong>{pins[pin].name}</strong><p>{pins[pin].description}</p><span>ILLUSTRATIVE WIRING / SIMULATED DEMO AVAILABLE</span></div>
        </div>
        <div className={s.sectionFoot}><span>MEASUREMENT → EXPECTATION → EVIDENCE</span><span>03 / 05</span></div>
      </section>

      <section id="diagnose" data-chapter="4" className={`${s.chapter} ${s.diagnose}`}>
        <div className={s.sectionHeading}><p className={s.eyebrow}>04 / DIAGNOSE</p><h2>A symptom is a clue.<br /><em>Evidence connects it.</em></h2><p>Keep observations and interpretations separate. Understand the possible causes, then choose a test that tells you more.</p></div>
        <div className={s.evidenceWorld}>
          <div className={s.evidenceSwitch}><span>EXPLORE AN EXAMPLE</span><button aria-pressed={fault} onClick={() => setFault(true)}>Missing response</button><button aria-pressed={!fault} onClick={() => setFault(false)}>Response present</button></div>
          <div className={s.evidenceRows}>{["Supply present", "Trigger detected", fault ? "Echo not detected" : "Echo detected"].map((label,index) => <div key={index} className={index===2 && fault ? s.warningRow : ""}><span className={s.evidenceNumber}>0{index+1}</span><span>{label}</span><svg viewBox="0 0 230 36" aria-hidden="true"><path d={index===0 ? "M0 18 H230" : index===2 && fault ? "M0 28 H230" : "M0 28 H35 V7 H55 V28 H100 V7 H120 V28 H175 V7 H195 V28 H230"}/></svg><small>{index===2 && fault ? "INVESTIGATE" : "OBSERVED"}</small>{index===2 && fault ? <Minus size={18} /> : <Check size={18} />}</div>)}</div>
          <div className={s.reasoning} aria-live="polite"><div><span className={s.eyebrow}>WHAT IT COULD MEAN</span><h3>{fault ? "The return path needs a closer look." : "The response path is active."}</h3><p>{fault ? "A wiring issue, signal-level mismatch, or sensor behavior could explain the missing response. The observation alone does not identify the cause." : "An echo was detected in this illustrative case. Check its timing and consistency against the project’s expected behavior before calling it resolved."}</p></div><div className={s.nextTest}><span>NEXT DIAGNOSTIC TEST</span><ArrowUpRight size={24} /><p>{fault ? "Inspect the ECHO connection and capture the signal at the sensor output." : "Repeat the capture and compare response timing with the baseline."}</p></div></div>
        </div>
        <div className={s.sectionFoot}><span>ILLUSTRATIVE EVIDENCE / NOT A LIVE DIAGNOSIS</span><span>04 / 05</span></div>
      </section>

      <section id="verify" data-chapter="5" className={`${s.chapter} ${s.verify}`}>
        <div className={s.sectionHeading}><p className={s.eyebrow}>05 / VERIFY</p><h2>Finding the fault matters.<br /><em>Proving the fix matters more.</em></h2><p>Compare before and after. Keep the measurements, the reasoning, and the result together.</p></div>
        <div className={s.compareWorld}>
          <div className={s.compareLabels}><span>BEFORE / NO RESPONSE</span><span>AFTER / RESPONSE DETECTED</span></div>
          <div className={s.comparePlot}><div className={s.plotGrid}/><svg viewBox="0 0 1000 150" preserveAspectRatio="none" aria-hidden="true"><path className={s.beforeTrace} d="M0 112 H1000" /><path className={s.afterTrace} style={{clipPath:`inset(0 ${100-comparison}% 0 0)`}} d="M0 112 H85 V35 H125 V112 H255 V35 H295 V112 H425 V35 H465 V112 H595 V35 H635 V112 H765 V35 H805 V112 H935 V35 H975 V112 H1000" /></svg><div className={s.compareHandle} style={{left:`${comparison}%`}}><span>↔</span></div></div>
          <label className={s.sliderLabel}>Drag to compare the illustrative captures<input type="range" min="0" max="100" value={comparison} onChange={event => setComparison(Number(event.target.value))} aria-label="Reveal after capture" /></label>
        </div>
        <div className={s.records}>{[
          ["A record of the reasoning", "Session history keeps your measurements and diagnostic context together, so the next investigation starts with evidence."],
          ["A report you can revisit", "Generate a report from the session to review the diagnosis, supporting evidence, and verification results."],
          ["You stay in control", "Simulated and physical hardware states are labeled. PATCH remains locked; the landing-page experience never executes a hardware repair."],
          ["A baseline you can trust", "Save a confirmed capture as Known Good — source-labeled, so a serial reading and a simulated one are never mixed. Later captures compare against it as Healthy, Deviation detected, or Needs verification: a comparison, not a certified score."],
        ].map(([title,body],index) => <div key={title}><button aria-expanded={open===index} aria-controls={`record-${index}`} onClick={() => setOpen(open===index ? null : index)}><span>0{index+1}</span>{title}{open===index ? <Minus size={17}/> : <Plus size={17}/>}</button><p id={`record-${index}`} hidden={open!==index}>{body}</p></div>)}</div>
        <div className={s.sectionFoot}><span>CAPTURE → COMPARE → KEEP THE RECORD</span><span>05 / 05</span></div>
      </section>
      <section className={s.closing}><p className={s.eyebrow}>YOUR NEXT BUILD DESERVES A CLEARER PICTURE</p><h2>Make the connection.</h2><Link href="/projects/demo" className={s.primary}>Experience ReWeird <ArrowUpRight size={18}/></Link><p>Start with the simulated demo. Bring your own project when you’re ready.</p><a href="#hardware" className={s.replay}><RotateCcw size={13}/> Back to the beginning</a></section>
    </main>
    <footer className={s.footer}><Link href="/" className={s.brand}><img src="/images/reweird-logo-mark.png" alt="" width="52" height="26"/><span>ReWeird</span></Link><span>Built for the questions between code and circuit.</span><WorkspaceLink>Open workspace <ArrowRight size={14}/></WorkspaceLink></footer>
  </div>;
}
