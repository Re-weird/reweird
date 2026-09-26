package codeanalysis

import "testing"

func TestArduinoGPIOExtraction(t *testing.T) {
	source := `
#include <Arduino.h>
#define TRIG 5
const int ECHO = 18;
void setup() {
  pinMode(TRIG, OUTPUT);
  pinMode(ECHO, INPUT);
}
void loop() {
  digitalWrite(TRIG, HIGH);
  delayMicroseconds(10);
  pulseIn(ECHO, HIGH);
}`
	result := New().Analyze("distance.ino", source)
	if result.Status != "OK" || len(result.Pins) != 2 {
		t.Fatalf("Analyze() = %#v", result)
	}
	bySymbol := make(map[string]struct {
		gpio                int
		direction, behavior string
	})
	for _, pin := range result.Pins {
		bySymbol[pin.Symbol] = struct {
			gpio                int
			direction, behavior string
		}{pin.GPIO, pin.Direction, pin.Behavior}
	}
	if pin := bySymbol["TRIG"]; pin.gpio != 5 || pin.direction != "output" || pin.behavior != "digital_pulse" {
		t.Fatalf("TRIG = %#v", pin)
	}
	if pin := bySymbol["ECHO"]; pin.gpio != 18 || pin.direction != "input" || pin.behavior != "pulse_input" {
		t.Fatalf("ECHO = %#v", pin)
	}
}

func TestPWMI2CAndMicroPythonExtraction(t *testing.T) {
	cpp := New().Analyze("main.cpp", `const int MOTOR = 4; const int SDA_PIN = 21; const int SCL_PIN = 22; ledcAttachPin(MOTOR, 0); Wire.begin(SDA_PIN, SCL_PIN);`)
	if len(cpp.Pins) != 3 {
		t.Fatalf("C++ pins = %#v", cpp.Pins)
	}
	python := New().Analyze("main.py", "LED = 2\nled = Pin(LED, Pin.OUT)\npwm = PWM(Pin(LED))\n")
	if len(python.Pins) != 1 || python.Pins[0].Behavior != "pwm_output" {
		t.Fatalf("Python result = %#v", python)
	}
}
