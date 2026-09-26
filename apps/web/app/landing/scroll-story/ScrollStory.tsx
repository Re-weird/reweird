"use client";

import { useRef } from "react";
import Link from "next/link";
import { ArrowUpRight, Moon, Sun, Waves } from "lucide-react";
import { SignInButton } from "@clerk/nextjs";
import { useTheme } from "@/lib/theme";
import { useUser, CLERK_ENABLED } from "@/lib/clerk";
import { ContinueWithGoogle } from "../ContinueWithGoogle";
import { beats, poseAt, activeBeatIndex } from "./beats";
import { useStoryProgress } from "./useStoryProgress";
import { useReducedMotion } from "./useReducedMotion";
import { Scene } from "./Scene";
import styles from "./scroll-story.module.css";

export function ScrollStory() {
  const containerRef = useRef<HTMLDivElement>(null);
  const { progress, progressRef } = useStoryProgress(containerRef);
  const reducedMotion = useReducedMotion();
  const [theme, toggleTheme] = useTheme();
  const { isSignedIn } = useUser();
  const active = activeBeatIndex(progress);
  const pose = poseAt(progress);

  return (
    <div className={styles.root}>
      <header className={styles.nav}>
        <span className={styles.brand}><Waves size={20} /> Re<span>Weird</span></span>
        <nav className={styles.navLinks}>
          <Link href="/app?mode=demo">Demo</Link>
          {isSignedIn ? (
            <Link href="/app" className={styles.signIn}>Open workspace</Link>
          ) : CLERK_ENABLED ? (
            <SignInButton mode="modal"><button className={styles.navSignInButton} type="button">Sign in</button></SignInButton>
          ) : (
            <Link href="/app" className={styles.signIn}>Sign in</Link>
          )}
          <button
            className={styles.themeToggle}
            onClick={toggleTheme}
            aria-label={`Switch to ${theme === "dark" ? "light" : "dark"} mode`}
            title={`Switch to ${theme === "dark" ? "light" : "dark"} mode`}
          >
            {theme === "dark" ? <Sun size={15} /> : <Moon size={15} />}
          </button>
        </nav>
      </header>

      <div ref={containerRef} className={styles.storyContainer} style={{ height: `${beats.length * 100}vh` }}>
        <div className={styles.sticky}>
          <Scene progressRef={progressRef} reducedMotion={reducedMotion} />
          <div className={styles.textLayer}>
            {beats.map((beat, index) => {
              const isActive = index === active;
              return (
                <div
                  key={beat.id}
                  className={`${styles.beat} ${styles.beatLeft} ${isActive ? styles.beatActive : ""}`}
                  aria-hidden={!isActive}
                >
                  {beat.kicker && <p className={styles.kicker}>{beat.kicker}</p>}
                  <h2 className={styles.heading}>
                    {beat.heading.split("\n").map((line, i) => (
                      <span key={i}>{line}<br /></span>
                    ))}
                  </h2>
                  {beat.body && <p className={styles.body}>{beat.body}</p>}
                </div>
              );
            })}
          </div>
          <div className={styles.ctaLayer} style={{ opacity: pose.cta, pointerEvents: pose.cta > 0.5 ? "auto" : "none" }}>
            <div className={styles.ctas}>
              <Link href="/app?mode=demo" className={styles.primaryCta}>
                Try Live Demo <ArrowUpRight size={16} />
              </Link>
              {isSignedIn ? (
                <Link href="/app" className={styles.secondaryCta}>Open your workspace</Link>
              ) : (
                <ContinueWithGoogle className={styles.secondaryCta} />
              )}
            </div>
            <p className={styles.fineprint}>No hardware required for demo.</p>
          </div>
        </div>
      </div>
    </div>
  );
}
