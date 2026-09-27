#include <Arduino.h>
#include <ArduinoJson.h>
#include <Preferences.h>
#include <cstring>
#include <cstdarg>

#include "probe_config.h"
#include "patch_safety.h"

#include "esp_timer.h"
#if defined(ESP_PLATFORM)
#include "patch_runtime.h"
#endif

#if defined(REWEIRD_HW_CAPTURE)
#include "driver/mcpwm.h"
#include "soc/soc.h"
#endif

#if defined(REWEIRD_HAS_OLED)
#include <Adafruit_GFX.h>
#include <Adafruit_SSD1306.h>
#include <Fonts/FreeSansBold12pt7b.h>
#include <Fonts/FreeSansBold18pt7b.h>
#include "oled_boot_logo.h"
#include <Wire.h>
#endif

// Everything below measures within one capture window. A gap, period, or
// HIGH time is only reported when both of its edges fall inside the window
// it is reported in, so no value can span windows.
//
// Interval values are stored as integers in the capture's native unit
// (rawPerUS counts per microsecond) because Xtensa interrupt handlers must
// not use the FPU; they are converted to microseconds when a frame is built.
#include "capture_window.h"

static ProbeAccumulator accumulators[REWEIRD_PROBE_COUNT];
static ProbeSnapshot completedWindows[REWEIRD_PROBE_COUNT];
static uint16_t analogMV[REWEIRD_PROBE_COUNT][REWEIRD_ANALOG_SAMPLES] = {};
static uint8_t analogCounts[REWEIRD_PROBE_COUNT] = {};
static portMUX_TYPE probeMux = portMUX_INITIALIZER_UNLOCKED;
static uint32_t windowStartedMS = 0;
static uint32_t windowStartedUS = 0;
static uint32_t nextAnalogSampleMS = 0;
static uint32_t nextActivityBucketMS = 0;
static size_t activityBucketIndex = 0;
static uint64_t sequenceNumber = 0;
static char deviceID[32] = {};

static const char *modeName(ReWeirdProbeMode mode) {
  switch (mode) {
    case ReWeirdProbeMode::Analog:
      return "analog";
    case ReWeirdProbeMode::Digital:
      return "digital";
    case ReWeirdProbeMode::Pulse:
      return "pulse";
  }
  return "digital";
}

// GPIO interrupt capture (P1-P6 without a hardware capture channel). The
// level is read after the interrupt fires, so pulses shorter than the
// interrupt latency can be misclassified; the backend detects that from the
// rising/falling imbalance and reports the capture as unreliable.
void IRAM_ATTR onProbeEdge(void *argument) {
  const size_t index = reinterpret_cast<size_t>(argument);
  if (index >= REWEIRD_PROBE_COUNT) {
    return;
  }
  auto &accumulator = accumulators[index];
  portENTER_CRITICAL_ISR(&probeMux);
  const uint32_t nowUS = micros();
  const bool rising = digitalRead(REWEIRD_PROBES[index].pin) == HIGH;
  recordEdge(accumulator, rising, nowUS, nowUS);
  portEXIT_CRITICAL_ISR(&probeMux);
}

#if defined(REWEIRD_HW_CAPTURE)
// MCPWM capture: the hardware latches edge polarity and an APB-clock
// timestamp, so widths and periods keep 12.5 ns resolution regardless of
// interrupt latency. If both edges of a pulse arrive before the interrupt is
// serviced, the rising edge is lost; the rising/falling imbalance then makes
// the backend report the capture as unreliable rather than guessing.
static constexpr uint32_t APB_TICKS_PER_US = APB_CLK_FREQ / 1000000;

static bool IRAM_ATTR onCaptureEdge(mcpwm_unit_t, mcpwm_capture_channel_id_t, const cap_event_data_t *event, void *argument) {
  const size_t index = reinterpret_cast<size_t>(argument);
  if (index >= REWEIRD_PROBE_COUNT) {
    return false;
  }
  auto &accumulator = accumulators[index];
  portENTER_CRITICAL_ISR(&probeMux);
  const uint32_t nowUS = static_cast<uint32_t>(esp_timer_get_time());
  const bool rising = event->cap_edge == MCPWM_POS_EDGE;
  recordEdge(accumulator, rising, nowUS, event->cap_value);
  portEXIT_CRITICAL_ISR(&probeMux);
  return false;
}

static bool startHardwareCapture(size_t index, int8_t channel) {
  const mcpwm_io_signals_t signal = channel == 0 ? MCPWM_CAP_0 : channel == 1 ? MCPWM_CAP_1 : MCPWM_CAP_2;
  const mcpwm_capture_channel_id_t captureChannel =
      channel == 0 ? MCPWM_SELECT_CAP0 : channel == 1 ? MCPWM_SELECT_CAP1 : MCPWM_SELECT_CAP2;
  if (mcpwm_gpio_init(MCPWM_UNIT_0, signal, REWEIRD_PROBES[index].pin) != ESP_OK) {
    return false;
  }
  portENTER_CRITICAL(&probeMux);
  accumulators[index].rawPerUS = APB_TICKS_PER_US;
  mcpwm_capture_config_t configuration = {};
  configuration.cap_edge = MCPWM_BOTH_EDGE;
  configuration.cap_prescale = 1;
  configuration.capture_cb = onCaptureEdge;
  configuration.user_data = reinterpret_cast<void *>(index);
  portEXIT_CRITICAL(&probeMux);
  return mcpwm_capture_enable_channel(MCPWM_UNIT_0, captureChannel, &configuration) == ESP_OK;
}
#endif

static void sampleAnalogInputs() {
  for (size_t index = 0; index < REWEIRD_PROBE_COUNT; ++index) {
    if (REWEIRD_PROBES[index].mode != ReWeirdProbeMode::Analog ||
        analogCounts[index] >= REWEIRD_ANALOG_SAMPLES) {
      continue;
    }
    analogMV[index][analogCounts[index]++] =
        static_cast<uint16_t>(analogReadMilliVolts(REWEIRD_PROBES[index].pin));
  }
}

static void closeActivityBucket() {
  if (activityBucketIndex >= REWEIRD_ACTIVITY_BUCKETS) {
    return;
  }
  portENTER_CRITICAL(&probeMux);
  for (size_t index = 0; index < REWEIRD_PROBE_COUNT; ++index) {
    const uint32_t current = accumulators[index].risingEdges;
    accumulators[index].activityCounts[activityBucketIndex] =
        current - accumulators[index].bucketRisingStart;
    accumulators[index].bucketRisingStart = current;
  }
  portEXIT_CRITICAL(&probeMux);
  activityBucketIndex++;
}

#if defined(REWEIRD_HAS_OLED)
// ---------------------------------------------------------------------------
// OLED status display. It shows a copy of the numbers already placed in the
// last Telemetry v2 frame; it never measures. Boot diagnostics are queued for
// the telemetry task to print between complete JSON frames (no interleaving). All
// I2C traffic runs in its own task on core 0 so a slow display cannot delay
// sampling, edge capture, or frame emission on core 1. If the display is
// missing or fails to start, telemetry continues unchanged.
// ---------------------------------------------------------------------------
struct DisplayProbe {
  ReWeirdProbeMode mode = ReWeirdProbeMode::Digital;
  uint32_t risingEdges = 0;
  uint8_t state = 0;
  bool hasWidth = false;
  double widthUS = 0;
  bool hasMillivolts = false;
  double millivolts = 0;
};

struct DisplaySnapshot {
  uint64_t sequence = 0;
  uint32_t windowMS = 0;
  DisplayProbe probes[REWEIRD_PROBE_COUNT];
};

static QueueHandle_t displayQueue = nullptr;
static DisplaySnapshot pendingDisplay;
static TwoWire I2C_A(0);
static TwoWire I2C_B(1);
static Adafruit_SSD1306 liveOneOLED(128, 64, &I2C_A, -1);
static Adafruit_SSD1306 liveTwoOLED(128, 64, &I2C_B, -1);
static bool liveOneReady = false;
static bool liveTwoReady = false;
struct DisplayBootReport { char text[512] = {}; };
static DisplayBootReport displayBootReport;
static size_t displayLogLength = 0;
static QueueHandle_t displayLogQueue = nullptr;

static void displayLog(const char *format, ...) {
  const size_t remaining = sizeof(displayBootReport.text) - displayLogLength;
  if (remaining < 2) return;
  va_list args;
  va_start(args, format);
  const int length = vsnprintf(displayBootReport.text + displayLogLength, remaining, format, args);
  va_end(args);
  if (length > 0) displayLogLength += static_cast<size_t>(length) < remaining ? length : remaining - 1;
}

static void printDisplayBootReport() {
#if defined(REWEIRD_DISPLAY_DIAGNOSTICS) && REWEIRD_DISPLAY_DIAGNOSTICS
  static DisplayBootReport report;
  if (!Serial) return;
  if (displayLogQueue && xQueueReceive(displayLogQueue, &report, 0) == pdTRUE)
    Serial.write(reinterpret_cast<const uint8_t *>(report.text), strlen(report.text));
#endif
}

static bool oledACK(TwoWire &bus) {
  bus.beginTransmission(REWEIRD_OLED_ADDRESS);
  return bus.endTransmission() == 0;
}

static bool beginOLED(Adafruit_SSD1306 &screen, TwoWire &bus) {
  // Verified against installed Adafruit SSD1306 2.5.17:
  // begin(switchvcc, address, reset, periphBegin).
  // Both buses were explicitly initialized; never reinitialize custom pins.
  return oledACK(bus) &&
         screen.begin(SSD1306_SWITCHCAPVCC, REWEIRD_OLED_ADDRESS, false, false) &&
         oledACK(bus);
}

static void formatProbeLine(char *line, size_t size, size_t index, const DisplayProbe &probe) {
  if (index == 5) { snprintf(line, size, "unused"); return; }
  if (probe.mode == ReWeirdProbeMode::Analog) {
    if (probe.hasMillivolts) {
      // Voltage at the ESP32 pin; any divider scale is applied by the backend.
      snprintf(line, size, "%.3fV", probe.millivolts / 1000.0);
    } else {
      snprintf(line, size, "--");
    }
    return;
  }
  if (probe.risingEdges == 0) {
    snprintf(line, size, "0p %s", probe.state ? "HIGH" : "LOW");
  } else if (!probe.hasWidth) {
    snprintf(line, size, "%lup", static_cast<unsigned long>(probe.risingEdges));
  } else if (probe.widthUS >= 1000) {
    snprintf(line, size, "%lup %.1fms", static_cast<unsigned long>(probe.risingEdges), probe.widthUS / 1000.0);
  } else {
    snprintf(line, size, "%lup %.0fus", static_cast<unsigned long>(probe.risingEdges), probe.widthUS);
  }
}


static void drawLiveScreen(Adafruit_SSD1306 &screen, TwoWire &bus,
                           const DisplaySnapshot &snapshot, size_t first, size_t end) {
  if (!oledACK(bus)) return;
  char line[22];
  static const char *labels[] = {"P1 POWER", "P2 TRIG", "P3 ECHO", "P4 SERVO", "P5 ZMPT", "P6 SPARE"};
  screen.clearDisplay();
  screen.setTextWrap(false);
  screen.setFont(nullptr);
  screen.setTextSize(1);
  // A single inverted title bar, three evenly spaced rows, and one footer.
  // All values still come from the completed telemetry window; no resampling.
  screen.fillRect(0, 0, 128, 11, SSD1306_WHITE);
  screen.setTextColor(SSD1306_BLACK);
  screen.setCursor(3, 2);
  screen.print(first == 0 ? "REWEIRD" : "PROBES");
  screen.setCursor(98, 2);
  screen.print("LIVE");
  screen.setTextColor(SSD1306_WHITE);
  for (size_t index = first; index < end; ++index) {
    const int16_t y = 16 + (index - first) * 13;
    screen.setCursor(2, y);
    screen.print(labels[index]);
    formatProbeLine(line, sizeof(line), index, snapshot.probes[index]);
    // Reserve 12 characters for values. Large raw counts remain available in
    // telemetry; use a compact count-only view instead of overlapping labels.
    if (strlen(line) > 12) snprintf(line, sizeof(line), "%lup", static_cast<unsigned long>(snapshot.probes[index].risingEdges));
    screen.setCursor(126 - 6 * strlen(line), y);
    screen.print(line);
  }
  screen.drawFastHLine(0, 53, 128, SSD1306_WHITE);
  screen.setCursor(2, 56);
  if (!liveOneReady || !liveTwoReady)
    screen.print(!liveOneReady ? "OLED A OFFLINE" : "OLED B OFFLINE");
  else
    screen.print(first == 0 ? "V:pin  p:per window" : "SERIAL | PATCH LOCKED");
  screen.display();
}

static void drawDisplay(const DisplaySnapshot &snapshot) {
  // Probe both buses first so the surviving OLED reports the other's loss.
  if (liveOneReady && !oledACK(I2C_A)) liveOneReady = false;
  if (liveTwoReady && !oledACK(I2C_B)) liveTwoReady = false;
  if (liveOneReady) drawLiveScreen(liveOneOLED, I2C_A, snapshot, 0, 3);
  if (liveTwoReady) drawLiveScreen(liveTwoOLED, I2C_B, snapshot, 3, 6);
}

static void startupScreen(Adafruit_SSD1306 &screen, TwoWire &bus, bool logo) {
  if (!oledACK(bus)) return;
  screen.clearDisplay();
  screen.setTextColor(SSD1306_WHITE);
  screen.setTextWrap(false);
  screen.setTextSize(1);
  if (logo) {
    screen.drawBitmap(2, 2, reweirdBootLogo, 124, 60, SSD1306_WHITE);
  } else {
    int16_t x, y;
    uint16_t width, height;
    screen.setFont(&FreeSansBold18pt7b);
    screen.getTextBounds("ReWeird", 0, 0, &x, &y, &width, &height);
    // Fit the larger glyphs to the full usable width instead of dropping
    // down a whole font size when the native wordmark is slightly too wide.
    GFXcanvas1 wordmark(width, height);
    if (wordmark.getBuffer()) {
      wordmark.setFont(&FreeSansBold18pt7b);
      wordmark.setTextColor(1);
      wordmark.setTextWrap(false);
      wordmark.setCursor(-x, -y);
      wordmark.print("ReWeird");
      const uint16_t fittedWidth = 124;
      const uint16_t fittedHeight = height * fittedWidth / width;
      for (uint16_t row = 0; row < fittedHeight; ++row)
        for (uint16_t column = 0; column < fittedWidth; ++column)
          if (wordmark.getPixel(column * width / fittedWidth, row * height / fittedHeight))
            screen.drawPixel(2 + column, (64 - fittedHeight) / 2 + row, SSD1306_WHITE);
    } else {
      screen.setFont(&FreeSansBold12pt7b);
      screen.getTextBounds("ReWeird", 0, 0, &x, &y, &width, &height);
      screen.setCursor((128 - width) / 2 - x, (64 - height) / 2 - y);
      screen.print("ReWeird");
    }
    screen.setFont(nullptr);
  }
  screen.display();
}

static void diagnoseDisplays(bool busAReady, bool busBReady, bool detailed) {
  displayLogLength = 0;
  displayBootReport.text[0] = '\0';
  const bool ackA = busAReady && oledACK(I2C_A);
  const bool ackB = busBReady && oledACK(I2C_B);
  // Do not clear/reset a healthy screen during periodic health checks.
  liveOneReady = ackA && (liveOneReady || beginOLED(liveOneOLED, I2C_A));
  liveTwoReady = ackB && (liveTwoReady || beginOLED(liveTwoOLED, I2C_B));
  if (detailed) {
    displayLog("DISPLAY_WIRE A: BUS=0 SDA=17 SCL=18 BEGIN=%s\n", busAReady ? "OK" : "FAILED");
    displayLog("DISPLAY_WIRE B: BUS=1 SDA=4 SCL=5 BEGIN=%s\n", busBReady ? "OK" : "FAILED");
    displayLog("OLED_A 0x3C: %s INIT=%s\n", ackA ? "ACK" : "NO_ACK", liveOneReady ? "OK" : "OFFLINE");
    displayLog("OLED_B 0x3C: %s INIT=%s\n", ackB ? "ACK" : "NO_ACK", liveTwoReady ? "OK" : "OFFLINE");
  }
  displayLog("DISPLAY_STATUS OLED_A=BUS0:%s OLED_B=BUS1:%s\n",
             liveOneReady ? "OK" : "OFFLINE", liveTwoReady ? "OK" : "OFFLINE");
  if (displayLogQueue) xQueueOverwrite(displayLogQueue, &displayBootReport);
}

static void oledTask(void *) {
  const bool busAReady = I2C_A.begin(REWEIRD_OLED_A_SDA, REWEIRD_OLED_A_SCL, 400000);
  const bool busBReady = I2C_B.begin(REWEIRD_OLED_B_SDA, REWEIRD_OLED_B_SCL, 400000);
  I2C_A.setTimeOut(20);
  I2C_B.setTimeOut(20);
  diagnoseDisplays(busAReady, busBReady, true);
  if (liveOneReady) startupScreen(liveOneOLED, I2C_A, true);
  if (liveTwoReady) startupScreen(liveTwoOLED, I2C_B, false);
  delay(3000); // Display task only; capture and telemetry never wait for OLEDs.
  uint32_t nextDiagnosticMS = 3000;
  bool delayedReport = true;
  for (;;) {
    if (static_cast<int32_t>(millis() - nextDiagnosticMS) >= 0) {
      diagnoseDisplays(busAReady, busBReady, delayedReport);
      delayedReport = false;
      nextDiagnosticMS = millis() + 10000;
    }
    DisplaySnapshot snapshot;
    if (xQueueReceive(displayQueue, &snapshot, portMAX_DELAY) == pdTRUE)
      drawDisplay(snapshot); // Both displays use this SAME completed v2 window.
  }
}

static void startDisplay() {
  displayLogQueue = xQueueCreate(1, sizeof(DisplayBootReport));
  displayQueue = xQueueCreate(1, sizeof(DisplaySnapshot));
  if (displayQueue == nullptr || displayLogQueue == nullptr ||
      xTaskCreatePinnedToCore(oledTask, "reweird-oled", 4096, nullptr, 1, nullptr, 0) != pdTRUE) {
    // setup() has not started telemetry yet; this cannot split a JSON frame.
    displayLogLength = 0;
    displayLog("DISPLAY_STATUS ERROR=TASK_OR_QUEUE_FAILED\n");
    if (displayLogQueue) xQueueOverwrite(displayLogQueue, &displayBootReport);
  }
}

static void publishDisplay(uint64_t sequence, uint32_t windowMS) {
  if (displayQueue == nullptr) {
    return;
  }
  pendingDisplay.sequence = sequence;
  pendingDisplay.windowMS = windowMS;
  xQueueOverwrite(displayQueue, &pendingDisplay);
}
#endif

// Converts a raw interval to microseconds, keeping 0.1 us for hardware
// captures. Runs outside interrupt context.
static double toMicroseconds(uint32_t raw, uint32_t rawPerUS) {
  if (rawPerUS <= 1) {
    return raw;
  }
  return round(static_cast<double>(raw) * 10.0 / rawPerUS) / 10.0;
}

static void emitTelemetry(uint32_t windowMS, uint32_t windowEndUS) {
  JsonDocument document;
  document["schema_version"] = 2;
  document["device_id"] = deviceID;
  document["profile_id"] = REWEIRD_PROFILE_ID;
  // Measurement capability never implies electrical-output capability.
  JsonObject patch = document["patch"].to<JsonObject>();
  patch["capable"] = false; // stock/host builds have no qualified output stage
  patch["state"] = "LOCKED";
  patch["reason"] = "NO_VERIFIED_DEDICATED_OUTPUT_STAGE";
  patch["boot_id"] = static_cast<uint32_t>(sequenceNumber >> 32);
  patch["max_duration_ms"] = 0; // no physical commands are supported
#if defined(ESP_PLATFORM)
  patch["capable"] = PatchRuntime::provisioned && PatchRuntime::timerReady && PatchRuntime::pins.interlock() && strcmp(REWEIRD_PROFILE_ID, PatchProvision::ProfileID)==0;
  patch["state"] = patch["capable"].as<bool>() ? PatchRuntime::stateLabel() : "LOCKED";
  if (patch["capable"].as<bool>()) { patch["max_duration_ms"] = ReWeirdPatch::MaxDurationMS; patch["reason"] = "QUALIFIED_INTERFACE_REQUIRES_HUMAN_APPROVAL"; }
#endif
  document["captured_at_ms"] = 0;  // No trusted wall clock on the device.
  document["uptime_ms"] = millis();
  document["window_ms"] = windowMS;
  const uint64_t frameSequence = sequenceNumber++;
  document["sequence"] = frameSequence;
  JsonArray samples = document["samples"].to<JsonArray>();

  for (size_t index = 0; index < REWEIRD_PROBE_COUNT; ++index) {
    const auto &configuration = REWEIRD_PROBES[index];
    JsonObject sample = samples.add<JsonObject>();
    sample["probe"] = configuration.id;
    sample["mode"] = modeName(configuration.mode);

    if (configuration.mode == ReWeirdProbeMode::Analog) {
      JsonArray values = sample["analog_mv"].to<JsonArray>();
      uint32_t totalMV = 0;
      for (size_t value = 0; value < analogCounts[index]; ++value) {
        values.add(analogMV[index][value]);
        totalMV += analogMV[index][value];
      }
#if defined(REWEIRD_HAS_OLED)
      pendingDisplay.probes[index].mode = configuration.mode;
      pendingDisplay.probes[index].hasMillivolts = analogCounts[index] > 0;
      pendingDisplay.probes[index].millivolts = analogCounts[index] ? static_cast<double>(totalMV) / analogCounts[index] : 0;
#endif
      analogCounts[index] = 0;
      continue;
    }

    const ProbeSnapshot &snapshot = completedWindows[index];
    sample["state"] = snapshot.state;
    sample["edge_count"] = snapshot.edgeCount;
    sample["rising_edges"] = snapshot.risingEdges;
    sample["falling_edges"] = snapshot.fallingEdges;
    sample["max_gap_us"] = snapshot.maxGapUS;

    JsonArray periods = sample["periods_us"].to<JsonArray>();
    for (size_t value = 0; value < snapshot.periodCount; ++value) {
      periods.add(toMicroseconds(snapshot.periodsRaw[value], snapshot.rawPerUS));
    }
    JsonArray highWidths = sample["high_pulse_widths_us"].to<JsonArray>();
    double widthTotal = 0;
    for (size_t value = 0; value < snapshot.highCount; ++value) {
      const double width = toMicroseconds(snapshot.highWidthsRaw[value], snapshot.rawPerUS);
      highWidths.add(width);
      widthTotal += width;
    }
#if defined(REWEIRD_HAS_OLED)
    pendingDisplay.probes[index].mode = configuration.mode;
    pendingDisplay.probes[index].risingEdges = snapshot.risingEdges;
    pendingDisplay.probes[index].state = snapshot.state;
    pendingDisplay.probes[index].hasWidth = snapshot.highCount > 0;
    pendingDisplay.probes[index].widthUS = snapshot.highCount ? widthTotal / snapshot.highCount : 0;
#endif
    JsonArray activity = sample["activity_counts"].to<JsonArray>();
    for (size_t bucket = 0; bucket < REWEIRD_ACTIVITY_BUCKETS; ++bucket) {
      activity.add(snapshot.activityCounts[bucket]);
    }
  }

#if defined(REWEIRD_HAS_OLED)
  printDisplayBootReport(); // boot-only text; JSON schema and measurements unchanged
#endif
  serializeJson(document, Serial);
  Serial.println();
#if defined(REWEIRD_HAS_OLED)
  publishDisplay(frameSequence, windowMS);
#endif
}

// The device has no wall clock, so captured_at_ms is 0 and the sequence is
// the only frame identity. A boot counter kept in flash (one write per boot)
// forms the upper 32 bits, so the sequence keeps increasing across resets
// and reflashes instead of restarting at 0.
static uint64_t firstSequenceForThisBoot() {
  Preferences preferences;
  if (!preferences.begin("reweird", false)) {
    return 0;
  }
  const uint32_t boots = preferences.getUInt("boots", 0) + 1;
  preferences.putUInt("boots", boots);
  preferences.end();
  return static_cast<uint64_t>(boots) << 32;
}

void setup() {
  Serial.begin(REWEIRD_SERIAL_BAUD);
  delay(250);

  const uint64_t chipID = ESP.getEfuseMac();
  snprintf(deviceID, sizeof(deviceID), "reweird-%04X%08X",
           static_cast<uint16_t>(chipID >> 32), static_cast<uint32_t>(chipID));
  sequenceNumber = firstSequenceForThisBoot();
#if defined(ESP_PLATFORM)
  PatchRuntime::begin(deviceID, REWEIRD_PROFILE_ID, static_cast<uint32_t>(sequenceNumber >> 32));
#endif
#if defined(REWEIRD_HAS_OLED)
  startDisplay();
#endif

  // PATCH remains electrically passive. Firmware never switches it to OUTPUT.
  // Targets without a PATCH line (ESP32-S3 board) configure no PATCH pin at all.
  if (REWEIRD_HAS_PATCH_PIN) {
    pinMode(REWEIRD_PATCH_PIN, INPUT);
  }

  analogReadResolution(12);
  for (size_t index = 0; index < REWEIRD_PROBE_COUNT; ++index) {
    const auto &configuration = REWEIRD_PROBES[index];
    pinMode(configuration.pin, INPUT);
    accumulators[index].lastState = static_cast<uint8_t>(digitalRead(configuration.pin));
    if (configuration.mode == ReWeirdProbeMode::Analog) {
      analogSetPinAttenuation(configuration.pin, ADC_11db);
      continue;
    }
#if defined(REWEIRD_HW_CAPTURE)
    if (REWEIRD_CAPTURE_CHANNEL[index] >= 0 && startHardwareCapture(index, REWEIRD_CAPTURE_CHANNEL[index])) {
      continue;
    }
#endif
    attachInterruptArg(configuration.pin, onProbeEdge, reinterpret_cast<void *>(index), CHANGE);
  }

  // Open the first window for every probe at one instant.
  portENTER_CRITICAL(&probeMux);
  windowStartedUS = micros();
  for (size_t index = 0; index < REWEIRD_PROBE_COUNT; ++index) {
    snapshotAndReset(accumulators[index], windowStartedUS);
  }
  portEXIT_CRITICAL(&probeMux);
  windowStartedMS = millis();
  nextAnalogSampleMS = windowStartedMS;
  nextActivityBucketMS = windowStartedMS + REWEIRD_WINDOW_MS / REWEIRD_ACTIVITY_BUCKETS;
}

void loop() {
#if defined(ESP_PLATFORM)
  PatchRuntime::tick();
#endif
  const uint32_t nowMS = millis();
  if (static_cast<int32_t>(nowMS - nextAnalogSampleMS) >= 0) {
    sampleAnalogInputs();
    nextAnalogSampleMS += REWEIRD_WINDOW_MS / REWEIRD_ANALOG_SAMPLES;
  }
  if (static_cast<int32_t>(nowMS - nextActivityBucketMS) >= 0) {
    closeActivityBucket();
    nextActivityBucketMS += REWEIRD_WINDOW_MS / REWEIRD_ACTIVITY_BUCKETS;
  }
  if (nowMS - windowStartedMS >= REWEIRD_WINDOW_MS) {
    while (activityBucketIndex < REWEIRD_ACTIVITY_BUCKETS) {
      closeActivityBucket();
    }
    // window_ms is measured on the same microsecond clock that bounds every
    // gap and pulse, so no reported interval can exceed the window.
    // Close ALL digital probes before JSON/analog serialization. No ISR can
    // append an edge after this boundary to an already-closing window.
    portENTER_CRITICAL(&probeMux);
    const uint32_t windowEndUS = micros();
    for (size_t index = 0; index < REWEIRD_PROBE_COUNT; ++index)
      completedWindows[index] = snapshotAndReset(accumulators[index], windowEndUS);
    portEXIT_CRITICAL(&probeMux);
    emitTelemetry((windowEndUS - windowStartedUS + 999) / 1000, windowEndUS);
    windowStartedUS = windowEndUS;
    windowStartedMS = nowMS;
    nextAnalogSampleMS = nowMS;
    nextActivityBucketMS = nowMS + REWEIRD_WINDOW_MS / REWEIRD_ACTIVITY_BUCKETS;
    activityBucketIndex = 0;
  }
  delay(1);
}
