#include <Arduino.h>
#include <ArduinoJson.h>

#include "probe_config.h"

struct ProbeAccumulator {
  volatile uint32_t edgeCount = 0;
  volatile uint32_t risingEdges = 0;
  volatile uint32_t fallingEdges = 0;
  volatile uint32_t lastEdgeUS = 0;
  volatile uint32_t lastRiseUS = 0;
  volatile uint8_t lastState = 0;
  volatile uint8_t periodCount = 0;
  volatile uint8_t highCount = 0;
  volatile uint32_t periodsUS[REWEIRD_PULSE_SAMPLES] = {};
  volatile uint32_t highPulseWidthsUS[REWEIRD_PULSE_SAMPLES] = {};
  uint32_t activityCounts[REWEIRD_ACTIVITY_BUCKETS] = {};
  uint32_t bucketRisingStart = 0;
};

struct ProbeSnapshot {
  uint32_t edgeCount = 0;
  uint32_t risingEdges = 0;
  uint32_t fallingEdges = 0;
  uint32_t lastEdgeUS = 0;
  uint8_t state = 0;
  uint8_t periodCount = 0;
  uint8_t highCount = 0;
  uint32_t periodsUS[REWEIRD_PULSE_SAMPLES] = {};
  uint32_t highPulseWidthsUS[REWEIRD_PULSE_SAMPLES] = {};
  uint32_t activityCounts[REWEIRD_ACTIVITY_BUCKETS] = {};
};

static ProbeAccumulator accumulators[REWEIRD_PROBE_COUNT];
static uint16_t analogMV[REWEIRD_PROBE_COUNT][REWEIRD_ANALOG_SAMPLES] = {};
static uint8_t analogCounts[REWEIRD_PROBE_COUNT] = {};
static portMUX_TYPE probeMux = portMUX_INITIALIZER_UNLOCKED;
static uint32_t windowStartedMS = 0;
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

void IRAM_ATTR onProbeEdge(void *argument) {
  const size_t index = reinterpret_cast<size_t>(argument);
  if (index >= REWEIRD_PROBE_COUNT) {
    return;
  }
  const auto &configuration = REWEIRD_PROBES[index];
  auto &accumulator = accumulators[index];
  const uint32_t nowUS = micros();
  const uint8_t state = static_cast<uint8_t>(digitalRead(configuration.pin));

  portENTER_CRITICAL_ISR(&probeMux);
  accumulator.edgeCount++;
  accumulator.lastEdgeUS = nowUS;
  accumulator.lastState = state;
  if (state == HIGH) {
    accumulator.risingEdges++;
    if (accumulator.lastRiseUS != 0 && accumulator.periodCount < REWEIRD_PULSE_SAMPLES) {
      accumulator.periodsUS[accumulator.periodCount++] = nowUS - accumulator.lastRiseUS;
    }
    accumulator.lastRiseUS = nowUS;
  } else {
    accumulator.fallingEdges++;
    if (accumulator.lastRiseUS != 0 && accumulator.highCount < REWEIRD_PULSE_SAMPLES) {
      accumulator.highPulseWidthsUS[accumulator.highCount++] = nowUS - accumulator.lastRiseUS;
    }
  }
  portEXIT_CRITICAL_ISR(&probeMux);
}

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

static ProbeSnapshot snapshotAndReset(size_t index) {
  ProbeSnapshot snapshot;
  auto &accumulator = accumulators[index];
  portENTER_CRITICAL(&probeMux);
  snapshot.edgeCount = accumulator.edgeCount;
  snapshot.risingEdges = accumulator.risingEdges;
  snapshot.fallingEdges = accumulator.fallingEdges;
  snapshot.lastEdgeUS = accumulator.lastEdgeUS;
  snapshot.state = accumulator.lastState;
  snapshot.periodCount = accumulator.periodCount;
  snapshot.highCount = accumulator.highCount;
  for (size_t sample = 0; sample < REWEIRD_PULSE_SAMPLES; ++sample) {
    snapshot.periodsUS[sample] = accumulator.periodsUS[sample];
    snapshot.highPulseWidthsUS[sample] = accumulator.highPulseWidthsUS[sample];
  }
  for (size_t bucket = 0; bucket < REWEIRD_ACTIVITY_BUCKETS; ++bucket) {
    snapshot.activityCounts[bucket] = accumulator.activityCounts[bucket];
    accumulator.activityCounts[bucket] = 0;
  }
  accumulator.edgeCount = 0;
  accumulator.risingEdges = 0;
  accumulator.fallingEdges = 0;
  accumulator.periodCount = 0;
  accumulator.highCount = 0;
  accumulator.bucketRisingStart = 0;
  portEXIT_CRITICAL(&probeMux);
  return snapshot;
}

static void emitTelemetry(uint32_t windowMS) {
  JsonDocument document;
  document["schema_version"] = 2;
  document["device_id"] = deviceID;
  document["profile_id"] = REWEIRD_PROFILE_ID;
  document["captured_at_ms"] = 0;  // No trusted wall clock on the device.
  document["uptime_ms"] = millis();
  document["window_ms"] = windowMS;
  document["sequence"] = sequenceNumber++;
  JsonArray samples = document["samples"].to<JsonArray>();

  for (size_t index = 0; index < REWEIRD_PROBE_COUNT; ++index) {
    const auto &configuration = REWEIRD_PROBES[index];
    JsonObject sample = samples.add<JsonObject>();
    sample["probe"] = configuration.id;
    sample["mode"] = modeName(configuration.mode);

    if (configuration.mode == ReWeirdProbeMode::Analog) {
      JsonArray values = sample["analog_mv"].to<JsonArray>();
      for (size_t value = 0; value < analogCounts[index]; ++value) {
        values.add(analogMV[index][value]);
      }
      analogCounts[index] = 0;
      continue;
    }

    const ProbeSnapshot snapshot = snapshotAndReset(index);
    sample["state"] = snapshot.state;
    sample["edge_count"] = snapshot.edgeCount;
    sample["rising_edges"] = snapshot.risingEdges;
    sample["falling_edges"] = snapshot.fallingEdges;
    sample["max_gap_us"] = snapshot.lastEdgeUS == 0 ? windowMS * 1000UL : micros() - snapshot.lastEdgeUS;

    JsonArray periods = sample["periods_us"].to<JsonArray>();
    for (size_t value = 0; value < snapshot.periodCount; ++value) {
      periods.add(snapshot.periodsUS[value]);
    }
    JsonArray highWidths = sample["high_pulse_widths_us"].to<JsonArray>();
    for (size_t value = 0; value < snapshot.highCount; ++value) {
      highWidths.add(snapshot.highPulseWidthsUS[value]);
    }
    JsonArray activity = sample["activity_counts"].to<JsonArray>();
    for (size_t bucket = 0; bucket < REWEIRD_ACTIVITY_BUCKETS; ++bucket) {
      activity.add(snapshot.activityCounts[bucket]);
    }
  }

  serializeJson(document, Serial);
  Serial.println();
}

void setup() {
  Serial.begin(REWEIRD_SERIAL_BAUD);
  delay(250);

  const uint64_t chipID = ESP.getEfuseMac();
  snprintf(deviceID, sizeof(deviceID), "reweird-%04X%08X",
           static_cast<uint16_t>(chipID >> 32), static_cast<uint32_t>(chipID));

  // PATCH remains electrically passive. Firmware never switches it to OUTPUT.
  pinMode(REWEIRD_PATCH_PIN, INPUT);

  analogReadResolution(12);
  analogSetPinAttenuation(REWEIRD_PROBES[0].pin, ADC_11db);
  for (size_t index = 0; index < REWEIRD_PROBE_COUNT; ++index) {
    const auto &configuration = REWEIRD_PROBES[index];
    pinMode(configuration.pin, INPUT);
    accumulators[index].lastState = static_cast<uint8_t>(digitalRead(configuration.pin));
    if (configuration.mode != ReWeirdProbeMode::Analog) {
      attachInterruptArg(configuration.pin, onProbeEdge,
                         reinterpret_cast<void *>(index), CHANGE);
    }
  }

  windowStartedMS = millis();
  nextAnalogSampleMS = windowStartedMS;
  nextActivityBucketMS = windowStartedMS + REWEIRD_WINDOW_MS / REWEIRD_ACTIVITY_BUCKETS;
}

void loop() {
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
    emitTelemetry(nowMS - windowStartedMS);
    windowStartedMS = nowMS;
    nextAnalogSampleMS = nowMS;
    nextActivityBucketMS = nowMS + REWEIRD_WINDOW_MS / REWEIRD_ACTIVITY_BUCKETS;
    activityBucketIndex = 0;
  }
  delay(1);
}
