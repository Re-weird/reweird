# Component catalog

Each JSON file is one conservative catalog entry loaded directly by the Go API.
Entries include aliases, interface and pin roles, known signal types, safe
measurement notes, and source URLs. Catalog facts are labeled `CATALOG`; they do
not become measured evidence and do not confirm a user's wiring.

The initial catalog intentionally stays small: HC-SR04, SG90-style servo, LED,
push button, generic digital input/output, PWM output, I2C device, and UART
device. Exact part variants still require their own datasheet and user review.
