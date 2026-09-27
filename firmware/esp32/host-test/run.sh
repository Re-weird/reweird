#!/bin/sh
# Compiles the real firmware (src/main.cpp) for the host with mocked Arduino /
# ESP-IDF APIs and simulated bench signals, then prints the emitted frames.
# This checks the window accounting logic only; it is not a hardware test.
set -e
cd "$(dirname "$0")/.."
: "${ARDUINOJSON_SRC:?set ARDUINOJSON_SRC to an ArduinoJson v7 src/ directory}"
g++ -std=gnu++17 -O1 -Wall -Wextra -Ihost-test/mock -I"$ARDUINOJSON_SRC" -Iinclude \
  -DCONFIG_IDF_TARGET_ESP32S3=1 -DARDUINOJSON_USE_LONG_LONG=1 \
  '-DREWEIRD_PROFILE_ID_OVERRIDE="ultrasonic-servo-bench"' \
  src/main.cpp host-test/host_harness.cpp -o /tmp/reweird-firmware-host
/tmp/reweird-firmware-host "${1:-4}"
