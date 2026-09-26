# Diagnostic rules

The MVP orders reasoning from strongest deterministic evidence to weaker
interpretation.

1. **Missing signal:** a profile requires activity but the window contains no
   pulse or transition evidence.
2. **Voltage outside specification:** an average measured voltage falls outside a
   trusted minimum/maximum from the confirmed profile.
3. **Unexpected dropout:** observed pulses fall below the expected rate or a
   required activity bucket is empty.
4. **Power rail instability:** configured rail variation exceeds tolerance or its
   voltage leaves the trusted range.
5. **Simultaneous dropout:** two or more probes fail in the same analysis bucket,
   suggesting a shared power, ground, or harness cause.
6. **Baseline deviation:** a derived value differs from an explicitly trusted
   healthy baseline. UNKNOWN baselines never influence a conclusion.
7. **Movement correlation:** dropout count increases by a meaningful amount and
   ratio relative to the saved pre-test window. This
   test strengthens—but does not mathematically prove—the intermittent-connection
   hypothesis.

The demo begins at rule 5, clears the shared-rail hypothesis with rule 6, gathers
movement evidence with rule 7, then verifies the repair against the baseline.

Raw high-frequency streams are bounded at the transport and converted into
`AnalysisResult`. Any future PROBE model receives structured measurements,
derived facts, specification results, baseline comparisons, deterministic rule
results, and unresolved questions—not the raw stream.
