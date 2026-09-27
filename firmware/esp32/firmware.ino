#include <Arduino.h>
#include <Wire.h>
#include <Adafruit_GFX.h>
#include <Adafruit_SSD1306.h>

// ======================================================
// REWIRE V1 - DIRECT MODE
// ======================================================

// ---------------- OLED ----------------

#define SCREEN_WIDTH 128
#define SCREEN_HEIGHT 64

#define OLED_SDA 17
#define OLED_SCL 18

Adafruit_SSD1306 display(
    SCREEN_WIDTH,
    SCREEN_HEIGHT,
    &Wire,
    -1
);

// ---------------- PROBES ----------------

#define P1 8
#define P2 3
#define P3 16
#define P4 21
#define P5 9
#define P6 48

// ---------------- TIMING ----------------

const unsigned long TELEMETRY_INTERVAL_MS = 250;
const unsigned long DISPLAY_INTERVAL_MS = 500;

unsigned long lastTelemetry = 0;
unsigned long lastDisplay = 0;

bool displayPage = false;

// ======================================================
// ANALOG READING
//
// P1 and P5 use your 22k / 22k divider.
//
// Incoming voltage is divided by 2,
// so multiply the measured GPIO voltage by 2.
// ======================================================

float readDividedVoltage(int pin) {
    uint32_t millivolts = analogReadMilliVolts(pin);

    float gpioVoltage = millivolts / 1000.0;

    return gpioVoltage * 2.0;
}

// ======================================================
// PULSE READING
// ======================================================

unsigned long readPulseUS(
    int pin,
    unsigned long timeoutUS
) {
    return pulseIn(pin, HIGH, timeoutUS);
}

// ======================================================
// OLED BOOT SCREEN
// ======================================================

void showBootScreen() {

    display.clearDisplay();
    display.setTextColor(SSD1306_WHITE);

    display.setTextSize(2);
    display.setCursor(18, 8);
    display.println("REWIRE");

    display.setTextSize(1);
    display.setCursor(25, 36);
    display.println("DIRECT MODE");

    display.setCursor(38, 51);
    display.println("READY");

    display.display();
}

// ======================================================
// OLED LIVE PAGE 1
// ======================================================

void showLivePage1(
    float railVoltage,
    unsigned long trigUS,
    unsigned long echoUS
) {

    display.clearDisplay();
    display.setTextColor(SSD1306_WHITE);
    display.setTextSize(1);

    display.setCursor(0, 0);
    display.println("REWIRE | LIVE");

    display.setCursor(0, 17);
    display.print("POWER: ");
    display.print(railVoltage, 2);
    display.println(" V");

    display.setCursor(0, 31);
    display.print("TRIG:  ");

    if (trigUS > 0) {
        display.print(trigUS);
        display.println(" us");
    } else {
        display.println("NONE");
    }

    display.setCursor(0, 45);
    display.print("ECHO:  ");

    if (echoUS > 0) {
        display.print(echoUS);
        display.println(" us");
    } else {
        display.println("MISSING");
    }

    display.display();
}

// ======================================================
// OLED LIVE PAGE 2
// ======================================================

void showLivePage2(
    unsigned long servoUS,
    float zmptVoltage,
    int p6State
) {

    display.clearDisplay();
    display.setTextColor(SSD1306_WHITE);
    display.setTextSize(1);

    display.setCursor(0, 0);
    display.println("REWIRE | LIVE");

    display.setCursor(0, 17);
    display.print("SERVO: ");

    if (servoUS > 0) {
        display.print(servoUS);
        display.println(" us");
    } else {
        display.println("NONE");
    }

    display.setCursor(0, 31);
    display.print("ZMPT:  ");
    display.print(zmptVoltage, 2);
    display.println(" V");

    display.setCursor(0, 45);
    display.print("P6:    ");
    display.println(p6State ? "HIGH" : "LOW");

    display.display();
}

// ======================================================
// SEND JSON TELEMETRY
// ======================================================

void sendTelemetry(
    float p1Voltage,
    unsigned long trigUS,
    unsigned long echoUS,
    unsigned long servoUS,
    float zmptVoltage,
    int p6State
) {

    Serial.print("{");

    Serial.print("\"type\":\"telemetry\",");

    Serial.print("\"timestamp\":");
    Serial.print(millis());
    Serial.print(",");

    // ---------------- P1 ----------------

    Serial.print("\"P1\":{");
    Serial.print("\"name\":\"power_rail\",");
    Serial.print("\"type\":\"voltage\",");
    Serial.print("\"value\":");
    Serial.print(p1Voltage, 3);
    Serial.print("},");

    // ---------------- P2 ----------------

    Serial.print("\"P2\":{");
    Serial.print("\"name\":\"ultrasonic_trig\",");
    Serial.print("\"type\":\"pulse\",");
    Serial.print("\"pulse_us\":");
    Serial.print(trigUS);
    Serial.print(",");
    Serial.print("\"detected\":");
    Serial.print(trigUS > 0 ? "true" : "false");
    Serial.print("},");

    // ---------------- P3 ----------------

    Serial.print("\"P3\":{");
    Serial.print("\"name\":\"ultrasonic_echo\",");
    Serial.print("\"type\":\"pulse\",");
    Serial.print("\"pulse_us\":");
    Serial.print(echoUS);
    Serial.print(",");
    Serial.print("\"detected\":");
    Serial.print(echoUS > 0 ? "true" : "false");
    Serial.print("},");

    // ---------------- P4 ----------------

    Serial.print("\"P4\":{");
    Serial.print("\"name\":\"servo_pwm\",");
    Serial.print("\"type\":\"pwm\",");
    Serial.print("\"pulse_us\":");
    Serial.print(servoUS);
    Serial.print(",");
    Serial.print("\"detected\":");
    Serial.print(servoUS > 0 ? "true" : "false");
    Serial.print("},");

    // ---------------- P5 ----------------

    Serial.print("\"P5\":{");
    Serial.print("\"name\":\"zmpt\",");
    Serial.print("\"type\":\"analog\",");
    Serial.print("\"value\":");
    Serial.print(zmptVoltage, 3);
    Serial.print("},");

    // ---------------- P6 ----------------

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

    // ---------------- INPUTS ----------------

    pinMode(P1, INPUT);
    pinMode(P2, INPUT);
    pinMode(P3, INPUT);
    pinMode(P4, INPUT);
    pinMode(P5, INPUT);
    pinMode(P6, INPUT);

    analogReadResolution(12);

    // ---------------- OLED ----------------

    Wire.begin(OLED_SDA, OLED_SCL);

    if (!display.begin(
            SSD1306_SWITCHCAPVCC,
            0x3C
        )) {

        Serial.println(
            "{\"type\":\"error\","
            "\"message\":\"OLED_INIT_FAILED\"}"
        );

    } else {

        showBootScreen();
    }

    // ---------------- DEVICE STATUS ----------------

    Serial.println(
        "{\"type\":\"device_status\","
        "\"device\":\"rewire\","
        "\"status\":\"ready\","
        "\"mode\":\"direct\","
        "\"firmware\":\"rewire-v1.0\"}"
    );

    delay(1500);
}

// ======================================================
// MAIN LOOP
// ======================================================

void loop() {

    // --------------------------------------------------
    // P1
    // Target circuit 5V rail
    // --------------------------------------------------

    float railVoltage =
        readDividedVoltage(P1);

    // --------------------------------------------------
    // P2
    // HC-SR04 TRIG
    //
    // Expected approximately 10 us
    // --------------------------------------------------

    unsigned long trigUS =
        readPulseUS(P2, 5000);

    // --------------------------------------------------
    // P3
    // HC-SR04 ECHO
    //
    // Width depends on measured distance
    // --------------------------------------------------

    unsigned long echoUS =
        readPulseUS(P3, 30000);

    // --------------------------------------------------
    // P4
    // Servo PWM
    //
    // Expected roughly 500 - 2400 us
    // --------------------------------------------------

    unsigned long servoUS =
        readPulseUS(P4, 25000);

    // --------------------------------------------------
    // P5
    // ZMPT analog output
    // --------------------------------------------------

    float zmptVoltage =
        readDividedVoltage(P5);

    // --------------------------------------------------
    // P6
    // Spare digital probe
    // --------------------------------------------------

    int p6State =
        digitalRead(P6);

    // --------------------------------------------------
    // OLED
    // --------------------------------------------------

    if (
        millis() - lastDisplay
        >= DISPLAY_INTERVAL_MS
    ) {

        lastDisplay = millis();

        displayPage = !displayPage;

        if (displayPage) {

            showLivePage1(
                railVoltage,
                trigUS,
                echoUS
            );

        } else {

            showLivePage2(
                servoUS,
                zmptVoltage,
                p6State
            );
        }
    }

    // --------------------------------------------------
    // JSON SERIAL TELEMETRY
    // --------------------------------------------------

    if (
        millis() - lastTelemetry
        >= TELEMETRY_INTERVAL_MS
    ) {

        lastTelemetry = millis();

        sendTelemetry(
            railVoltage,
            trigUS,
            echoUS,
            servoUS,
            zmptVoltage,
            p6State
        );
    }
}