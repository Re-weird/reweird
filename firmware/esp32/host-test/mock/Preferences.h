#pragma once
#include <cstdint>
class Preferences { public: bool begin(const char*, bool); uint32_t getUInt(const char*, uint32_t); size_t putUInt(const char*, uint32_t); void end(); };
