# Signal analysis service boundary

The first implementation lives in `apps/api/internal/signalanalysis` so the
hackathon deployment remains simple. It converts validated, bounded telemetry
windows into voltage statistics, digital state and transitions, pulse count,
frequency, duty cycle, pulse-width statistics, jitter, maximum gap, dropout
events, missing activity, rail stability, simultaneous failures, and
trusted-baseline deviation. The original frame and derived result are persisted
together before diagnosis.

Only these structured facts proceed to deterministic rules and the optional
Python PROBE adapter. This directory remains the extraction point if analysis
later becomes a separate service.
