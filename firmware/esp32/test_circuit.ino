#include <ESP32Servo.h>

Servo myServo;

// Pins
const int SERVO_PIN = 23;
const int TRIG_PIN = 5;
const int ECHO_PIN = 18;
const int ZMPT_PIN = 34;

// Distance range
const int MIN_DISTANCE = 2;
const int MAX_DISTANCE = 40;

void setup() {
  Serial.begin(115200);

  pinMode(TRIG_PIN, OUTPUT);
  pinMode(ECHO_PIN, INPUT);

  myServo.setPeriodHertz(50);
  myServo.attach(SERVO_PIN, 500, 2400);

  analogReadResolution(12);

  Serial.println("Test circuit starting...");
}

float readDistanceCM() {
  digitalWrite(TRIG_PIN, LOW);
  delayMicroseconds(2);

  digitalWrite(TRIG_PIN, HIGH);
  delayMicroseconds(10);

  digitalWrite(TRIG_PIN, LOW);

  long duration = pulseIn(ECHO_PIN, HIGH, 30000);

  if (duration == 0) {
    return -1;
  }

  float distance = duration * 0.0343 / 2.0;

  return distance;
}

void loop() {
  float distance = readDistanceCM();

  if (distance > 0) {
    Serial.print("Distance: ");
    Serial.print(distance);
    Serial.println(" cm");

    // Close = large angle
    // Far = small angle
    int angle = map(
      (int)distance,
      MIN_DISTANCE,
      MAX_DISTANCE,
      180,
      0
    );

    angle = constrain(angle, 0, 180);

    myServo.write(angle);

    Serial.print("Servo angle: ");
    Serial.println(angle);
  }
  else {
    Serial.println("HC-SR04: No echo detected");
  }

  // ZMPT101B raw ADC reading
  int zmptRaw = analogRead(ZMPT_PIN);

  Serial.print("ZMPT raw ADC: ");
  Serial.println(zmptRaw);

  Serial.println("----------------------");

  delay(250);
}