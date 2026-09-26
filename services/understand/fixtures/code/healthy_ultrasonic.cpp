#include <Arduino.h>

#define TRIG_PIN 25
#define ECHO_PIN 26
static constexpr uint8_t POWER_PIN = 34;

void setup() {
  Serial.begin(115200);
  pinMode(POWER_PIN, INPUT);
  pinMode(TRIG_PIN, OUTPUT);
  pinMode(ECHO_PIN, INPUT);
}

void loop() {
  digitalWrite(TRIG_PIN, HIGH);
  delayMicroseconds(10);
  digitalWrite(TRIG_PIN, LOW);

  long duration = pulseIn(ECHO_PIN, HIGH);
  int supplyMilliVolts = analogReadMilliVolts(POWER_PIN);
  Serial.println(duration);
}
