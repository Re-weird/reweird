#pragma once

#include <Arduino.h>

// The profile ID must match the confirmed Project Profile the API is running,
// or the API rejects every frame. Override it at build time without editing
// this file, e.g. PowerShell: $env:REWEIRD_PROFILE_ID = "my-project-id"
#ifndef REWEIRD_PROFILE_ID_OVERRIDE
#define REWEIRD_PROFILE_ID_OVERRIDE ""
#endif
#if defined(CONFIG_IDF_TARGET_ESP32S3)
static_assert(sizeof(REWEIRD_PROFILE_ID_OVERRIDE) > 1,
              "Set REWEIRD_PROFILE_ID to the confirmed physical project ID before building the ESP32-S3 firmware");
#endif
static constexpr const char *REWEIRD_PROFILE_ID =
    sizeof(REWEIRD_PROFILE_ID_OVERRIDE) > 1 ? REWEIRD_PROFILE_ID_OVERRIDE : "ultrasonic-demo";

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

// These are passive input pins. Update the mapping to match the diagnostic PCB,
// then update the confirmed Project Profile on the backend to match.
//
// P1/P5 are analog inputs; P2/P3 collect pulse edges; P4/P6 collect digital
// state and transitions. PATCH is intentionally absent on the S3 board.
#if defined(CONFIG_IDF_TARGET_ESP32S3)
// ESP32-S3 (env:esp32s3). Classic ESP32 pins 25/26/34 do not exist on the S3 and
// 27/32/33 fall inside the S3's SPI flash / PSRAM range, so this map follows the
// S3 probe board wiring used by firmware.ino (REWIRE direct mode).
//
// Avoided on purpose: GPIO19/20 (native USB = COM5), GPIO26-37 (flash / octal
// PSRAM), GPIO43/44 (UART0), GPIO0/45/46 (boot strapping), ADC2 pins for P1.
// Notes: GPIO3 is a JTAG-select strapping pin but is ignored unless that eFuse
// is burned; GPIO48 drives the RGB LED on some DevKitC-1 v1.0 boards.
static constexpr ReWeirdProbeConfig REWEIRD_PROBES[] = {
    {"P1", 8, ReWeirdProbeMode::Analog},    // ADC1_CH7, 5 V rail via 2:1 divider
    {"P2", 3, ReWeirdProbeMode::Pulse},     // HC-SR04 TRIG
    {"P3", 16, ReWeirdProbeMode::Pulse},    // HC-SR04 ECHO via divider
    {"P4", 21, ReWeirdProbeMode::Digital}, // servo PWM edge/period observation
    {"P5", 9, ReWeirdProbeMode::Analog},    // ADC1_CH8, ZMPT OUT (user-confirmed)
    {"P6", 48, ReWeirdProbeMode::Digital}, // spare; leave disconnected
};

// P2 (TRIG) and P3 (ECHO) use the MCPWM capture unit: edge polarity and a
// 12.5 ns timestamp are latched in hardware, so a 10 us trigger pulse is not
// lost to interrupt latency. -1 = GPIO interrupt capture.
#define REWEIRD_HW_CAPTURE 1
static constexpr int8_t REWEIRD_CAPTURE_CHANNEL[] = {-1, 0, 1, -1, -1, -1};

// Final hardware: two independent I2C controllers, no display multiplexer.
#define REWEIRD_HAS_OLED 1
static constexpr uint8_t REWEIRD_OLED_A_SDA = 17;
static constexpr uint8_t REWEIRD_OLED_A_SCL = 18;
static constexpr uint8_t REWEIRD_OLED_B_SDA = 4;
static constexpr uint8_t REWEIRD_OLED_B_SCL = 5;
static constexpr uint8_t REWEIRD_OLED_ADDRESS = 0x3C;

// The S3 probe board has no PATCH line. No PATCH pin is configured at all.
static constexpr bool REWEIRD_HAS_PATCH_PIN = false;
static constexpr uint8_t REWEIRD_PATCH_PIN = 0xFF;
#else
// Classic ESP32 DevKit (env:esp32dev). Unchanged.
static constexpr ReWeirdProbeConfig REWEIRD_PROBES[] = {
    {"P1", 34, ReWeirdProbeMode::Analog},
    {"P2", 25, ReWeirdProbeMode::Pulse},
    {"P3", 26, ReWeirdProbeMode::Pulse},
    {"P4", 27, ReWeirdProbeMode::Digital},
    {"P5", 32, ReWeirdProbeMode::Digital},
    {"P6", 33, ReWeirdProbeMode::Digital},
};

static constexpr int8_t REWEIRD_CAPTURE_CHANNEL[] = {-1, -1, -1, -1, -1, -1};

static constexpr bool REWEIRD_HAS_PATCH_PIN = true;
static constexpr uint8_t REWEIRD_PATCH_PIN = 4;
#endif

static constexpr size_t REWEIRD_PROBE_COUNT =
    sizeof(REWEIRD_PROBES) / sizeof(REWEIRD_PROBES[0]);
static_assert(sizeof(REWEIRD_CAPTURE_CHANNEL) == REWEIRD_PROBE_COUNT,
              "every probe needs a capture channel entry");

static constexpr uint32_t REWEIRD_SERIAL_BAUD = 115200;
static constexpr uint32_t REWEIRD_WINDOW_MS = 1000;
static constexpr size_t REWEIRD_ANALOG_SAMPLES = 32;
static constexpr size_t REWEIRD_PULSE_SAMPLES = 32;
static constexpr size_t REWEIRD_ACTIVITY_BUCKETS = 10;
