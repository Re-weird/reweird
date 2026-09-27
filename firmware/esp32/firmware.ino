#include <Arduino.h>
#include <Wire.h>
#include <Adafruit_GFX.h>
#include <Adafruit_SSD1306.h>

// ======================================================
// REWIRE V1.2 - DIRECT MODE + DUAL I2C BUS
// Two SSD1306 displays, one per hardware I2C peripheral,
// so both can keep the default 0x3C address.
// ======================================================

#define SCREEN_WIDTH 128
#define SCREEN_HEIGHT 64
#define OLED_ADDR 0x3C

#define BUS_A_SDA 17
#define BUS_A_SCL 18
#define BUS_B_SDA 4
#define BUS_B_SCL 5

TwoWire I2C_A = TwoWire(0);
TwoWire I2C_B = TwoWire(1);

Adafruit_SSD1306 dispA(SCREEN_WIDTH, SCREEN_HEIGHT, &I2C_A, -1);
Adafruit_SSD1306 dispB(SCREEN_WIDTH, SCREEN_HEIGHT, &I2C_B, -1);

bool dispAOk = false;
bool dispBOk = false;

// ---------------- PROBES ----------------

#define P1 8
#define P2 3
#define P3 16
#define P4 21
#define P5 9
#define P6 48

// ---------------- TIMING ----------------

const unsigned long TELEMETRY_INTERVAL_MS = 250;
const unsigned long DISPLAY_INTERVAL_MS = 250;
const unsigned long SPLASH_MS = 3000;

unsigned long lastTelemetry = 0;
unsigned long lastDisplay = 0;

// ======================================================
// REWIRE LOGO - 64 x 64 monochrome
// ======================================================

const unsigned char PROGMEM rewireLogo[] = {
  0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00,
  0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00,
  0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00,
  0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00,
  0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00,
  0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00,
  0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00,
  0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00,

  0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00,
  0x00, 0xFF, 0xFF, 0xF8, 0x00, 0x00, 0x00, 0x00,
  0x01, 0xFF, 0xFF, 0xFE, 0x00, 0x00, 0x00, 0x00,
  0x00, 0xFF, 0xFF, 0xFF, 0x00, 0x00, 0x00, 0x00,
  0x00, 0x7F, 0xFF, 0xFF, 0x80, 0x00, 0x00, 0x00,
  0x00, 0x3F, 0xFF, 0xFF, 0xC0, 0x00, 0x00, 0x00,
  0x00, 0x00, 0x00, 0x3F, 0xC0, 0x00, 0x00, 0x00,
  0x00, 0x00, 0x00, 0x0F, 0xE0, 0x00, 0x00, 0x00,

  0x00, 0x00, 0x00, 0x07, 0xE0, 0x00, 0x01, 0xF8,
  0x0E, 0x01, 0xFF, 0x87, 0xE0, 0x00, 0x03, 0xF8,
  0x1F, 0x83, 0xFF, 0xC7, 0xE0, 0x00, 0x07, 0xF0,
  0x3F, 0x81, 0xFF, 0xC7, 0xE0, 0xE0, 0x07, 0xF0,
  0x7B, 0xFF, 0xFF, 0xC7, 0xE1, 0xF0, 0x0F, 0xE0,
  0x71, 0xFF, 0xFF, 0x87, 0xE1, 0xF0, 0x0F, 0xE0,
  0x71, 0xFF, 0xFF, 0x8F, 0xE3, 0xF8, 0x1F, 0xC0,
  0x7F, 0xFF, 0xFF, 0xBF, 0xC7, 0xF8, 0x1F, 0xC0,

  0x3F, 0x81, 0xFF, 0xFF, 0xC7, 0xFC, 0x3F, 0x80,
  0x1F, 0x01, 0xF7, 0xFF, 0x8F, 0xFC, 0x3F, 0x80,
  0x00, 0x01, 0xF3, 0xFF, 0x0F, 0xFE, 0x7F, 0x00,
  0x00, 0x01, 0xF3, 0xFC, 0x1F, 0xFE, 0x7F, 0x00,
  0x00, 0x01, 0xF1, 0xF8, 0x1F, 0xFF, 0x7E, 0x00,
  0x00, 0x01, 0xF1, 0xF8, 0x3F, 0xFF, 0xFE, 0x00,
  0x00, 0x01, 0xF0, 0xFC, 0x3F, 0x3F, 0xFC, 0x00,
  0x00, 0x01, 0xF0, 0xFC, 0x7F, 0x3F, 0xFC, 0x00,

  0x00, 0x01, 0xF0, 0x7E, 0x7E, 0x3F, 0xF8, 0x00,
  0x00, 0x01, 0xF0, 0x7F, 0xFE, 0x1F, 0xF8, 0x00,
  0x00, 0x01, 0xF0, 0x3F, 0xFC, 0x1F, 0xF0, 0x00,
  0x00, 0x01, 0xF0, 0x1F, 0xFC, 0x0F, 0xF0, 0x00,
  0x00, 0x01, 0xF0, 0x0F, 0xF8, 0x07, 0xE0, 0x00,
  0x00, 0x00, 0x00, 0x07, 0xE0, 0x03, 0x80, 0x00,
  0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00,
  0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00,

  0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00,
  0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00,
  0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00,
  0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00,
  0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00,
  0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00,
  0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00,
  0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00,

  0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00,
  0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00,
  0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00,
  0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00,
  0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00,
  0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00,
  0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00,
  0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00,

  0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00,
  0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00,
  0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00,
  0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00,
  0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00,
  0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00,
  0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00,
  0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00
};

// ======================================================
// INPUT HELPERS
// ======================================================

// 22k / 22k divider halves the incoming voltage.
float readDividedVoltage(int pin) {
    uint32_t mv = analogReadMilliVolts(pin);
    return (mv / 1000.0) * 2.0;
}

unsigned long readPulseUS(int pin, unsigned long timeoutUS) {
    return pulseIn(pin, HIGH, timeoutUS);
}

// ======================================================
// BOOT SPLASH
// Logo on the left screen, wordmark on the right. Both
// panels are inverted so every pixel is lit edge to edge.
// ======================================================

void drawSplash() {
    if (dispAOk) {
        dispA.clearDisplay();
        dispA.fillRect(0, 0, SCREEN_WIDTH, SCREEN_HEIGHT, SSD1306_WHITE);

        // Glyph sits in the upper rows of the 64x64 frame, so nudge
        // it down to land centred on the panel.
        dispA.drawBitmap(32, 4, rewireLogo, 64, 64, SSD1306_BLACK);

        dispA.display();
    }

    if (dispBOk) {
        dispB.clearDisplay();
        dispB.fillRect(0, 0, SCREEN_WIDTH, SCREEN_HEIGHT, SSD1306_WHITE);

        // size 3 -> 18px per char, "ReWeird" = 126x24
        dispB.setTextColor(SSD1306_BLACK);
        dispB.setTextSize(3);
        dispB.setCursor(1, 20);
        dispB.print("ReWeird");

        dispB.display();
    }
}

// ======================================================
// SCREEN A - hero distance
// One number, large enough to read across a table.
// ======================================================

void drawHero(unsigned long echoUS, bool beat) {
    if (!dispAOk) return;

    dispA.clearDisplay();
    dispA.setTextColor(SSD1306_WHITE);

    dispA.setTextSize(1);
    dispA.setCursor(40, 3);
    dispA.print("DISTANCE");

    dispA.setCursor(122, 3);
    dispA.print(beat ? "*" : " ");

    char value[12];
    if (echoUS > 0) {
        snprintf(value, sizeof(value), "%.1f", echoUS * 0.0343 / 2.0);
    } else {
        snprintf(value, sizeof(value), "--");
    }

    // size 3 -> 18px per char; centre on the measured width
    int textWidth = strlen(value) * 18;
    dispA.setTextSize(3);
    dispA.setCursor((SCREEN_WIDTH - textWidth) / 2, 24);
    dispA.print(value);

    if (echoUS > 0) {
        dispA.setTextSize(2);
        dispA.setCursor(52, 50);
        dispA.print("cm");
    } else {
        dispA.setTextSize(1);
        dispA.setCursor(34, 54);
        dispA.print("NO ECHO");
    }

    dispA.display();
}

// ======================================================
// SCREEN B - probe panel
// Six fixed-width rows so the columns line up.
// ======================================================

void drawRow(int y, int index, const char *label,
             const char *value, const char *state) {
    char line[24];
    snprintf(line, sizeof(line), "P%d %-5s%7s %s",
             index, label, value, state);
    dispB.setCursor(0, y);
    dispB.print(line);
}

void drawPanel(float railVoltage,
               unsigned long trigUS,
               unsigned long echoUS,
               unsigned long servoUS,
               float zmptVoltage,
               int p6State,
               bool beat) {
    if (!dispBOk) return;

    dispB.clearDisplay();
    dispB.setTextColor(SSD1306_WHITE);
    dispB.setTextSize(1);

    dispB.setCursor(0, 0);
    dispB.print("REWIRE  PROBES");
    dispB.setCursor(122, 0);
    dispB.print(beat ? "*" : " ");

    dispB.drawFastHLine(0, 9, SCREEN_WIDTH, SSD1306_WHITE);

    char value[12];

    snprintf(value, sizeof(value), "%.2fV", railVoltage);
    drawRow(12, 1, "RAIL", value, railVoltage > 4.0 ? "OK" : "--");

    if (trigUS > 0) snprintf(value, sizeof(value), "%luus", trigUS);
    else            snprintf(value, sizeof(value), "none");
    drawRow(20, 2, "TRIG", value, trigUS > 0 ? "OK" : "--");

    if (echoUS > 0) snprintf(value, sizeof(value), "%.0fcm", echoUS * 0.0343 / 2.0);
    else            snprintf(value, sizeof(value), "none");
    drawRow(28, 3, "ECHO", value, echoUS > 0 ? "OK" : "--");

    if (servoUS > 0) snprintf(value, sizeof(value), "%luus", servoUS);
    else             snprintf(value, sizeof(value), "none");
    drawRow(36, 4, "SERVO", value, servoUS > 0 ? "OK" : "--");

    snprintf(value, sizeof(value), "%.2fV", zmptVoltage);
    drawRow(44, 5, "ZMPT", value, zmptVoltage > 0.10 ? "OK" : "--");

    drawRow(52, 6, "SPARE", p6State ? "HIGH" : "LOW", "  ");

    dispB.display();
}

// ======================================================
// JSON TELEMETRY
// ======================================================

void sendTelemetry(float railVoltage,
                   unsigned long trigUS,
                   unsigned long echoUS,
                   unsigned long servoUS,
                   float zmptVoltage,
                   int p6State) {
    Serial.print("{");
    Serial.print("\"type\":\"telemetry\",");

    Serial.print("\"timestamp\":");
    Serial.print(millis());
    Serial.print(",");

    Serial.print("\"P1\":{");
    Serial.print("\"name\":\"power_rail\",");
    Serial.print("\"type\":\"voltage\",");
    Serial.print("\"value\":");
    Serial.print(railVoltage, 3);
    Serial.print("},");

    Serial.print("\"P2\":{");
    Serial.print("\"name\":\"ultrasonic_trig\",");
    Serial.print("\"type\":\"pulse\",");
    Serial.print("\"pulse_us\":");
    Serial.print(trigUS);
    Serial.print(",\"detected\":");
    Serial.print(trigUS > 0 ? "true" : "false");
    Serial.print("},");

    Serial.print("\"P3\":{");
    Serial.print("\"name\":\"ultrasonic_echo\",");
    Serial.print("\"type\":\"pulse\",");
    Serial.print("\"pulse_us\":");
    Serial.print(echoUS);
    Serial.print(",\"distance_cm\":");
    Serial.print(echoUS > 0 ? (echoUS * 0.0343 / 2.0) : -1.0, 1);
    Serial.print(",\"detected\":");
    Serial.print(echoUS > 0 ? "true" : "false");
    Serial.print("},");

    Serial.print("\"P4\":{");
    Serial.print("\"name\":\"servo_pwm\",");
    Serial.print("\"type\":\"pwm\",");
    Serial.print("\"pulse_us\":");
    Serial.print(servoUS);
    Serial.print(",\"detected\":");
    Serial.print(servoUS > 0 ? "true" : "false");
    Serial.print("},");

    Serial.print("\"P5\":{");
    Serial.print("\"name\":\"zmpt\",");
    Serial.print("\"type\":\"analog\",");
    Serial.print("\"value\":");
    Serial.print(zmptVoltage, 3);
    Serial.print("},");

    Serial.print("\"P6\":{");
    Serial.print("\"name\":\"spare\",");
    Serial.print("\"type\":\"digital\",");
    Serial.print("\"value\":");
    Serial.print(p6State);
    Serial.print("}");

    Serial.println("}");
}

// ======================================================
// SETUP
// ======================================================

void setup() {
    Serial.begin(115200);
    delay(500);

    pinMode(P1, INPUT);
    pinMode(P2, INPUT);
    pinMode(P3, INPUT);
    pinMode(P4, INPUT);
    pinMode(P5, INPUT);
    pinMode(P6, INPUT);

    analogReadResolution(12);

    I2C_A.begin(BUS_A_SDA, BUS_A_SCL, 400000);
    I2C_B.begin(BUS_B_SDA, BUS_B_SCL, 400000);

    // periphBegin=false keeps the library from calling Wire.begin()
    // and resetting these buses back to the default pins.
    dispAOk = dispA.begin(SSD1306_SWITCHCAPVCC, OLED_ADDR, false, false);
    dispBOk = dispB.begin(SSD1306_SWITCHCAPVCC, OLED_ADDR, false, false);

    Serial.printf(
        "{\"type\":\"display_status\",\"bus_a\":%s,\"bus_b\":%s}\n",
        dispAOk ? "true" : "false",
        dispBOk ? "true" : "false");

    drawSplash();

    Serial.println(
        "{\"type\":\"device_status\","
        "\"device\":\"rewire\","
        "\"status\":\"ready\","
        "\"mode\":\"direct\","
        "\"firmware\":\"rewire-v1.3-dualbus\"}");

    delay(SPLASH_MS);
}

// ======================================================
// MAIN LOOP
// ======================================================

void loop() {
    float railVoltage = readDividedVoltage(P1);

    // Target fires TRIG every ~250ms, so the window must outlast that.
    unsigned long trigUS = readPulseUS(P2, 400000);

    // ECHO starts as soon as TRIG ends, so TRIG is the sync point.
    unsigned long echoUS = trigUS > 0 ? readPulseUS(P3, 30000) : 0;

    unsigned long servoUS = readPulseUS(P4, 25000);

    float zmptVoltage = readDividedVoltage(P5);
    int p6State = digitalRead(P6);

    if (millis() - lastDisplay >= DISPLAY_INTERVAL_MS) {
        lastDisplay = millis();

        // Toggles every refresh so a frozen screen is obvious.
        static bool beat = false;
        beat = !beat;

        drawHero(echoUS, beat);
        drawPanel(railVoltage, trigUS, echoUS,
                  servoUS, zmptVoltage, p6State, beat);
    }

    if (millis() - lastTelemetry >= TELEMETRY_INTERVAL_MS) {
        lastTelemetry = millis();
        sendTelemetry(railVoltage, trigUS, echoUS,
                      servoUS, zmptVoltage, p6State);
    }
}
