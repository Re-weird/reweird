// Host harness: runs the real firmware main.cpp against a simulated clock and
// simulated bench signals, printing the Telemetry v2 frames it emits.
#include <Arduino.h>
#include <Preferences.h>
#include <esp_timer.h>
#include <driver/mcpwm.h>
#include <Wire.h>
#include <Adafruit_SSD1306.h>
#include <vector>
#include <algorithm>
#include <iostream>
#include <cstring>

static uint64_t nowUS = 5000;
static int level[64] = {};
struct Isr { void (*fn)(void*) = nullptr; void* arg = nullptr; } isr[64];
struct Cap { int pin = -1; cap_isr_cb_t cb = nullptr; void* arg = nullptr; } caps[3];
HWCDC Serial; EspClass ESP;
unsigned long micros() { return (unsigned long)(uint32_t)nowUS; }
unsigned long millis() { return (unsigned long)(uint32_t)(nowUS / 1000); }
int64_t esp_timer_get_time() { return (int64_t)nowUS; }
int digitalRead(uint8_t pin) { return level[pin]; }
void pinMode(uint8_t, uint8_t) {}
void analogReadResolution(uint8_t) {}
void analogSetPinAttenuation(uint8_t, adc_attenuation_t) {}
uint32_t analogReadMilliVolts(uint8_t pin) { return pin == 8 ? 2533 : 1650 + (nowUS / 1000) % 7; }
void attachInterruptArg(uint8_t pin, void (*fn)(void*), void* arg, int) { isr[pin] = {fn, arg}; }
void HWCDC::begin(unsigned long) {}
size_t HWCDC::write(uint8_t c) { std::cout << (char)c; return 1; }
size_t HWCDC::write(const uint8_t* b, size_t n) { std::cout.write((const char*)b, n); return n; }
size_t HWCDC::println() { std::cout << "\n"; return 1; }
uint64_t EspClass::getEfuseMac() { return 0x7C4FAD8B2834ULL; }
static uint32_t boots = 0;
bool Preferences::begin(const char*, bool) { return true; }
uint32_t Preferences::getUInt(const char*, uint32_t) { return boots; }
size_t Preferences::putUInt(const char*, uint32_t v) { boots = v; return 4; }
void Preferences::end() {}
esp_err_t mcpwm_gpio_init(mcpwm_unit_t, mcpwm_io_signals_t s, int pin) { caps[s - MCPWM_CAP_0].pin = pin; return 0; }
esp_err_t mcpwm_capture_enable_channel(mcpwm_unit_t, mcpwm_capture_channel_id_t c, const mcpwm_capture_config_t* conf) { caps[c].cb = conf->capture_cb; caps[c].arg = conf->user_data; return 0; }

// --- OLED / FreeRTOS mocks: the task is run once after the capture ends. ---
TwoWire Wire;
bool TwoWire::begin(int sda, int scl, uint32_t frequency) {
  if (frequency != 400000 || (bus == 0 ? (sda != 17 || scl != 18) : (bus != 1 || sda != 4 || scl != 5))) abort();
  initialized = true; return true;
}
void TwoWire::setClock(uint32_t) {}
void TwoWire::setTimeOut(uint16_t) {}
void TwoWire::beginTransmission(uint8_t target) { if (target != 0x3C) abort(); address = target; }
size_t TwoWire::write(uint8_t) { return 1; }
uint8_t TwoWire::endTransmission() {
  return initialized && address == 0x3C ? 0 : 2;
}
Adafruit_SSD1306::Adafruit_SSD1306(int16_t, int16_t, TwoWire* bus, int8_t) : wire(bus) {}
bool Adafruit_SSD1306::begin(uint8_t, uint8_t address, bool reset, bool periphBegin) { if (!wire || !wire->initialized || reset || periphBegin || address != 0x3C) abort(); return true; }
void Adafruit_SSD1306::clearDisplay() { lines.clear(); }
void Adafruit_SSD1306::setTextSize(uint8_t) {}
void Adafruit_SSD1306::setTextColor(uint16_t) {}
void Adafruit_SSD1306::setCursor(int16_t, int16_t) {}
size_t Adafruit_SSD1306::println(const char* text) { lines.push_back(text); return 1; }
void Adafruit_SSD1306::display() { if (!wire || !wire->initialized) abort(); std::cerr << "--- OLED BUS " << unsigned(wire->bus) << " ---\n"; for (auto& line : lines) std::cerr << line << "\n"; }
struct StopTask {};
struct MockQueue { std::vector<unsigned char> queued; bool hasQueued = false; unsigned itemSize; };
static TaskFunction_t oledTaskFn = nullptr;
QueueHandle_t xQueueCreate(unsigned, unsigned itemSize) { auto *queue = new MockQueue; queue->itemSize = itemSize; return queue; }
int xQueueOverwrite(QueueHandle_t handle, const void* item) { auto &q = *static_cast<MockQueue*>(handle); q.queued.assign((const unsigned char*)item, (const unsigned char*)item + q.itemSize); q.hasQueued = true; return pdTRUE; }
int xQueueReceive(QueueHandle_t handle, void* item, uint32_t ticks) { auto &q = *static_cast<MockQueue*>(handle); if (!q.hasQueued) { if (ticks == 0) return pdFALSE; throw StopTask{}; } memcpy(item, q.queued.data(), q.itemSize); q.hasQueued = false; return pdTRUE; }
int xTaskCreatePinnedToCore(TaskFunction_t fn, const char*, uint32_t, void*, unsigned, void*, int core) { if (core != 0) abort(); oledTaskFn = fn; return 1; }
void vTaskDelete(void*) { throw StopTask{}; }

struct Edge { uint64_t t; int pin; int value; };
static std::vector<Edge> schedule;
static void pulseTrain(int pin, uint64_t start, uint64_t period, double width, uint64_t until) {
  for (uint64_t t = start; t < until; t += period) {
    schedule.push_back({t, pin, 1});
    schedule.push_back({t + (uint64_t)width, pin, 0});
  }
}
static void fire(const Edge& e) {
  nowUS = e.t;
  level[e.pin] = e.value;
  for (auto& c : caps) {
    if (c.pin == e.pin && c.cb) {
      cap_event_data_t data{e.value ? MCPWM_POS_EDGE : MCPWM_NEG_EDGE, (uint32_t)(e.t * 80)};
      c.cb(MCPWM_UNIT_0, MCPWM_SELECT_CAP0, &data, c.arg);
      return;
    }
  }
  if (isr[e.pin].fn) isr[e.pin].fn(isr[e.pin].arg);
}
static size_t cursor = 0;
void delay(uint32_t ms) {
  uint64_t target = nowUS + (uint64_t)ms * 1000;
  while (cursor < schedule.size() && schedule[cursor].t <= target) fire(schedule[cursor++]);
  nowUS = target;
}
void setup(); void loop();
int main(int argc, char** argv) {
  uint64_t seconds = argc > 1 ? atoi(argv[1]) : 4;
  uint64_t until = 5000 + seconds * 1'000'000 + 400'000;
  pulseTrain(3, 200'000, 265'000, 10, until);          // P2 TRIG 10 us
  pulseTrain(16, 200'500, 265'000, 12'270, until);     // P3 ECHO
  pulseTrain(21, 7'000, 20'000, 486, until);           // P4 servo
  std::sort(schedule.begin(), schedule.end(), [](const Edge& a, const Edge& b) { return a.t < b.t; });
  setup();
  while (nowUS < 5000 + seconds * 1'000'000 + 300'000) loop();
  // Render the last frame on the simulated OLED (stderr), as the core-0 task would.
  if (oledTaskFn) { try { oledTaskFn(nullptr); } catch (const StopTask&) {} }
}
