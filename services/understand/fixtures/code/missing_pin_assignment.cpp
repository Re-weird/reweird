#include <Arduino.h>

#define TRIG_PIN 25

void setup() {
  pinMode(TRIG_PIN, OUTPUT);
  // ECHO pin is read directly via a raw GPIO number, never named or
  // assigned to a constant - only a bare hardware_api_call fact, no
  // pin_constant/symbol_reference will exist for it.
  pinMode(26, INPUT);
}

void loop() {
  digitalWrite(TRIG_PIN, HIGH);
  long duration = pulseIn(26, HIGH);
}
