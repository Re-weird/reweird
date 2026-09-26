# Diagnostic rules

The MVP orders reasoning from strongest deterministic evidence to weaker
interpretation.

1. **Dead signal:** no activity within an expected measurement window.
2. **Zero voltage:** measured rail or signal is effectively 0 V when nonzero is
   expected.
3. **Missing activity:** expected pulses, frequency, or output transitions are
   absent.
4. **Specification violation:** voltage falls outside a catalog range.
5. **Unexpected dropouts:** signal activity exists but falls below its expected
   continuity or baseline.
6. **Rail comparison:** simultaneous signal and rail failure suggests a shared
   power issue; isolated signal failure does not.
7. **Movement correlation:** a repeatable dropout increase during a guided wiggle
   test strengthens—but does not mathematically prove—the intermittent-connection
   hypothesis.

The demo begins at rule 5, clears the shared-rail hypothesis with rule 6, gathers
movement evidence with rule 7, then verifies the repair against the baseline.

Raw sample streams should not be sent directly to an LLM. Summaries and rule
results form the structured evidence packet.
