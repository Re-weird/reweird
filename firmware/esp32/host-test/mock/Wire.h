#pragma once
#include <cstdint>
#include <cstddef>
struct TwoWire {
  explicit TwoWire(uint8_t index = 0) : bus(index) {}
  bool begin(int sda = -1, int scl = -1, uint32_t frequency = 0); void setClock(uint32_t);
  void setTimeOut(uint16_t);
  void beginTransmission(uint8_t address); size_t write(uint8_t value); uint8_t endTransmission();
  uint8_t address = 0; uint8_t bus; bool initialized = false;
};
extern TwoWire Wire;
