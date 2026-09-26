# Component catalog

Each JSON file is one conservative catalog entry loaded directly by the Go API.
Entries include aliases, interface and pin roles, known signal types, safe
measurement notes, and source URLs. Catalog facts are labeled `CATALOG`; they do
not become measured evidence and do not confirm a user's wiring.

The initial catalog intentionally stays small: HC-SR04, SG90-style servo, LED,
push button, generic digital input/output, PWM output, I2C device, and UART
device. Exact part variants still require their own datasheet and user review.

## Optional `specifications` extension (services/probe)

`hc-sr04.json` additionally carries an optional, additive `specifications`
block: a per-pin-role, datasheet-sourced electrical specification (voltage
range, pulse-width range, whether a signal is required) that
`services/probe`'s own deterministic evaluator (`app/specification.py`)
compares against submitted evidence. This key is ignored by the Go loader
(`packages/component-catalog/catalog.go` decodes into a fixed struct and
silently drops unknown fields) and is not required on any other entry -
missing it simply means that service's evaluator has nothing to check for
that component yet.
