#pragma once

#include <Arduino.h>

enum class ReWeirdProbeMode : uint8_t {
  Analog,
  Digital,
  Pulse,
};

struct ReWeirdProbeConfig {
  const char *id;
  uint8_t pin;
  ReWeirdProbeMode mode;
};

// These are passive ESP32 input pins. Update the mapping to match the diagnostic
// PCB, then update the confirmed Project Profile on the backend to match.
//
// P1 is ADC input only. P2/P3 collect pulse edges. P4-P6 collect digital state
// and transitions. PATCH is intentionally absent from this configuration.
static constexpr ReWeirdProbeConfig REWEIRD_PROBES[] = {
    {"P1", 34, ReWeirdProbeMode::Analog},
    {"P2", 25, ReWeirdProbeMode::Pulse},
    {"P3", 26, ReWeirdProbeMode::Pulse},
    {"P4", 27, ReWeirdProbeMode::Digital},
    {"P5", 32, ReWeirdProbeMode::Digital},
    {"P6", 33, ReWeirdProbeMode::Digital},
};

static constexpr size_t REWEIRD_PROBE_COUNT =
    sizeof(REWEIRD_PROBES) / sizeof(REWEIRD_PROBES[0]);

static constexpr uint8_t REWEIRD_PATCH_PIN = 4;
static constexpr uint32_t REWEIRD_SERIAL_BAUD = 115200;
static constexpr uint32_t REWEIRD_WINDOW_MS = 1000;
static constexpr size_t REWEIRD_ANALOG_SAMPLES = 32;
static constexpr size_t REWEIRD_PULSE_SAMPLES = 32;
static constexpr size_t REWEIRD_ACTIVITY_BUCKETS = 10;
