#pragma once
#include <stdint.h>

// No verified PATCH protection/output stage exists on the current breadboard.
// This is NOT controlled by a server command, environment variable, or probe
// pin inference. Changing this release gate requires a separate hardware review.
namespace ReWeirdPatch {
static constexpr bool PhysicalInterfaceVerified = false;
static constexpr uint32_t MaxDurationMS = 250;
static constexpr uint32_t MaxLeaseMS = 100;
static constexpr uint32_t MaxFrequencyHz = 100;
enum class Mode { Digital, Pulse, PulseTrain };

constexpr bool reservedPin(int pin) {
  return pin < 0 || pin > 48 || pin == 0 || pin == 3 || pin == 8 || pin == 9 ||
    pin == 16 || pin == 17 || pin == 18 || pin == 19 || pin == 20 || pin == 21 ||
    (pin >= 26 && pin <= 37) || pin == 45 || pin == 46 || pin == 48;
}
constexpr bool validLimits(int pin, uint32_t millivolts, uint32_t durationMS,
                           Mode mode, uint32_t frequencyHz, uint32_t dutyPermille) {
  return !reservedPin(pin) && millivolts == 3300 && durationMS > 0 &&
    durationMS <= MaxDurationMS &&
    ((mode == Mode::Digital || mode == Mode::Pulse) ?
      frequencyHz == 0 && dutyPermille == 0 :
      mode == Mode::PulseTrain && frequencyHz >= 1 && frequencyHz <= MaxFrequencyHz &&
      dutyPermille >= 100 && dutyPermille <= 900 && frequencyHz * durationMS >= 1000);
}

// Compile-time contract checks run in every firmware build. No OUTPUT operation
// is present: physical actions remain unsupported, even with otherwise valid
// limits. A future qualified driver must additionally enforce lease expiry,
// one-use boot-scoped arming, output disable acknowledgement and hardware OE.
static_assert(!PhysicalInterfaceVerified, "PATCH physical output is not qualified");
static_assert(validLimits(10, 3300, 10, Mode::Pulse, 0, 0), "model bounds");
static_assert(!validLimits(8, 3300, 10, Mode::Pulse, 0, 0), "protect P1");
static_assert(!validLimits(9, 3300, 10, Mode::Pulse, 0, 0), "protect P5");
static_assert(!validLimits(48, 3300, 10, Mode::Pulse, 0, 0), "protect P6");
static_assert(!validLimits(10, 5000, 10, Mode::Pulse, 0, 0), "reject 5V");
static_assert(!validLimits(10, 3300, 251, Mode::Pulse, 0, 0), "bounded duration");
static_assert(!validLimits(10, 3300, 0, Mode::Pulse, 0, 0), "no indefinite output");
static_assert(!validLimits(10, 3300, 250, Mode::PulseTrain, 101, 500), "bounded frequency");
static_assert(!validLimits(10, 3300, 250, Mode::PulseTrain, 50, 1000), "bounded duty");
}
