#pragma once
#include <stdint.h>

struct ProbeAccumulator {
  uint32_t rawPerUS = 1;
  volatile uint32_t edgeCount = 0;
  volatile uint32_t risingEdges = 0;
  volatile uint32_t fallingEdges = 0;
  volatile uint32_t windowStartUS = 0;
  volatile uint32_t lastEdgeUS = 0;
  volatile bool edgeInWindow = false;
  volatile uint32_t maxGapUS = 0;
  volatile bool riseInWindow = false;
  volatile bool previousComplete = false;
  volatile bool periodPending = false;
  volatile uint32_t previousCompleteRise = 0;
  volatile uint32_t pendingPeriod = 0;
  volatile uint32_t lastRiseUS = 0;     // interrupt capture timebase
  volatile uint32_t lastRiseTicks = 0;  // hardware capture timebase (APB ticks)
  volatile uint8_t lastState = 0;
  volatile uint8_t periodCount = 0;
  volatile uint8_t highCount = 0;
  volatile uint32_t periodsRaw[REWEIRD_PULSE_SAMPLES] = {};
  volatile uint32_t highWidthsRaw[REWEIRD_PULSE_SAMPLES] = {};
  uint32_t activityCounts[REWEIRD_ACTIVITY_BUCKETS] = {};
  uint32_t bucketRisingStart = 0;
};

struct ProbeSnapshot {
  uint32_t edgeCount = 0;
  uint32_t risingEdges = 0;
  uint32_t fallingEdges = 0;
  uint32_t maxGapUS = 0;
  uint8_t state = 0;
  uint8_t periodCount = 0;
  uint8_t highCount = 0;
  uint32_t rawPerUS = 1;
  uint32_t periodsRaw[REWEIRD_PULSE_SAMPLES] = {};
  uint32_t highWidthsRaw[REWEIRD_PULSE_SAMPLES] = {};
  uint32_t activityCounts[REWEIRD_ACTIVITY_BUCKETS] = {};
};

// Called under the same lock as window closure. Timestamps are sampled only
// after acquiring it. Unsigned elapsed times are valid across micros() wrap
// for windows shorter than 2^31 us; stale/reversed timestamps are rejected.
static inline void IRAM_ATTR recordEdge(ProbeAccumulator &a, bool rising, uint32_t nowUS, uint32_t rawTime) {
  const uint32_t elapsed = nowUS - a.windowStartUS;
  if (elapsed >= 0x80000000UL) return;
  const uint32_t reference = a.edgeInWindow ? a.lastEdgeUS : a.windowStartUS;
  const uint32_t gap = nowUS - reference;
  if (gap > elapsed) return;
  if (gap > a.maxGapUS) a.maxGapUS = gap;
  a.edgeInWindow = true;
  a.lastEdgeUS = nowUS;
  a.edgeCount++;
  a.lastState = rising ? 1 : 0;
  if (rising) {
    a.risingEdges++;
    // A repeated rise cannot establish a complete pulse/period.
    if (a.riseInWindow) a.previousComplete = false;
    a.pendingPeriod = rawTime - a.previousCompleteRise;
    a.periodPending = a.previousComplete && a.pendingPeriod > 0 &&
      static_cast<uint64_t>(a.pendingPeriod) <= static_cast<uint64_t>(elapsed) * a.rawPerUS;
    a.lastRiseTicks = rawTime;
    a.lastRiseUS = nowUS;
    a.riseInWindow = true;
  } else {
    a.fallingEdges++;
    const uint32_t width = rawTime - a.lastRiseTicks;
    const bool complete = a.riseInWindow && width > 0 &&
      static_cast<uint64_t>(width) <= static_cast<uint64_t>(elapsed) * a.rawPerUS;
    if (complete) {
      if (a.highCount < REWEIRD_PULSE_SAMPLES) a.highWidthsRaw[a.highCount++] = width;
      // Commit a period only after BOTH of its pulses have completed.
      if (a.periodPending && a.periodCount < REWEIRD_PULSE_SAMPLES)
        a.periodsRaw[a.periodCount++] = a.pendingPeriod;
      a.previousCompleteRise = a.lastRiseTicks;
    }
    a.previousComplete = complete;
    a.riseInWindow = false;
    a.periodPending = false;
  }
}

// Caller holds the shared capture lock and supplies the boundary sampled
// inside that lock. All probes close at this same instant.
static ProbeSnapshot snapshotAndReset(ProbeAccumulator &accumulator, uint32_t windowEndUS) {
  ProbeSnapshot snapshot;
  const uint32_t reference = accumulator.edgeInWindow ? accumulator.lastEdgeUS : accumulator.windowStartUS;
  const uint32_t windowUS = windowEndUS - accumulator.windowStartUS;
  const uint32_t trailingGap = windowEndUS - reference;
  // Defensive window bound; the atomic boundary prevents a future edge here.
  const uint32_t boundedTrailing = trailingGap <= windowUS ? trailingGap : 0;
  const uint32_t boundedGap = accumulator.maxGapUS <= windowUS ? accumulator.maxGapUS : 0;
  snapshot.maxGapUS = boundedTrailing > boundedGap ? boundedTrailing : boundedGap;
  snapshot.edgeCount = accumulator.edgeCount;
  snapshot.risingEdges = accumulator.risingEdges;
  snapshot.fallingEdges = accumulator.fallingEdges;
  snapshot.state = accumulator.lastState;
  snapshot.periodCount = accumulator.periodCount;
  snapshot.highCount = accumulator.highCount;
  snapshot.rawPerUS = accumulator.rawPerUS;
  for (size_t sample = 0; sample < REWEIRD_PULSE_SAMPLES; ++sample) {
    snapshot.periodsRaw[sample] = accumulator.periodsRaw[sample];
    snapshot.highWidthsRaw[sample] = accumulator.highWidthsRaw[sample];
  }
  for (size_t bucket = 0; bucket < REWEIRD_ACTIVITY_BUCKETS; ++bucket) {
    snapshot.activityCounts[bucket] = accumulator.activityCounts[bucket];
    accumulator.activityCounts[bucket] = 0;
  }
  accumulator.edgeCount = 0;
  accumulator.risingEdges = 0;
  accumulator.fallingEdges = 0;
  accumulator.periodCount = 0;
  accumulator.highCount = 0;
  accumulator.bucketRisingStart = 0;
  accumulator.windowStartUS = windowEndUS;
  accumulator.edgeInWindow = false;
  accumulator.maxGapUS = 0;
  accumulator.riseInWindow = false;
  accumulator.previousComplete = false;
  accumulator.periodPending = false;
  return snapshot;
}
