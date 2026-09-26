#include <Arduino.h>

#define TRIG_PIN 25
#define ECHO_PIN 26

void setup() {
  pinMode(TRIG_PIN, OUTPUT);
  pinMode(ECHO_PIN, INPUT);
}

void loop() {
  digitalWrite(TRIG_PIN, HIGH);
  long duration = pulseIn(ECHO_PIN, HIGH);
}
