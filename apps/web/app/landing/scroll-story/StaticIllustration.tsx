import styles from "./scroll-story.module.css";

/** Shown when WebGL is unavailable — the page must still work without it. */
export function StaticIllustration() {
  return (
    <div className={styles.staticIllustration} role="img" aria-label="Illustration of a microcontroller board with probe points measuring signal lines">
      <svg viewBox="0 0 320 260" width="100%" height="100%" preserveAspectRatio="xMidYMid meet">
        <rect x="70" y="70" width="180" height="120" rx="10" fill="var(--surface-2)" stroke="var(--line)" strokeWidth="1.5" />
        <rect x="110" y="95" width="70" height="45" rx="4" fill="var(--surface-3)" stroke="var(--line)" />
        {Array.from({ length: 8 }).map((_, i) => (
          <rect key={`l${i}`} x={78} y={80 + i * 12} width="10" height="4" rx="1" fill="var(--muted)" />
        ))}
        {Array.from({ length: 8 }).map((_, i) => (
          <rect key={`r${i}`} x={232} y={80 + i * 12} width="10" height="4" rx="1" fill="var(--muted)" />
        ))}
        <circle cx="205" cy="205" r="5" fill="var(--cyan)" />
        <circle cx="245" cy="160" r="5" fill="var(--green)" />
        <line x1="205" y1="205" x2="245" y2="160" stroke="var(--cyan)" strokeWidth="1.5" strokeDasharray="4 3" />
        <rect x="250" y="140" width="40" height="24" rx="3" fill="var(--surface-2)" stroke="var(--line)" />
      </svg>
    </div>
  );
}
