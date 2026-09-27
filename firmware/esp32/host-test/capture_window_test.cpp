#include <cassert>
#include <cstdio>
#include <initializer_list>
#define IRAM_ATTR
#define REWEIRD_PULSE_SAMPLES 32
#define REWEIRD_ACTIVITY_BUCKETS 10
#include "capture_window.h"

static void edge(ProbeAccumulator &a, uint32_t offset, bool high) {
  const uint32_t time = a.windowStartUS + offset;
  recordEdge(a, high, time, time * a.rawPerUS);
}
static void check(const ProbeSnapshot &s, uint32_t duration) {
  assert(s.maxGapUS <= duration);
  assert(s.periodCount > 0 && s.highCount > 0);
  for (unsigned i=0;i<s.periodCount;i++) assert(s.periodsRaw[i] == 20000*s.rawPerUS);
  for (unsigned i=0;i<s.highCount;i++) assert(s.highWidthsRaw[i] == 486*s.rawPerUS);
}
int main() {
  for (uint32_t start : {uint32_t(0), uint32_t(0xffff0000)}) {
    for (uint32_t scale : {uint32_t(1),uint32_t(80)}) {
      ProbeAccumulator a; a.windowStartUS=start; a.rawPerUS=scale;
      for (unsigned i=0;i<50;i++) { edge(a,1000+i*20000,true);edge(a,1486+i*20000,false); }
      auto s=snapshotAndReset(a,start+1000000);
      assert(s.risingEdges==50 && s.fallingEdges==50); check(s,1000000);
      // Closing cannot share the pending rise with the next window.
      edge(a,999800,true);
      s=snapshotAndReset(a,start+2000000);
      assert(s.risingEdges==1 && s.highCount==0);
      edge(a,286,false); // previous window's pulse: raw fall, no derived width
      assert(a.highCount==0 && a.fallingEdges==1);
      for(unsigned i=0;i<49;i++){edge(a,19800+i*20000,true);edge(a,20286+i*20000,false);}
      edge(a,999800,true); // unfinished final pulse excluded from timing
      s=snapshotAndReset(a,start+3000000);
      assert(s.risingEdges==50 && s.fallingEdges==50);check(s,1000000);
      // Slightly longer real window: 50 complete pulses plus one final rise.
      for(unsigned i=0;i<50;i++){edge(a,100+i*20000,true);edge(a,586+i*20000,false);}
      edge(a,1000100,true);
      s=snapshotAndReset(a,start+4000200);
      assert(s.risingEdges==51 && s.fallingEdges==50);check(s,1000200);
    }
  }
  ProbeAccumulator a;
  // Edge arriving during slow JSON serialization belongs to NEW window.
  edge(a,999900,true);
  auto before=snapshotAndReset(a,1000000);
  edge(a,440,false);
  assert(before.maxGapUS<=1000000 && before.highCount==0);
  auto after=snapshotAndReset(a,2000000);
  assert(after.maxGapUS<=1000000 && after.highCount==0);
  // Duplicated/misclassified raw edges are preserved but never paired twice.
  edge(a,100,true);edge(a,110,true);edge(a,596,false);edge(a,600,false);
  auto noisy=snapshotAndReset(a,3000000);
  assert(noisy.risingEdges==2 && noisy.fallingEdges==2);
  assert(noisy.highCount==1 && noisy.periodCount==0);
  // Stale ISR timestamp cannot subtract across a reset window.
  recordEdge(a,true,2999999,2999999);
  assert(a.edgeCount==0);
  puts("Capture window tests PASS: PWM, paired edges, boundaries, wrap, bounded gaps");
}
