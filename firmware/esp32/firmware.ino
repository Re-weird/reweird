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

unsigned long lastTelemetry = 0;
unsigned long lastDisplay = 0;

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
// DISPLAYS
// ======================================================

void drawInputs(float railVoltage,
                unsigned long trigUS,
                unsigned long echoUS) {
    if (!dispAOk) return;

    dispA.clearDisplay();
    dispA.setTextColor(SSD1306_WHITE);
    dispA.setTextSize(1);

    dispA.setCursor(0, 0);
    dispA.println("REWIRE | INPUTS");

    dispA.setCursor(0, 17);
    dispA.print("POWER ");
    dispA.print(railVoltage, 2);
    dispA.println(" V");

    dispA.setCursor(0, 31);
    dispA.print("TRIG  ");
    if (trigUS > 0) {
        dispA.print(trigUS);
        dispA.println(" us");
    } else {
        dispA.println("NONE");
    }

    dispA.setCursor(0, 45);
    dispA.print("DIST  ");
    if (echoUS > 0) {
        dispA.print(echoUS * 0.0343 / 2.0, 1);
        dispA.println(" cm");
    } else {
        dispA.println("--");
    }

    dispA.display();
}

void drawStatus(unsigned long servoUS,
                float zmptVoltage,
                int p6State) {
    if (!dispBOk) return;

    dispB.clearDisplay();
    dispB.setTextColor(SSD1306_WHITE);
    dispB.setTextSize(1);

    dispB.setCursor(0, 0);
    dispB.println("REWIRE | STATUS");

    dispB.setCursor(0, 17);
    dispB.print("SERVO ");
    if (servoUS > 0) {
        dispB.print(servoUS);
        dispB.println(" us");
    } else {
        dispB.println("NONE");
    }

    dispB.setCursor(0, 31);
    dispB.print("ZMPT  ");
    dispB.print(zmptVoltage, 2);
    dispB.println(" V");

    dispB.setCursor(0, 45);
    dispB.print("P6    ");
    dispB.println(p6State ? "HIGH" : "LOW");

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

    if (dispAOk) {
        dispA.clearDisplay();
        dispA.setTextColor(SSD1306_WHITE);
        dispA.setTextSize(2);
        dispA.setCursor(18, 16);
        dispA.println("REWIRE");
        dispA.setTextSize(1);
        dispA.setCursor(38, 44);
        dispA.println("READY");
        dispA.display();
    }

    if (dispBOk) {
        dispB.clearDisplay();
        dispB.setTextColor(SSD1306_WHITE);
        dispB.setTextSize(1);
        dispB.setCursor(20, 28);
        dispB.println("DIRECT MODE");
        dispB.display();
    }

    Serial.println(
        "{\"type\":\"device_status\","
        "\"device\":\"rewire\","
        "\"status\":\"ready\","
        "\"mode\":\"direct\","
        "\"firmware\":\"rewire-v1.2-dualbus\"}");

    delay(1200);
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
        drawInputs(railVoltage, trigUS, echoUS);
        drawStatus(servoUS, zmptVoltage, p6State);
    }

    if (millis() - lastTelemetry >= TELEMETRY_INTERVAL_MS) {
        lastTelemetry = millis();
        sendTelemetry(railVoltage, trigUS, echoUS,
                      servoUS, zmptVoltage, p6State);
    }
}
