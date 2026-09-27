#pragma once
#include <cstdint>
#include <string>
#include <vector>
#include "Wire.h"
#define SSD1306_SWITCHCAPVCC 0x02
#define SSD1306_WHITE 1
struct Adafruit_SSD1306 {
  Adafruit_SSD1306(int16_t w, int16_t h, TwoWire* wire, int8_t reset);
  bool begin(uint8_t vcs, uint8_t address, bool reset = true, bool periphBegin = true);
  void clearDisplay(); void setTextSize(uint8_t); void setTextColor(uint16_t); void setCursor(int16_t, int16_t);
  size_t println(const char*); void display();
  std::vector<std::string> lines;
  TwoWire *wire = nullptr;
};
