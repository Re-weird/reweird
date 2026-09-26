# Signal analysis service boundary

The first implementation lives in `apps/api/internal/signalanalysis` so the
hackathon deployment remains simple. It converts validated, bounded telemetry
windows into voltage statistics, transitions, pulse count, frequency, duty cycle,
jitter, dropout events, missing activity, stability, simultaneous failures, and
trusted-baseline deviation.

Only these structured facts proceed to deterministic rules or the future PROBE
adapter. This directory remains the extraction point if analysis later becomes a
separate service.
