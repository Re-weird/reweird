// ReWeird ESP32-S3 standalone firmware export.
// Arduino-ESP32 2.0.17-compatible core; ArduinoJson 7.4.3;
// Adafruit SSD1306 2.5.17 + GFX 1.12.6 + BusIO.
// Build with ARDUINO_USB_MODE=1 and ARDUINO_USB_CDC_ON_BOOT=1.
// Use as src/main.cpp in an ESP32-S3 PlatformIO project, not alongside another main.cpp.
#ifndef REWEIRD_PROFILE_ID_OVERRIDE
#define REWEIRD_PROFILE_ID_OVERRIDE "ultrasonic-servo-bench-0e43097d23"
#endif
#define ARDUINOJSON_USE_LONG_LONG 1
#define REWEIRD_PATCH_LOCKED 1

#include <Arduino.h>
#include <ArduinoJson.h>
#include <Preferences.h>
#include <cstring>
#include <cstdarg>

// ---- probe_config.h ----

#include <Arduino.h>

// The profile ID must match the confirmed Project Profile the API is running,
// or the API rejects every frame. Override it at build time without editing
// this file, e.g. PowerShell: $env:REWEIRD_PROFILE_ID = "my-project-id"
#ifndef REWEIRD_PROFILE_ID_OVERRIDE
#define REWEIRD_PROFILE_ID_OVERRIDE ""
#endif
#if defined(CONFIG_IDF_TARGET_ESP32S3)
static_assert(sizeof(REWEIRD_PROFILE_ID_OVERRIDE) > 1,
              "Set REWEIRD_PROFILE_ID to the confirmed physical project ID before building the ESP32-S3 firmware");
#endif
static constexpr const char *REWEIRD_PROFILE_ID =
    sizeof(REWEIRD_PROFILE_ID_OVERRIDE) > 1 ? REWEIRD_PROFILE_ID_OVERRIDE : "ultrasonic-demo";

enum class ReWeirdProbeMode : uint8_t {
  Analog,
  Digital,
  Pulse,
};

struct ReWeirdProbeConfig {
  const char *id;
  uint8_t pin;
  ReWeirdProbeMode mode;
};

// These are passive input pins. Update the mapping to match the diagnostic PCB,
// then update the confirmed Project Profile on the backend to match.
//
// P1/P5 are analog inputs; P2/P3 collect pulse edges; P4/P6 collect digital
// state and transitions. PATCH is intentionally absent on the S3 board.
#if defined(CONFIG_IDF_TARGET_ESP32S3)
// ESP32-S3 (env:esp32s3). Classic ESP32 pins 25/26/34 do not exist on the S3 and
// 27/32/33 fall inside the S3's SPI flash / PSRAM range, so this map follows the
// S3 probe board wiring used by firmware.ino (REWIRE direct mode).
//
// Avoided on purpose: GPIO19/20 (native USB = COM5), GPIO26-37 (flash / octal
// PSRAM), GPIO43/44 (UART0), GPIO0/45/46 (boot strapping), ADC2 pins for P1.
// Notes: GPIO3 is a JTAG-select strapping pin but is ignored unless that eFuse
// is burned; GPIO48 drives the RGB LED on some DevKitC-1 v1.0 boards.
static constexpr ReWeirdProbeConfig REWEIRD_PROBES[] = {
    {"P1", 8, ReWeirdProbeMode::Analog},    // ADC1_CH7, 5 V rail via 2:1 divider
    {"P2", 3, ReWeirdProbeMode::Pulse},     // HC-SR04 TRIG
    {"P3", 16, ReWeirdProbeMode::Pulse},    // HC-SR04 ECHO via divider
    {"P4", 21, ReWeirdProbeMode::Digital}, // servo PWM edge/period observation
    {"P5", 9, ReWeirdProbeMode::Analog},    // ADC1_CH8, ZMPT OUT (user-confirmed)
    {"P6", 48, ReWeirdProbeMode::Digital}, // spare; leave disconnected
};

// P2 (TRIG) and P3 (ECHO) use the MCPWM capture unit: edge polarity and a
// 12.5 ns timestamp are latched in hardware, so a 10 us trigger pulse is not
// lost to interrupt latency. -1 = GPIO interrupt capture.
#define REWEIRD_HW_CAPTURE 1
static constexpr int8_t REWEIRD_CAPTURE_CHANNEL[] = {-1, 0, 1, -1, -1, -1};

// Final hardware: two independent I2C controllers, no display multiplexer.
#define REWEIRD_HAS_OLED 1
static constexpr uint8_t REWEIRD_OLED_A_SDA = 17;
static constexpr uint8_t REWEIRD_OLED_A_SCL = 18;
static constexpr uint8_t REWEIRD_OLED_B_SDA = 4;
static constexpr uint8_t REWEIRD_OLED_B_SCL = 5;
static constexpr uint8_t REWEIRD_OLED_ADDRESS = 0x3C;

// The S3 probe board has no PATCH line. No PATCH pin is configured at all.
static constexpr bool REWEIRD_HAS_PATCH_PIN = false;
static constexpr uint8_t REWEIRD_PATCH_PIN = 0xFF;
#else
// Classic ESP32 DevKit (env:esp32dev). Unchanged.
static constexpr ReWeirdProbeConfig REWEIRD_PROBES[] = {
    {"P1", 34, ReWeirdProbeMode::Analog},
    {"P2", 25, ReWeirdProbeMode::Pulse},
    {"P3", 26, ReWeirdProbeMode::Pulse},
    {"P4", 27, ReWeirdProbeMode::Digital},
    {"P5", 32, ReWeirdProbeMode::Digital},
    {"P6", 33, ReWeirdProbeMode::Digital},
};

static constexpr int8_t REWEIRD_CAPTURE_CHANNEL[] = {-1, -1, -1, -1, -1, -1};

static constexpr bool REWEIRD_HAS_PATCH_PIN = true;
static constexpr uint8_t REWEIRD_PATCH_PIN = 4;
#endif

static constexpr size_t REWEIRD_PROBE_COUNT =
    sizeof(REWEIRD_PROBES) / sizeof(REWEIRD_PROBES[0]);
static_assert(sizeof(REWEIRD_CAPTURE_CHANNEL) == REWEIRD_PROBE_COUNT,
              "every probe needs a capture channel entry");

static constexpr uint32_t REWEIRD_SERIAL_BAUD = 115200;
static constexpr uint32_t REWEIRD_WINDOW_MS = 1000;
static constexpr size_t REWEIRD_ANALOG_SAMPLES = 32;
static constexpr size_t REWEIRD_PULSE_SAMPLES = 32;
static constexpr size_t REWEIRD_ACTIVITY_BUCKETS = 10;

// ---- end probe_config.h ----
// ---- patch_safety.h ----

#include <stdint.h>

// No verified PATCH protection/output stage exists on the current breadboard.
// This is NOT controlled by a server command, environment variable, or probe
// pin inference. Changing this release gate requires a separate hardware review.
namespace ReWeirdPatch {
static constexpr uint32_t MaxDurationMS = 250;
static constexpr uint32_t MaxLeaseMS = 100;
static constexpr uint32_t MaxFrequencyHz = 100;
enum class Mode { Digital, Pulse, PulseTrain };

constexpr bool reservedPin(int pin) {
  return pin < 0 || pin > 48 || pin == 0 || pin == 3 || pin == 8 || pin == 9 ||
    pin == 4 || pin == 5 || pin == 16 || pin == 17 || pin == 18 || pin == 19 || pin == 20 || pin == 21 ||
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

// Compile-time model checks run in every build. The stock board remains
// unqualified. patch_runtime.h separately enforces the reviewed provisioning
// record, physical interlock, bounded state machine and dedicated hardware OE.
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

// ---- end patch_safety.h ----

#include "esp_timer.h"
#if defined(ESP_PLATFORM)
// ---- patch_runtime.h ----

// ---- patch_provision.h ----

#if defined(REWEIRD_PATCH_HOST_TEST) && !defined(ESP_PLATFORM)
#error "Host-test provisioning is not supported by this standalone export"
#else
// Hardware qualification record, reviewed with the dedicated protected stage.
// NEVER fill this from an API, environment flag, telemetry or inferred probe.
// The shipped board has no such stage: these values deliberately provision none.
namespace PatchProvision {
static constexpr bool Verified = false;
static constexpr int OutputPin = -1;
static constexpr int EnablePin = -1; // active HIGH; external pull-down mandatory
static constexpr int InterlockPin = -1; // physical jumper, external pull-down
static constexpr const char *QualificationID = "";
static constexpr const char *ProfileID = "";
static constexpr int ProfileRevision = 0;
static constexpr const char *ProbeMapHash = "";
static constexpr const char *TargetNode = ""; // one qualified isolated INPUT
}
#endif

// ---- end patch_provision.h ----
// ---- patch_machine.h ----

#include <stdint.h>
// ---- patch_safety.h ----

// ---- end patch_safety.h ----

// Platform-independent, testable output state machine. All time is monotonic.
// Output.disable MUST deassert external OE first, then put data in input mode.
namespace ReWeirdPatch {
struct Output {
 virtual ~Output() = default;
 virtual bool interlock() = 0;
 virtual void disable() = 0;
 virtual void drive(bool high) = 0;
};
class Machine {
 Output &io;
 bool provisioned;
 uint64_t lease=0, end=0;
 uint64_t approvalDeadline=UINT64_MAX;
 uint32_t duration=0;
 bool high=false;
public:
 enum State { Locked, Ready, Armed, Active, Disabled };
 State state=Locked;
 bool completed=false;
 explicit Machine(Output &output,bool qualified):io(output),provisioned(qualified){io.disable();state=qualified?Ready:Locked;}
 void stop(){io.disable();state=provisioned&&io.interlock()?Disabled:Locked;lease=end=0;completed=false;}
 bool arm(uint64_t now,uint32_t ms,bool level,uint64_t approvedUntil=UINT64_MAX){
  if(!provisioned||!io.interlock()||state==Armed||state==Active||ms<1||ms>MaxDurationMS||now+ms>approvedUntil){stop();return false;}
  io.disable();completed=false;duration=ms;high=level;approvalDeadline=approvedUntil;lease=now+MaxLeaseMS;state=Armed;return true;
 }
 bool execute(uint64_t now){
  tick(now,true);if(state!=Armed||now+duration>approvalDeadline){stop();return false;}
  end=now+duration;state=Active;io.drive(high);return true;
 }
 void tick(uint64_t now,bool connected){
  if(!provisioned||!io.interlock()||!connected){stop();return;}
  if(state==Locked){state=Ready;completed=false;} // readiness is never arming/output
  if((state==Armed||state==Active)&&now>=lease){stop();return;}
  if(state==Active && now>=end){stop();completed=true;}
 }
 bool heartbeat(uint64_t now){tick(now,true);if(state!=Active)return false;lease=now+MaxLeaseMS;return true;}
 const char *label()const{switch(state){case Ready:return "READY";case Armed:return "ARMED";case Active:return "ACTIVE";case Disabled:return "DISABLED";default:return "LOCKED";}}
};
}

// ---- end patch_machine.h ----
#include <esp_system.h>
#include <driver/gpio.h>
#include <mbedtls/sha256.h>

namespace PatchRuntime {
using namespace PatchProvision;
#if defined(CONFIG_IDF_TARGET_ESP32S3)
constexpr bool boardSupported = true;
#else
constexpr bool boardSupported = false;
#endif
constexpr bool provisioned = boardSupported && Verified && !ReWeirdPatch::reservedPin(OutputPin) &&
 !ReWeirdPatch::reservedPin(EnablePin) && !ReWeirdPatch::reservedPin(InterlockPin) &&
 OutputPin!=EnablePin && OutputPin!=InterlockPin && EnablePin!=InterlockPin &&
 QualificationID[0] && ProfileID[0] && ProfileRevision>0 && ProbeMapHash[0] && TargetNode[0];
static_assert(!Verified || provisioned,"Incomplete/unsafe PATCH qualification record");
struct Pins:ReWeirdPatch::Output {
 bool interlock()override{return provisioned && gpio_get_level((gpio_num_t)InterlockPin)==1;}
 void disable()override{if(provisioned){gpio_set_level((gpio_num_t)EnablePin,0);gpio_set_direction((gpio_num_t)OutputPin,GPIO_MODE_INPUT);}}
 void drive(bool high)override{if(provisioned){gpio_set_level((gpio_num_t)OutputPin,high?1:0);gpio_set_direction((gpio_num_t)OutputPin,GPIO_MODE_OUTPUT);gpio_set_level((gpio_num_t)EnablePin,1);}}
};
static Pins pins;
static ReWeirdPatch::Machine *machine=nullptr;
static portMUX_TYPE mux=portMUX_INITIALIZER_UNLOCKED;
static char challenge[33],device[65],boot[24],profile[65],actionID[65],digest[65];
static char line[4096];static size_t used=0;static bool overflow=false;
static bool consumed=false;static bool timerReady=false;
static int64_t epoch=0;static uint64_t helloAt=0;
static uint64_t now(){return (uint64_t)esp_timer_get_time()/1000;}
static char seenIDs[64][33];static uint64_t seenAt[64];
static bool claimID(const char *id){
 int slot=-1;for(int i=0;i<64;i++){
  if(seenIDs[i][0] && now()-seenAt[i]<=60000 && !strcmp(id,seenIDs[i]))return false;
  if(!seenIDs[i][0] || now()-seenAt[i]>60000)slot=i;
 }
 if(slot<0)return false;strcpy(seenIDs[slot],id);seenAt[slot]=now();return true;
}
static void watchdog(void*){portENTER_CRITICAL(&mux);if(machine)machine->tick(now(),true);portEXIT_CRITICAL(&mux);}
static void stop(){portENTER_CRITICAL(&mux);if(machine)machine->stop();portEXIT_CRITICAL(&mux);}
static const char *stateLabel(){portENTER_CRITICAL(&mux);const char *s=machine?machine->label():"LOCKED";portEXIT_CRITICAL(&mux);return s;}
static void begin(const char *deviceID,const char *profileID,uint32_t bootID){
 snprintf(device,sizeof(device),"%s",deviceID);snprintf(profile,sizeof(profile),"%s",profileID);snprintf(boot,sizeof(boot),"%lu",(unsigned long)bootID);
 for(int i=0;i<4;i++)snprintf(challenge+i*8,9,"%08lx",(unsigned long)esp_random());
 if(provisioned){gpio_set_level((gpio_num_t)EnablePin,0);gpio_set_direction((gpio_num_t)EnablePin,GPIO_MODE_OUTPUT);gpio_set_direction((gpio_num_t)InterlockPin,GPIO_MODE_INPUT);pins.disable();}
 static ReWeirdPatch::Machine instance(pins,provisioned);machine=&instance;
 if(!provisioned)return; // no 1 kHz safety task needed on measurement-only boards
 esp_timer_create_args_t args={};args.callback=watchdog;args.name="patch-deadline";
 esp_timer_handle_t timer=nullptr;
 timerReady=esp_timer_create(&args,&timer)==ESP_OK && esp_timer_start_periodic(timer,1000)==ESP_OK;
 if(!timerReady)stop();
}
static bool equal(JsonVariantConst v,const char *s){return v.is<const char*>() && strcmp(v.as<const char*>(),s)==0;}
static bool boundDigest(JsonObjectConst p,const char *expected){
 // Backend sends Parameters in its canonical struct field order. Re-serializing
 // rejects any changed parameter, not just a changed action ID. Unsupported
 // string encodings fail closed; provisioning identifiers should be ASCII.
 char canonical[2048];size_t length=measureJson(p);if(length>=sizeof(canonical))return false;
 serializeJson(p,canonical,sizeof(canonical));unsigned char hash[32];char hex[65];
 if(mbedtls_sha256_ret((const unsigned char*)canonical,length,hash,0)!=0)return false;
 for(int i=0;i<32;i++)snprintf(hex+i*2,3,"%02x",hash[i]);return strcmp(hex,expected)==0;
}
static void reply(JsonDocument &cmd,bool ok,const char *error){
 JsonDocument out;out["type"]="patch_status";out["request_id"]=cmd["request_id"]|"";out["ok"]=ok;out["error"]=error;
 portENTER_CRITICAL(&mux);const char *state=machine->label();bool completed=machine->completed;portEXIT_CRITICAL(&mux);
 out["state"]=state;out["challenge"]=challenge;out["device_id"]=device;out["boot_id"]=boot;out["profile_id"]=profile;
 out["profile_revision"]=ProfileRevision;out["probe_map_hash"]=ProbeMapHash;out["qualification_id"]=provisioned?QualificationID:"";
 out["pin"]=OutputPin;out["target_node"]=TargetNode;out["action_id"]=actionID;out["digest"]=digest;
 out["uptime_ms"]=millis();
 out["completed"]=completed;
 serializeJson(out,Serial);Serial.println();
}
static void command(){
 JsonDocument cmd;auto err=deserializeJson(cmd,line);
 if(err || !cmd.is<JsonObject>() || !equal(cmd["type"],"patch_command") || !cmd["request_id"].is<const char*>() || strlen(cmd["request_id"])!=32){stop();return;}
 bool valid=true;for(JsonPair pair:cmd.as<JsonObject>()){const char*k=pair.key().c_str();if(strcmp(k,"type")&&strcmp(k,"request_id")&&strcmp(k,"op")&&strcmp(k,"challenge")&&strcmp(k,"action")&&strcmp(k,"action_id")&&strcmp(k,"digest")&&strcmp(k,"now_ms"))valid=false;}
 const char *op=cmd["op"]|"";
 if(!valid){stop();reply(cmd,false,"malformed command");return;}
 if(!strcmp(op,"disable")){stop();reply(cmd,true,"");return;}
 if(!strcmp(op,"status")){reply(cmd,true,"");return;}
 if(!strcmp(op,"hello")){
  stop();if(!cmd["now_ms"].is<int64_t>() || cmd["now_ms"].as<int64_t>()<=0){reply(cmd,false,"clock required");return;}
  epoch=cmd["now_ms"];helloAt=now();consumed=false;actionID[0]=digest[0]=0;
  for(int i=0;i<4;i++)snprintf(challenge+i*8,9,"%08lx",(unsigned long)esp_random());
  reply(cmd,true,"");return;
 }
 if(!timerReady||!provisioned||!pins.interlock()||strcmp(profile,ProfileID)||!equal(cmd["challenge"],challenge)){
  stop();reply(cmd,false,"hardware qualification/interlock/profile/session missing");return;
 }
 if(!strcmp(op,"arm")){
  JsonObjectConst a=cmd["action"].as<JsonObjectConst>();JsonObjectConst p=a["parameters"].as<JsonObjectConst>();
  const char *id=a["id"]|"";const char *hash=a["digest"]|"";
  // Only one action per boot/session challenge. Reboot is never a retry: boot
  // identity and unpredictable challenge invalidate all previous approvals.
  valid=!consumed && strlen(id)==32 && strlen(hash)==64 && boundDigest(p,hash) && equal(cmd["action_id"],id) && equal(cmd["digest"],hash) &&
   equal(a["approval"]["digest"],hash) && a["approval"]["actor"].is<const char*>() && strlen(a["approval"]["actor"])>0 &&
   epoch>0 && p["expires_at_ms"].is<int64_t>() && p["expires_at_ms"].as<int64_t>()>=epoch+(int64_t)(now()-helloAt)+p["duration_ms"].as<int>() &&
   p["expires_at_ms"].as<int64_t>()<=epoch+(int64_t)(now()-helloAt)+60000 &&
   a["approval"]["expires_at_ms"].is<int64_t>() && a["approval"]["expires_at_ms"].as<int64_t>()>=epoch+(int64_t)(now()-helloAt)+p["duration_ms"].as<int>() &&
   a["approval"]["expires_at_ms"].as<int64_t>()<=epoch+(int64_t)(now()-helloAt)+15000 &&
   p.size()==15 && equal(p["device_id"],device)&&equal(p["boot_id"],boot)&&equal(p["profile_id"],ProfileID)&&
   p["profile_revision"].is<int>()&&p["profile_revision"].as<int>()==ProfileRevision&&equal(p["probe_map_hash"],ProbeMapHash)&&
   equal(p["target_node"],TargetNode)&&p["patch_pin"].is<int>()&&p["patch_pin"].as<int>()==OutputPin&&equal(p["source"],"REAL_SERIAL")&&
   (equal(p["mode"],"PULSE")||equal(p["mode"],"DIGITAL"))&&(equal(p["logic_level"],"HIGH")||equal(p["logic_level"],"LOW"))&&
   p["max_voltage"].is<float>()&&p["max_voltage"].as<float>()==3.3f&&p["frequency_hz"].is<float>()&&p["frequency_hz"].as<float>()==0&&
   p["duty_cycle"].is<float>()&&p["duty_cycle"].as<float>()==0&&p["duration_ms"].is<unsigned>()&&p["duration_ms"].as<unsigned>()>=1&&p["duration_ms"].as<unsigned>()<=250;
  if(valid)valid=claimID(id);
  if(valid){
   consumed=true;strcpy(actionID,id);strcpy(digest,hash);
   int64_t expiry=p["expires_at_ms"],approvalExpiry=a["approval"]["expires_at_ms"];
   const uint64_t approvedUntil=helloAt+(uint64_t)((expiry<approvalExpiry?expiry:approvalExpiry)-epoch);
   portENTER_CRITICAL(&mux);valid=machine->arm(now(),p["duration_ms"],equal(p["logic_level"],"HIGH"),approvedUntil);portEXIT_CRITICAL(&mux);
  }
 }else if(!strcmp(op,"execute")||!strcmp(op,"poll")){
  valid=equal(cmd["action_id"],actionID)&&equal(cmd["digest"],digest)&&actionID[0];
  if(valid){portENTER_CRITICAL(&mux);if(!strcmp(op,"execute"))valid=machine->execute(now());else{machine->tick(now(),true);if(machine->state==ReWeirdPatch::Machine::Active)machine->heartbeat(now());}portEXIT_CRITICAL(&mux);}
 }else valid=false;
 if(!valid)stop();reply(cmd,valid,valid?"":"unsafe, expired, replayed or mismatched action");
}
static void tick(){
 if(!Serial){stop();return;}
 // Bound work each loop so untrusted serial data cannot starve sensing.
 for(int n=0;n<256&&Serial.available();n++){
  char c=(char)Serial.read();if(c=='\n'){if(!overflow){line[used]=0;command();}else stop();used=0;overflow=false;}
  else if(used+1<sizeof(line)&&!overflow)line[used++]=c;else{overflow=true;stop();}
 }
}
}

// ---- end patch_runtime.h ----
#endif

#if defined(REWEIRD_HW_CAPTURE)
#include "driver/mcpwm.h"
#include "soc/soc.h"
#endif

#if defined(REWEIRD_HAS_OLED)
#include <Adafruit_GFX.h>
#include <Adafruit_SSD1306.h>
#include <Fonts/FreeSansBold12pt7b.h>
#include <Fonts/FreeSansBold18pt7b.h>
// ---- oled_boot_logo.h ----

// Monochrome silhouette of apps/web/public/images/reweird-logo-mark.png.
// 124 x 60, MSB-first padded rows; centered on the 128 x 64 display.
static const uint8_t reweirdBootLogo[] PROGMEM = {
  0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00,
  0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00,
  0x00, 0x03, 0xFF, 0xFF, 0xFF, 0xFF, 0xFF, 0xC0, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00,
  0x00, 0x01, 0xFF, 0xFF, 0xFF, 0xFF, 0xFF, 0xF0, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00,
  0x00, 0x01, 0xFF, 0xFF, 0xFF, 0xFF, 0xFF, 0xF8, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00,
  0x00, 0x00, 0xFF, 0xFF, 0xFF, 0xFF, 0xFF, 0xFE, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00,
  0x00, 0x00, 0x7F, 0xFF, 0xFF, 0xFF, 0xFF, 0xFF, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00,
  0x00, 0x00, 0x7F, 0xFF, 0xFF, 0xFF, 0xFF, 0xFF, 0x80, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00,
  0x00, 0x00, 0x3F, 0xFF, 0xFF, 0xFF, 0xFF, 0xFF, 0xC0, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00,
  0x00, 0x00, 0x1F, 0xFF, 0xFF, 0xFF, 0xFF, 0xFF, 0xC0, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00,
  0x00, 0x00, 0x0F, 0xFF, 0xFF, 0xFF, 0xFF, 0xFF, 0xE0, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00,
  0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x0F, 0xFF, 0xF0, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00,
  0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x03, 0xFF, 0xF0, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00,
  0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0xFF, 0xF8, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00,
  0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0xFF, 0xF8, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00,
  0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x7F, 0xF8, 0x00, 0x00, 0x00, 0x00, 0x01, 0xFF, 0xC0,
  0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x3F, 0xFC, 0x00, 0x00, 0x00, 0x00, 0x07, 0xFF, 0xC0,
  0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x3F, 0xFC, 0x00, 0x00, 0x00, 0x00, 0x07, 0xFF, 0xC0,
  0x01, 0xFC, 0x00, 0x0F, 0xFF, 0xFF, 0xE0, 0x3F, 0xFC, 0x00, 0x00, 0x00, 0x00, 0x0F, 0xFF, 0x80,
  0x07, 0xFF, 0x00, 0x07, 0xFF, 0xFF, 0xF0, 0x1F, 0xFC, 0x00, 0x00, 0x00, 0x00, 0x1F, 0xFF, 0x80,
  0x0F, 0xFF, 0x80, 0x03, 0xFF, 0xFF, 0xF0, 0x1F, 0xFC, 0x00, 0x00, 0x00, 0x00, 0x1F, 0xFF, 0x00,
  0x1F, 0xFF, 0xC0, 0x03, 0xFF, 0xFF, 0xF0, 0x1F, 0xFC, 0x00, 0x30, 0x00, 0x00, 0x1F, 0xFF, 0x00,
  0x1F, 0xFF, 0xC0, 0x01, 0xFF, 0xFF, 0xF0, 0x1F, 0xFC, 0x00, 0xFC, 0x00, 0x00, 0x3F, 0xFE, 0x00,
  0x3F, 0x0F, 0xE0, 0x01, 0xFF, 0xFF, 0xF0, 0x1F, 0xFC, 0x01, 0xFE, 0x00, 0x00, 0x3F, 0xFE, 0x00,
  0x3E, 0x07, 0xFF, 0xFF, 0xFF, 0xFF, 0xE0, 0x3F, 0xFC, 0x03, 0xFF, 0x00, 0x00, 0x7F, 0xFC, 0x00,
  0x3E, 0x03, 0xFF, 0xFF, 0xFF, 0xFF, 0x80, 0x3F, 0xFC, 0x03, 0xFF, 0x00, 0x00, 0x7F, 0xFC, 0x00,
  0x3E, 0x03, 0xFF, 0xFF, 0xFF, 0xFF, 0x00, 0x7F, 0xFC, 0x07, 0xFF, 0x80, 0x00, 0xFF, 0xF8, 0x00,
  0x3E, 0x03, 0xFF, 0xFF, 0xFF, 0xFF, 0x00, 0x7F, 0xF8, 0x0F, 0xFF, 0x80, 0x00, 0xFF, 0xF8, 0x00,
  0x3E, 0x07, 0xFF, 0xFF, 0xFF, 0xFF, 0x80, 0xFF, 0xF8, 0x0F, 0xFF, 0xC0, 0x01, 0xFF, 0xF0, 0x00,
  0x3F, 0x0F, 0xFF, 0xFF, 0xFF, 0xFF, 0xC3, 0xFF, 0xF0, 0x1F, 0xFF, 0xC0, 0x01, 0xFF, 0xF0, 0x00,
  0x1F, 0xFF, 0xC0, 0x00, 0x00, 0xFF, 0xDF, 0xFF, 0xF0, 0x1F, 0xFF, 0xE0, 0x03, 0xFF, 0xE0, 0x00,
  0x1F, 0xFF, 0x80, 0x00, 0x00, 0x7F, 0xFF, 0xFF, 0xE0, 0x3F, 0xFF, 0xE0, 0x03, 0xFF, 0xE0, 0x00,
  0x0F, 0xFF, 0x00, 0x03, 0xFF, 0x7F, 0xFF, 0xFF, 0xC0, 0x3F, 0xFF, 0xF0, 0x07, 0xFF, 0xC0, 0x00,
  0x07, 0xFE, 0x00, 0x03, 0xFF, 0x3F, 0xFF, 0xFF, 0x80, 0x7F, 0xFF, 0xF0, 0x07, 0xFF, 0xC0, 0x00,
  0x01, 0xF8, 0x00, 0x03, 0xFF, 0x3F, 0xFF, 0xFF, 0x00, 0x7F, 0xFF, 0xF8, 0x0F, 0xFF, 0x80, 0x00,
  0x00, 0x00, 0x00, 0x03, 0xFF, 0x1F, 0xFF, 0xFE, 0x00, 0xFF, 0xFF, 0xF8, 0x0F, 0xFF, 0x80, 0x00,
  0x00, 0x00, 0x00, 0x03, 0xFF, 0x0F, 0xFD, 0xF8, 0x00, 0xFF, 0xFF, 0xFC, 0x1F, 0xFF, 0x00, 0x00,
  0x00, 0x00, 0x00, 0x03, 0xFF, 0x0F, 0xFF, 0xE0, 0x01, 0xFF, 0xFF, 0xFC, 0x1F, 0xFF, 0x00, 0x00,
  0x00, 0x00, 0x00, 0x03, 0xFF, 0x07, 0xFF, 0x00, 0x01, 0xFF, 0xFF, 0xFE, 0x3F, 0xFE, 0x00, 0x00,
  0x00, 0x00, 0x00, 0x03, 0xFF, 0x07, 0xFF, 0x00, 0x03, 0xFF, 0xFF, 0xFE, 0x3F, 0xFE, 0x00, 0x00,
  0x00, 0x00, 0x00, 0x03, 0xFF, 0x03, 0xFF, 0x80, 0x07, 0xFF, 0xFF, 0xFF, 0x7F, 0xFC, 0x00, 0x00,
  0x00, 0x00, 0x00, 0x03, 0xFF, 0x03, 0xFF, 0xC0, 0x07, 0xFF, 0xFF, 0xFF, 0x7F, 0xFC, 0x00, 0x00,
  0x00, 0x00, 0x00, 0x03, 0xFF, 0x01, 0xFF, 0xC0, 0x0F, 0xFF, 0x9F, 0xFF, 0xFF, 0xF8, 0x00, 0x00,
  0x00, 0x00, 0x00, 0x03, 0xFF, 0x00, 0xFF, 0xE0, 0x0F, 0xFF, 0x9F, 0xFF, 0xFF, 0xF8, 0x00, 0x00,
  0x00, 0x00, 0x00, 0x03, 0xFF, 0x00, 0xFF, 0xF0, 0x1F, 0xFF, 0x0F, 0xFF, 0xFF, 0xF0, 0x00, 0x00,
  0x00, 0x00, 0x00, 0x03, 0xFF, 0x00, 0x7F, 0xF0, 0x1F, 0xFF, 0x0F, 0xFF, 0xFF, 0xF0, 0x00, 0x00,
  0x00, 0x00, 0x00, 0x03, 0xFF, 0x00, 0x7F, 0xF8, 0x3F, 0xFE, 0x07, 0xFF, 0xFF, 0xE0, 0x00, 0x00,
  0x00, 0x00, 0x00, 0x03, 0xFF, 0x00, 0x3F, 0xFC, 0x7F, 0xFE, 0x07, 0xFF, 0xFF, 0xE0, 0x00, 0x00,
  0x00, 0x00, 0x00, 0x03, 0xFF, 0x00, 0x3F, 0xFC, 0x7F, 0xFC, 0x03, 0xFF, 0xFF, 0xC0, 0x00, 0x00,
  0x00, 0x00, 0x00, 0x03, 0xFF, 0x00, 0x1F, 0xFE, 0xFF, 0xF8, 0x03, 0xFF, 0xFF, 0xC0, 0x00, 0x00,
  0x00, 0x00, 0x00, 0x03, 0xFF, 0x00, 0x0F, 0xFF, 0xFF, 0xF8, 0x01, 0xFF, 0xFF, 0x80, 0x00, 0x00,
  0x00, 0x00, 0x00, 0x03, 0xFF, 0x00, 0x0F, 0xFF, 0xFF, 0xF0, 0x01, 0xFF, 0xFF, 0x80, 0x00, 0x00,
  0x00, 0x00, 0x00, 0x03, 0xFF, 0x00, 0x07, 0xFF, 0xFF, 0xF0, 0x00, 0xFF, 0xFF, 0x00, 0x00, 0x00,
  0x00, 0x00, 0x00, 0x03, 0xFF, 0x00, 0x07, 0xFF, 0xFF, 0xE0, 0x00, 0xFF, 0xFF, 0x00, 0x00, 0x00,
  0x00, 0x00, 0x00, 0x03, 0xFF, 0x00, 0x03, 0xFF, 0xFF, 0xC0, 0x00, 0x7F, 0xFE, 0x00, 0x00, 0x00,
  0x00, 0x00, 0x00, 0x03, 0xFF, 0x00, 0x01, 0xFF, 0xFF, 0x80, 0x00, 0x7F, 0xFC, 0x00, 0x00, 0x00,
  0x00, 0x00, 0x00, 0x03, 0xFF, 0x00, 0x00, 0xFF, 0xFF, 0x00, 0x00, 0x3F, 0xF8, 0x00, 0x00, 0x00,
  0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x3F, 0xFC, 0x00, 0x00, 0x0F, 0xC0, 0x00, 0x00, 0x00,
  0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00,
  0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00,
};

// ---- end oled_boot_logo.h ----
#include <Wire.h>
#endif

// Everything below measures within one capture window. A gap, period, or
// HIGH time is only reported when both of its edges fall inside the window
// it is reported in, so no value can span windows.
//
// Interval values are stored as integers in the capture's native unit
// (rawPerUS counts per microsecond) because Xtensa interrupt handlers must
// not use the FPU; they are converted to microseconds when a frame is built.
// ---- capture_window.h ----

#include <stdint.h>

struct ProbeAccumulator {
  uint32_t rawPerUS = 1;
  volatile uint32_t edgeCount = 0;
  volatile uint32_t risingEdges = 0;
  volatile uint32_t fallingEdges = 0;
  volatile uint32_t windowStartUS = 0;
  volatile uint32_t lastEdgeUS = 0;
  volatile bool edgeInWindow = false;
  volatile uint32_t maxGapUS = 0;
  volatile bool riseInWindow = false;
  volatile bool previousComplete = false;
  volatile bool periodPending = false;
  volatile uint32_t previousCompleteRise = 0;
  volatile uint32_t pendingPeriod = 0;
  volatile uint32_t lastRiseUS = 0;     // interrupt capture timebase
  volatile uint32_t lastRiseTicks = 0;  // hardware capture timebase (APB ticks)
  volatile uint8_t lastState = 0;
  volatile uint8_t periodCount = 0;
  volatile uint8_t highCount = 0;
  volatile uint32_t periodsRaw[REWEIRD_PULSE_SAMPLES] = {};
  volatile uint32_t highWidthsRaw[REWEIRD_PULSE_SAMPLES] = {};
  uint32_t activityCounts[REWEIRD_ACTIVITY_BUCKETS] = {};
  uint32_t bucketRisingStart = 0;
};

struct ProbeSnapshot {
  uint32_t edgeCount = 0;
  uint32_t risingEdges = 0;
  uint32_t fallingEdges = 0;
  uint32_t maxGapUS = 0;
  uint8_t state = 0;
  uint8_t periodCount = 0;
  uint8_t highCount = 0;
  uint32_t rawPerUS = 1;
  uint32_t periodsRaw[REWEIRD_PULSE_SAMPLES] = {};
  uint32_t highWidthsRaw[REWEIRD_PULSE_SAMPLES] = {};
  uint32_t activityCounts[REWEIRD_ACTIVITY_BUCKETS] = {};
};

// Called under the same lock as window closure. Timestamps are sampled only
// after acquiring it. Unsigned elapsed times are valid across micros() wrap
// for windows shorter than 2^31 us; stale/reversed timestamps are rejected.
static inline void IRAM_ATTR recordEdge(ProbeAccumulator &a, bool rising, uint32_t nowUS, uint32_t rawTime) {
  const uint32_t elapsed = nowUS - a.windowStartUS;
  if (elapsed >= 0x80000000UL) return;
  const uint32_t reference = a.edgeInWindow ? a.lastEdgeUS : a.windowStartUS;
  const uint32_t gap = nowUS - reference;
  if (gap > elapsed) return;
  if (gap > a.maxGapUS) a.maxGapUS = gap;
  a.edgeInWindow = true;
  a.lastEdgeUS = nowUS;
  a.edgeCount++;
  a.lastState = rising ? 1 : 0;
  if (rising) {
    a.risingEdges++;
    // A repeated rise cannot establish a complete pulse/period.
    if (a.riseInWindow) a.previousComplete = false;
    a.pendingPeriod = rawTime - a.previousCompleteRise;
    a.periodPending = a.previousComplete && a.pendingPeriod > 0 &&
      static_cast<uint64_t>(a.pendingPeriod) <= static_cast<uint64_t>(elapsed) * a.rawPerUS;
    a.lastRiseTicks = rawTime;
    a.lastRiseUS = nowUS;
    a.riseInWindow = true;
  } else {
    a.fallingEdges++;
    const uint32_t width = rawTime - a.lastRiseTicks;
    const bool complete = a.riseInWindow && width > 0 &&
      static_cast<uint64_t>(width) <= static_cast<uint64_t>(elapsed) * a.rawPerUS;
    if (complete) {
      if (a.highCount < REWEIRD_PULSE_SAMPLES) a.highWidthsRaw[a.highCount++] = width;
      // Commit a period only after BOTH of its pulses have completed.
      if (a.periodPending && a.periodCount < REWEIRD_PULSE_SAMPLES)
        a.periodsRaw[a.periodCount++] = a.pendingPeriod;
      a.previousCompleteRise = a.lastRiseTicks;
    }
    a.previousComplete = complete;
    a.riseInWindow = false;
    a.periodPending = false;
  }
}

// Caller holds the shared capture lock and supplies the boundary sampled
// inside that lock. All probes close at this same instant.
static ProbeSnapshot snapshotAndReset(ProbeAccumulator &accumulator, uint32_t windowEndUS) {
  ProbeSnapshot snapshot;
  const uint32_t reference = accumulator.edgeInWindow ? accumulator.lastEdgeUS : accumulator.windowStartUS;
  const uint32_t windowUS = windowEndUS - accumulator.windowStartUS;
  const uint32_t trailingGap = windowEndUS - reference;
  // Defensive window bound; the atomic boundary prevents a future edge here.
  const uint32_t boundedTrailing = trailingGap <= windowUS ? trailingGap : 0;
  const uint32_t boundedGap = accumulator.maxGapUS <= windowUS ? accumulator.maxGapUS : 0;
  snapshot.maxGapUS = boundedTrailing > boundedGap ? boundedTrailing : boundedGap;
  snapshot.edgeCount = accumulator.edgeCount;
  snapshot.risingEdges = accumulator.risingEdges;
  snapshot.fallingEdges = accumulator.fallingEdges;
  snapshot.state = accumulator.lastState;
  snapshot.periodCount = accumulator.periodCount;
  snapshot.highCount = accumulator.highCount;
  snapshot.rawPerUS = accumulator.rawPerUS;
  for (size_t sample = 0; sample < REWEIRD_PULSE_SAMPLES; ++sample) {
    snapshot.periodsRaw[sample] = accumulator.periodsRaw[sample];
    snapshot.highWidthsRaw[sample] = accumulator.highWidthsRaw[sample];
  }
  for (size_t bucket = 0; bucket < REWEIRD_ACTIVITY_BUCKETS; ++bucket) {
    snapshot.activityCounts[bucket] = accumulator.activityCounts[bucket];
    accumulator.activityCounts[bucket] = 0;
  }
  accumulator.edgeCount = 0;
  accumulator.risingEdges = 0;
  accumulator.fallingEdges = 0;
  accumulator.periodCount = 0;
  accumulator.highCount = 0;
  accumulator.bucketRisingStart = 0;
  accumulator.windowStartUS = windowEndUS;
  accumulator.edgeInWindow = false;
  accumulator.maxGapUS = 0;
  accumulator.riseInWindow = false;
  accumulator.previousComplete = false;
  accumulator.periodPending = false;
  return snapshot;
}

// ---- end capture_window.h ----

static ProbeAccumulator accumulators[REWEIRD_PROBE_COUNT];
static ProbeSnapshot completedWindows[REWEIRD_PROBE_COUNT];
static uint16_t analogMV[REWEIRD_PROBE_COUNT][REWEIRD_ANALOG_SAMPLES] = {};
static uint8_t analogCounts[REWEIRD_PROBE_COUNT] = {};
static portMUX_TYPE probeMux = portMUX_INITIALIZER_UNLOCKED;
static uint32_t windowStartedMS = 0;
static uint32_t windowStartedUS = 0;
static uint32_t nextAnalogSampleMS = 0;
static uint32_t nextActivityBucketMS = 0;
static size_t activityBucketIndex = 0;
static uint64_t sequenceNumber = 0;
static char deviceID[32] = {};

static const char *modeName(ReWeirdProbeMode mode) {
  switch (mode) {
    case ReWeirdProbeMode::Analog:
      return "analog";
    case ReWeirdProbeMode::Digital:
      return "digital";
    case ReWeirdProbeMode::Pulse:
      return "pulse";
  }
  return "digital";
}

// GPIO interrupt capture (P1-P6 without a hardware capture channel). The
// level is read after the interrupt fires, so pulses shorter than the
// interrupt latency can be misclassified; the backend detects that from the
// rising/falling imbalance and reports the capture as unreliable.
void IRAM_ATTR onProbeEdge(void *argument) {
  const size_t index = reinterpret_cast<size_t>(argument);
  if (index >= REWEIRD_PROBE_COUNT) {
    return;
  }
  auto &accumulator = accumulators[index];
  portENTER_CRITICAL_ISR(&probeMux);
  const uint32_t nowUS = micros();
  const bool rising = digitalRead(REWEIRD_PROBES[index].pin) == HIGH;
  recordEdge(accumulator, rising, nowUS, nowUS);
  portEXIT_CRITICAL_ISR(&probeMux);
}

#if defined(REWEIRD_HW_CAPTURE)
// MCPWM capture: the hardware latches edge polarity and an APB-clock
// timestamp, so widths and periods keep 12.5 ns resolution regardless of
// interrupt latency. If both edges of a pulse arrive before the interrupt is
// serviced, the rising edge is lost; the rising/falling imbalance then makes
// the backend report the capture as unreliable rather than guessing.
static constexpr uint32_t APB_TICKS_PER_US = APB_CLK_FREQ / 1000000;

static bool IRAM_ATTR onCaptureEdge(mcpwm_unit_t, mcpwm_capture_channel_id_t, const cap_event_data_t *event, void *argument) {
  const size_t index = reinterpret_cast<size_t>(argument);
  if (index >= REWEIRD_PROBE_COUNT) {
    return false;
  }
  auto &accumulator = accumulators[index];
  portENTER_CRITICAL_ISR(&probeMux);
  const uint32_t nowUS = static_cast<uint32_t>(esp_timer_get_time());
  const bool rising = event->cap_edge == MCPWM_POS_EDGE;
  recordEdge(accumulator, rising, nowUS, event->cap_value);
  portEXIT_CRITICAL_ISR(&probeMux);
  return false;
}

static bool startHardwareCapture(size_t index, int8_t channel) {
  const mcpwm_io_signals_t signal = channel == 0 ? MCPWM_CAP_0 : channel == 1 ? MCPWM_CAP_1 : MCPWM_CAP_2;
  const mcpwm_capture_channel_id_t captureChannel =
      channel == 0 ? MCPWM_SELECT_CAP0 : channel == 1 ? MCPWM_SELECT_CAP1 : MCPWM_SELECT_CAP2;
  if (mcpwm_gpio_init(MCPWM_UNIT_0, signal, REWEIRD_PROBES[index].pin) != ESP_OK) {
    return false;
  }
  portENTER_CRITICAL(&probeMux);
  accumulators[index].rawPerUS = APB_TICKS_PER_US;
  mcpwm_capture_config_t configuration = {};
  configuration.cap_edge = MCPWM_BOTH_EDGE;
  configuration.cap_prescale = 1;
  configuration.capture_cb = onCaptureEdge;
  configuration.user_data = reinterpret_cast<void *>(index);
  portEXIT_CRITICAL(&probeMux);
  return mcpwm_capture_enable_channel(MCPWM_UNIT_0, captureChannel, &configuration) == ESP_OK;
}
#endif

static void sampleAnalogInputs() {
  for (size_t index = 0; index < REWEIRD_PROBE_COUNT; ++index) {
    if (REWEIRD_PROBES[index].mode != ReWeirdProbeMode::Analog ||
        analogCounts[index] >= REWEIRD_ANALOG_SAMPLES) {
      continue;
    }
    analogMV[index][analogCounts[index]++] =
        static_cast<uint16_t>(analogReadMilliVolts(REWEIRD_PROBES[index].pin));
  }
}

static void closeActivityBucket() {
  if (activityBucketIndex >= REWEIRD_ACTIVITY_BUCKETS) {
    return;
  }
  portENTER_CRITICAL(&probeMux);
  for (size_t index = 0; index < REWEIRD_PROBE_COUNT; ++index) {
    const uint32_t current = accumulators[index].risingEdges;
    accumulators[index].activityCounts[activityBucketIndex] =
        current - accumulators[index].bucketRisingStart;
    accumulators[index].bucketRisingStart = current;
  }
  portEXIT_CRITICAL(&probeMux);
  activityBucketIndex++;
}

#if defined(REWEIRD_HAS_OLED)
// ---------------------------------------------------------------------------
// OLED status display. It shows a copy of the numbers already placed in the
// last Telemetry v2 frame; it never measures. Boot diagnostics are queued for
// the telemetry task to print between complete JSON frames (no interleaving). All
// I2C traffic runs in its own task on core 0 so a slow display cannot delay
// sampling, edge capture, or frame emission on core 1. If the display is
// missing or fails to start, telemetry continues unchanged.
// ---------------------------------------------------------------------------
struct DisplayProbe {
  ReWeirdProbeMode mode = ReWeirdProbeMode::Digital;
  uint32_t risingEdges = 0;
  uint8_t state = 0;
  bool hasWidth = false;
  double widthUS = 0;
  bool hasMillivolts = false;
  double millivolts = 0;
};

struct DisplaySnapshot {
  uint64_t sequence = 0;
  uint32_t windowMS = 0;
  DisplayProbe probes[REWEIRD_PROBE_COUNT];
};

static QueueHandle_t displayQueue = nullptr;
static DisplaySnapshot pendingDisplay;
static TwoWire I2C_A(0);
static TwoWire I2C_B(1);
static Adafruit_SSD1306 liveOneOLED(128, 64, &I2C_A, -1);
static Adafruit_SSD1306 liveTwoOLED(128, 64, &I2C_B, -1);
static bool liveOneReady = false;
static bool liveTwoReady = false;
struct DisplayBootReport { char text[512] = {}; };
static DisplayBootReport displayBootReport;
static size_t displayLogLength = 0;
static QueueHandle_t displayLogQueue = nullptr;

static void displayLog(const char *format, ...) {
  const size_t remaining = sizeof(displayBootReport.text) - displayLogLength;
  if (remaining < 2) return;
  va_list args;
  va_start(args, format);
  const int length = vsnprintf(displayBootReport.text + displayLogLength, remaining, format, args);
  va_end(args);
  if (length > 0) displayLogLength += static_cast<size_t>(length) < remaining ? length : remaining - 1;
}

static void printDisplayBootReport() {
#if defined(REWEIRD_DISPLAY_DIAGNOSTICS) && REWEIRD_DISPLAY_DIAGNOSTICS
  static DisplayBootReport report;
  if (!Serial) return;
  if (displayLogQueue && xQueueReceive(displayLogQueue, &report, 0) == pdTRUE)
    Serial.write(reinterpret_cast<const uint8_t *>(report.text), strlen(report.text));
#endif
}

static bool oledACK(TwoWire &bus) {
  bus.beginTransmission(REWEIRD_OLED_ADDRESS);
  return bus.endTransmission() == 0;
}

static bool beginOLED(Adafruit_SSD1306 &screen, TwoWire &bus) {
  // Verified against installed Adafruit SSD1306 2.5.17:
  // begin(switchvcc, address, reset, periphBegin).
  // Both buses were explicitly initialized; never reinitialize custom pins.
  return oledACK(bus) &&
         screen.begin(SSD1306_SWITCHCAPVCC, REWEIRD_OLED_ADDRESS, false, false) &&
         oledACK(bus);
}

static void formatProbeLine(char *line, size_t size, size_t index, const DisplayProbe &probe) {
  if (index == 5) { snprintf(line, size, "unused"); return; }
  if (probe.mode == ReWeirdProbeMode::Analog) {
    if (probe.hasMillivolts) {
      // Voltage at the ESP32 pin; any divider scale is applied by the backend.
      snprintf(line, size, "%.3fV", probe.millivolts / 1000.0);
    } else {
      snprintf(line, size, "--");
    }
    return;
  }
  if (probe.risingEdges == 0) {
    snprintf(line, size, "0p %s", probe.state ? "HIGH" : "LOW");
  } else if (!probe.hasWidth) {
    snprintf(line, size, "%lup", static_cast<unsigned long>(probe.risingEdges));
  } else if (probe.widthUS >= 1000) {
    snprintf(line, size, "%lup %.1fms", static_cast<unsigned long>(probe.risingEdges), probe.widthUS / 1000.0);
  } else {
    snprintf(line, size, "%lup %.0fus", static_cast<unsigned long>(probe.risingEdges), probe.widthUS);
  }
}


static void drawLiveScreen(Adafruit_SSD1306 &screen, TwoWire &bus,
                           const DisplaySnapshot &snapshot, size_t first, size_t end) {
  if (!oledACK(bus)) return;
  char line[22];
  static const char *labels[] = {"P1 POWER", "P2 TRIG", "P3 ECHO", "P4 SERVO", "P5 ZMPT", "P6 SPARE"};
  screen.clearDisplay();
  screen.setTextWrap(false);
  screen.setFont(nullptr);
  screen.setTextSize(1);
  // A single inverted title bar, three evenly spaced rows, and one footer.
  // All values still come from the completed telemetry window; no resampling.
  screen.fillRect(0, 0, 128, 11, SSD1306_WHITE);
  screen.setTextColor(SSD1306_BLACK);
  screen.setCursor(3, 2);
  screen.print(first == 0 ? "REWEIRD" : "PROBES");
  screen.setCursor(98, 2);
  screen.print("LIVE");
  screen.setTextColor(SSD1306_WHITE);
  for (size_t index = first; index < end; ++index) {
    const int16_t y = 16 + (index - first) * 13;
    screen.setCursor(2, y);
    screen.print(labels[index]);
    formatProbeLine(line, sizeof(line), index, snapshot.probes[index]);
    // Reserve 12 characters for values. Large raw counts remain available in
    // telemetry; use a compact count-only view instead of overlapping labels.
    if (strlen(line) > 12) snprintf(line, sizeof(line), "%lup", static_cast<unsigned long>(snapshot.probes[index].risingEdges));
    screen.setCursor(126 - 6 * strlen(line), y);
    screen.print(line);
  }
  screen.drawFastHLine(0, 53, 128, SSD1306_WHITE);
  screen.setCursor(2, 56);
  if (!liveOneReady || !liveTwoReady)
    screen.print(!liveOneReady ? "OLED A OFFLINE" : "OLED B OFFLINE");
  else
    screen.print(first == 0 ? "V:pin  p:per window" : "SERIAL | PATCH LOCKED");
  screen.display();
}

static void drawDisplay(const DisplaySnapshot &snapshot) {
  // Probe both buses first so the surviving OLED reports the other's loss.
  if (liveOneReady && !oledACK(I2C_A)) liveOneReady = false;
  if (liveTwoReady && !oledACK(I2C_B)) liveTwoReady = false;
  if (liveOneReady) drawLiveScreen(liveOneOLED, I2C_A, snapshot, 0, 3);
  if (liveTwoReady) drawLiveScreen(liveTwoOLED, I2C_B, snapshot, 3, 6);
}

static void startupScreen(Adafruit_SSD1306 &screen, TwoWire &bus, bool logo) {
  if (!oledACK(bus)) return;
  screen.clearDisplay();
  screen.setTextColor(SSD1306_WHITE);
  screen.setTextWrap(false);
  screen.setTextSize(1);
  if (logo) {
    screen.drawBitmap(2, 2, reweirdBootLogo, 124, 60, SSD1306_WHITE);
  } else {
    int16_t x, y;
    uint16_t width, height;
    screen.setFont(&FreeSansBold18pt7b);
    screen.getTextBounds("ReWeird", 0, 0, &x, &y, &width, &height);
    // Fit the larger glyphs to the full usable width instead of dropping
    // down a whole font size when the native wordmark is slightly too wide.
    GFXcanvas1 wordmark(width, height);
    if (wordmark.getBuffer()) {
      wordmark.setFont(&FreeSansBold18pt7b);
      wordmark.setTextColor(1);
      wordmark.setTextWrap(false);
      wordmark.setCursor(-x, -y);
      wordmark.print("ReWeird");
      const uint16_t fittedWidth = 124;
      const uint16_t fittedHeight = height * fittedWidth / width;
      for (uint16_t row = 0; row < fittedHeight; ++row)
        for (uint16_t column = 0; column < fittedWidth; ++column)
          if (wordmark.getPixel(column * width / fittedWidth, row * height / fittedHeight))
            screen.drawPixel(2 + column, (64 - fittedHeight) / 2 + row, SSD1306_WHITE);
    } else {
      screen.setFont(&FreeSansBold12pt7b);
      screen.getTextBounds("ReWeird", 0, 0, &x, &y, &width, &height);
      screen.setCursor((128 - width) / 2 - x, (64 - height) / 2 - y);
      screen.print("ReWeird");
    }
    screen.setFont(nullptr);
  }
  screen.display();
}

static void diagnoseDisplays(bool busAReady, bool busBReady, bool detailed) {
  displayLogLength = 0;
  displayBootReport.text[0] = '\0';
  const bool ackA = busAReady && oledACK(I2C_A);
  const bool ackB = busBReady && oledACK(I2C_B);
  // Do not clear/reset a healthy screen during periodic health checks.
  liveOneReady = ackA && (liveOneReady || beginOLED(liveOneOLED, I2C_A));
  liveTwoReady = ackB && (liveTwoReady || beginOLED(liveTwoOLED, I2C_B));
  if (detailed) {
    displayLog("DISPLAY_WIRE A: BUS=0 SDA=17 SCL=18 BEGIN=%s\n", busAReady ? "OK" : "FAILED");
    displayLog("DISPLAY_WIRE B: BUS=1 SDA=4 SCL=5 BEGIN=%s\n", busBReady ? "OK" : "FAILED");
    displayLog("OLED_A 0x3C: %s INIT=%s\n", ackA ? "ACK" : "NO_ACK", liveOneReady ? "OK" : "OFFLINE");
    displayLog("OLED_B 0x3C: %s INIT=%s\n", ackB ? "ACK" : "NO_ACK", liveTwoReady ? "OK" : "OFFLINE");
  }
  displayLog("DISPLAY_STATUS OLED_A=BUS0:%s OLED_B=BUS1:%s\n",
             liveOneReady ? "OK" : "OFFLINE", liveTwoReady ? "OK" : "OFFLINE");
  if (displayLogQueue) xQueueOverwrite(displayLogQueue, &displayBootReport);
}

static void oledTask(void *) {
  const bool busAReady = I2C_A.begin(REWEIRD_OLED_A_SDA, REWEIRD_OLED_A_SCL, 400000);
  const bool busBReady = I2C_B.begin(REWEIRD_OLED_B_SDA, REWEIRD_OLED_B_SCL, 400000);
  I2C_A.setTimeOut(20);
  I2C_B.setTimeOut(20);
  diagnoseDisplays(busAReady, busBReady, true);
  if (liveOneReady) startupScreen(liveOneOLED, I2C_A, true);
  if (liveTwoReady) startupScreen(liveTwoOLED, I2C_B, false);
  delay(5000); // Display task only; capture and telemetry never wait for OLEDs.
  uint32_t nextDiagnosticMS = 3000;
  bool delayedReport = true;
  for (;;) {
    if (static_cast<int32_t>(millis() - nextDiagnosticMS) >= 0) {
      diagnoseDisplays(busAReady, busBReady, delayedReport);
      delayedReport = false;
      nextDiagnosticMS = millis() + 10000;
    }
    DisplaySnapshot snapshot;
    if (xQueueReceive(displayQueue, &snapshot, portMAX_DELAY) == pdTRUE)
      drawDisplay(snapshot); // Both displays use this SAME completed v2 window.
  }
}

static void startDisplay() {
  displayLogQueue = xQueueCreate(1, sizeof(DisplayBootReport));
  displayQueue = xQueueCreate(1, sizeof(DisplaySnapshot));
  if (displayQueue == nullptr || displayLogQueue == nullptr ||
      xTaskCreatePinnedToCore(oledTask, "reweird-oled", 4096, nullptr, 1, nullptr, 0) != pdTRUE) {
    // setup() has not started telemetry yet; this cannot split a JSON frame.
    displayLogLength = 0;
    displayLog("DISPLAY_STATUS ERROR=TASK_OR_QUEUE_FAILED\n");
    if (displayLogQueue) xQueueOverwrite(displayLogQueue, &displayBootReport);
  }
}

static void publishDisplay(uint64_t sequence, uint32_t windowMS) {
  if (displayQueue == nullptr) {
    return;
  }
  pendingDisplay.sequence = sequence;
  pendingDisplay.windowMS = windowMS;
  xQueueOverwrite(displayQueue, &pendingDisplay);
}
#endif

// Converts a raw interval to microseconds, keeping 0.1 us for hardware
// captures. Runs outside interrupt context.
static double toMicroseconds(uint32_t raw, uint32_t rawPerUS) {
  if (rawPerUS <= 1) {
    return raw;
  }
  return round(static_cast<double>(raw) * 10.0 / rawPerUS) / 10.0;
}

static void emitTelemetry(uint32_t windowMS, uint32_t windowEndUS) {
  JsonDocument document;
  document["schema_version"] = 2;
  document["device_id"] = deviceID;
  document["profile_id"] = REWEIRD_PROFILE_ID;
  // Measurement capability never implies electrical-output capability.
  JsonObject patch = document["patch"].to<JsonObject>();
  patch["capable"] = false; // stock/host builds have no qualified output stage
  patch["state"] = "LOCKED";
  patch["reason"] = "NO_VERIFIED_DEDICATED_OUTPUT_STAGE";
  patch["boot_id"] = static_cast<uint32_t>(sequenceNumber >> 32);
  patch["max_duration_ms"] = 0; // no physical commands are supported
#if defined(ESP_PLATFORM)
  patch["capable"] = PatchRuntime::provisioned && PatchRuntime::timerReady && PatchRuntime::pins.interlock() && strcmp(REWEIRD_PROFILE_ID, PatchProvision::ProfileID)==0;
  patch["state"] = patch["capable"].as<bool>() ? PatchRuntime::stateLabel() : "LOCKED";
  if (patch["capable"].as<bool>()) { patch["max_duration_ms"] = ReWeirdPatch::MaxDurationMS; patch["reason"] = "QUALIFIED_INTERFACE_REQUIRES_HUMAN_APPROVAL"; }
#endif
  document["captured_at_ms"] = 0;  // No trusted wall clock on the device.
  document["uptime_ms"] = millis();
  document["window_ms"] = windowMS;
  const uint64_t frameSequence = sequenceNumber++;
  document["sequence"] = frameSequence;
  JsonArray samples = document["samples"].to<JsonArray>();

  for (size_t index = 0; index < REWEIRD_PROBE_COUNT; ++index) {
    const auto &configuration = REWEIRD_PROBES[index];
    JsonObject sample = samples.add<JsonObject>();
    sample["probe"] = configuration.id;
    sample["mode"] = modeName(configuration.mode);

    if (configuration.mode == ReWeirdProbeMode::Analog) {
      JsonArray values = sample["analog_mv"].to<JsonArray>();
      uint32_t totalMV = 0;
      for (size_t value = 0; value < analogCounts[index]; ++value) {
        values.add(analogMV[index][value]);
        totalMV += analogMV[index][value];
      }
#if defined(REWEIRD_HAS_OLED)
      pendingDisplay.probes[index].mode = configuration.mode;
      pendingDisplay.probes[index].hasMillivolts = analogCounts[index] > 0;
      pendingDisplay.probes[index].millivolts = analogCounts[index] ? static_cast<double>(totalMV) / analogCounts[index] : 0;
#endif
      analogCounts[index] = 0;
      continue;
    }

    const ProbeSnapshot &snapshot = completedWindows[index];
    sample["state"] = snapshot.state;
    sample["edge_count"] = snapshot.edgeCount;
    sample["rising_edges"] = snapshot.risingEdges;
    sample["falling_edges"] = snapshot.fallingEdges;
    sample["max_gap_us"] = snapshot.maxGapUS;

    JsonArray periods = sample["periods_us"].to<JsonArray>();
    for (size_t value = 0; value < snapshot.periodCount; ++value) {
      periods.add(toMicroseconds(snapshot.periodsRaw[value], snapshot.rawPerUS));
    }
    JsonArray highWidths = sample["high_pulse_widths_us"].to<JsonArray>();
    double widthTotal = 0;
    for (size_t value = 0; value < snapshot.highCount; ++value) {
      const double width = toMicroseconds(snapshot.highWidthsRaw[value], snapshot.rawPerUS);
      highWidths.add(width);
      widthTotal += width;
    }
#if defined(REWEIRD_HAS_OLED)
    pendingDisplay.probes[index].mode = configuration.mode;
    pendingDisplay.probes[index].risingEdges = snapshot.risingEdges;
    pendingDisplay.probes[index].state = snapshot.state;
    pendingDisplay.probes[index].hasWidth = snapshot.highCount > 0;
    pendingDisplay.probes[index].widthUS = snapshot.highCount ? widthTotal / snapshot.highCount : 0;
#endif
    JsonArray activity = sample["activity_counts"].to<JsonArray>();
    for (size_t bucket = 0; bucket < REWEIRD_ACTIVITY_BUCKETS; ++bucket) {
      activity.add(snapshot.activityCounts[bucket]);
    }
  }

#if defined(REWEIRD_HAS_OLED)
  printDisplayBootReport(); // boot-only text; JSON schema and measurements unchanged
#endif
  serializeJson(document, Serial);
  Serial.println();
#if defined(REWEIRD_HAS_OLED)
  publishDisplay(frameSequence, windowMS);
#endif
}

// The device has no wall clock, so captured_at_ms is 0 and the sequence is
// the only frame identity. A boot counter kept in flash (one write per boot)
// forms the upper 32 bits, so the sequence keeps increasing across resets
// and reflashes instead of restarting at 0.
static uint64_t firstSequenceForThisBoot() {
  Preferences preferences;
  if (!preferences.begin("reweird", false)) {
    return 0;
  }
  const uint32_t boots = preferences.getUInt("boots", 0) + 1;
  preferences.putUInt("boots", boots);
  preferences.end();
  return static_cast<uint64_t>(boots) << 32;
}

void setup() {
  Serial.begin(REWEIRD_SERIAL_BAUD);
  delay(250);

  const uint64_t chipID = ESP.getEfuseMac();
  snprintf(deviceID, sizeof(deviceID), "reweird-%04X%08X",
           static_cast<uint16_t>(chipID >> 32), static_cast<uint32_t>(chipID));
  sequenceNumber = firstSequenceForThisBoot();
#if defined(ESP_PLATFORM)
  PatchRuntime::begin(deviceID, REWEIRD_PROFILE_ID, static_cast<uint32_t>(sequenceNumber >> 32));
#endif
#if defined(REWEIRD_HAS_OLED)
  startDisplay();
#endif

  // PATCH remains electrically passive. Firmware never switches it to OUTPUT.
  // Targets without a PATCH line (ESP32-S3 board) configure no PATCH pin at all.
  if (REWEIRD_HAS_PATCH_PIN) {
    pinMode(REWEIRD_PATCH_PIN, INPUT);
  }

  analogReadResolution(12);
  for (size_t index = 0; index < REWEIRD_PROBE_COUNT; ++index) {
    const auto &configuration = REWEIRD_PROBES[index];
    pinMode(configuration.pin, INPUT);
    accumulators[index].lastState = static_cast<uint8_t>(digitalRead(configuration.pin));
    if (configuration.mode == ReWeirdProbeMode::Analog) {
      analogSetPinAttenuation(configuration.pin, ADC_11db);
      continue;
    }
#if defined(REWEIRD_HW_CAPTURE)
    if (REWEIRD_CAPTURE_CHANNEL[index] >= 0 && startHardwareCapture(index, REWEIRD_CAPTURE_CHANNEL[index])) {
      continue;
    }
#endif
    attachInterruptArg(configuration.pin, onProbeEdge, reinterpret_cast<void *>(index), CHANGE);
  }

  // Open the first window for every probe at one instant.
  portENTER_CRITICAL(&probeMux);
  windowStartedUS = micros();
  for (size_t index = 0; index < REWEIRD_PROBE_COUNT; ++index) {
    snapshotAndReset(accumulators[index], windowStartedUS);
  }
  portEXIT_CRITICAL(&probeMux);
  windowStartedMS = millis();
  nextAnalogSampleMS = windowStartedMS;
  nextActivityBucketMS = windowStartedMS + REWEIRD_WINDOW_MS / REWEIRD_ACTIVITY_BUCKETS;
}

void loop() {
#if defined(ESP_PLATFORM)
  PatchRuntime::tick();
#endif
  const uint32_t nowMS = millis();
  if (static_cast<int32_t>(nowMS - nextAnalogSampleMS) >= 0) {
    sampleAnalogInputs();
    nextAnalogSampleMS += REWEIRD_WINDOW_MS / REWEIRD_ANALOG_SAMPLES;
  }
  if (static_cast<int32_t>(nowMS - nextActivityBucketMS) >= 0) {
    closeActivityBucket();
    nextActivityBucketMS += REWEIRD_WINDOW_MS / REWEIRD_ACTIVITY_BUCKETS;
  }
  if (nowMS - windowStartedMS >= REWEIRD_WINDOW_MS) {
    while (activityBucketIndex < REWEIRD_ACTIVITY_BUCKETS) {
      closeActivityBucket();
    }
    // window_ms is measured on the same microsecond clock that bounds every
    // gap and pulse, so no reported interval can exceed the window.
    // Close ALL digital probes before JSON/analog serialization. No ISR can
    // append an edge after this boundary to an already-closing window.
    portENTER_CRITICAL(&probeMux);
    const uint32_t windowEndUS = micros();
    for (size_t index = 0; index < REWEIRD_PROBE_COUNT; ++index)
      completedWindows[index] = snapshotAndReset(accumulators[index], windowEndUS);
    portEXIT_CRITICAL(&probeMux);
    emitTelemetry((windowEndUS - windowStartedUS + 999) / 1000, windowEndUS);
    windowStartedUS = windowEndUS;
    windowStartedMS = nowMS;
    nextAnalogSampleMS = nowMS;
    nextActivityBucketMS = nowMS + REWEIRD_WINDOW_MS / REWEIRD_ACTIVITY_BUCKETS;
    activityBucketIndex = 0;
  }
  delay(1);
}
