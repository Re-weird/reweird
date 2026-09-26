"""Tree-sitter query patterns for Arduino/ESP32-flavored C++.

Every query here extracts a LITERAL syntactic fact only - a name, a number,
a call site. None of them decide what a pin "is" or "does"; that
interpretation is Gemini's job, one layer up, over the facts these queries
produce.
"""

INCLUDE_QUERY = """
(preproc_include path: (_) @path) @include
"""

MACRO_CONSTANT_QUERY = """
(preproc_def name: (identifier) @name value: (preproc_arg) @value) @def
"""

DECLARED_CONSTANT_QUERY = """
(declaration
  declarator: (init_declarator
    declarator: (identifier) @name
    value: (number_literal) @value)) @decl
"""

# Plain function-style calls: pinMode(...), digitalWrite(...), etc.
PLAIN_CALL_QUERY = """
(call_expression
  function: (identifier) @func
  arguments: (argument_list) @args) @call
"""

# Method-style calls: Serial.begin(...), ESP.getEfuseMac(...)
FIELD_CALL_QUERY = """
(call_expression
  function: (field_expression
    argument: (identifier) @object
    field: (field_identifier) @method)
  arguments: (argument_list) @args) @call
"""

HARDWARE_API_NAMES = {
    "pinMode",
    "digitalWrite",
    "digitalRead",
    "analogRead",
    "analogWrite",
    "analogReadMilliVolts",
    "analogReadResolution",
    "analogSetPinAttenuation",
    "attachInterrupt",
    "attachInterruptArg",
    "detachInterrupt",
    "tone",
    "noTone",
    "pulseIn",
    "shiftOut",
    "shiftIn",
}

FIELD_CALL_OBJECT_ALLOWLIST = {"Serial", "ESP"}

PIN_MODE_VALUES = {"INPUT", "OUTPUT", "INPUT_PULLUP", "INPUT_PULLDOWN"}
