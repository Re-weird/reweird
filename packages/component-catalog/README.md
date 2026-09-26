# Component catalog

Static, human-curated reference data about real hardware components. Every
file here is read-only reference data, never diagnostic output and never a
substitute for measured evidence — `services/probe` and `services/understand`
both treat catalog content as `SPECIFICATION`/reference-only input, not as a
fact about any particular user's board.

## Format

```json
{
  "id": "hc-sr04",
  "name": "HC-SR04 Ultrasonic Distance Sensor",
  "pins": ["VCC", "TRIG", "ECHO", "GND"],
  "supply_voltage": { "typical": 5.0, "unit": "V" },
  "interfaces": ["trigger pulse", "echo pulse"],
  "notes": ["..."],
  "specifications": { "...": "see below" }
}
```

`id`, `name`, `pins`, `supply_voltage`, `interfaces`, `notes` are the
original fields `services/understand` reads for component matching. They are
unchanged by this document.

### `specifications` (Milestone 4)

An optional, additive map from a **pin role name** (the same role strings
used in `ProbeConfiguration.Role` / `StructuredEvidence.role`, e.g. `"TRIG"`,
`"ECHO"`, `"VCC"`) to a deterministic expected-signal specification that
`services/probe/app/specification.py` can evaluate against real evidence:

```json
"ECHO": {
  "pin": "ECHO",
  "signal_type": "digital_pulse",
  "mode": "pulse",
  "required": true,
  "voltage": { "min": 0, "max": 5.5, "nominal": 5.0, "unit": "V" },
  "pulse_width": { "min": 116, "max": 23200, "unit": "us" },
  "source": "..."
}
```

- `mode`: `"analog" | "digital" | "pulse"`.
- `required`: whether this signal must show activity; a required signal with
  a *present but zero* observed measurement deterministically fails
  `missing-signal`. A signal that cannot be evaluated at all (no matching
  observed measurement in the submitted evidence) is `specification-not-evaluable`,
  never treated as a pass or as proof of absence.
- `voltage` / `pulse_width`: optional numeric range checks, `unit` required
  and matched **exactly** against the observed measurement's own unit — no
  implicit conversion. A missing or mismatched unit fails closed to
  `specification-not-evaluable`, never a silent comparison.
- `source`: **required**, free text. Every numeric specification value must
  cite where it came from (a named datasheet/manufacturer document, or an
  explicitly-marked arithmetic derivation from two cited datasheet values).
  A dimension with no reliable citable source is omitted from the file
  entirely rather than estimated.

This block is intentionally additive: it is silently ignored by any Pydantic
model that doesn't already declare it (e.g. `services/understand`'s simpler
`CatalogEntry`), so extending it never breaks an existing reader.

## Ownership

This directory is not owned by any one service. Each consumer (currently
`services/probe`, `services/understand`) loads and validates it read-only
with its own model shaped for its own needs, rather than sharing a loader
package across independently-deployable services. Nothing writes to this
directory at runtime.
