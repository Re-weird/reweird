#include <cassert>
#include <cstdio>
#include <cstring>
#include <string>
#include <ArduinoJson.h>
using namespace ArduinoJson;
static uint64_t clockMS=1;
uint64_t esp_timer_get_time(){return clockMS*1000;}
uint32_t millis(){return (uint32_t)clockMS;}
using portMUX_TYPE=int;
#define portMUX_INITIALIZER_UNLOCKED 0
#define portENTER_CRITICAL(x) ((void)0)
#define portEXIT_CRITICAL(x) ((void)0)
constexpr int ESP_OK=0;
using esp_timer_handle_t=void*;
struct esp_timer_create_args_t{void(*callback)(void*);const char*name;};
int esp_timer_create(esp_timer_create_args_t*,esp_timer_handle_t*){return 0;}
int esp_timer_start_periodic(esp_timer_handle_t,unsigned){return 0;}
struct SerialMock {
 std::string output;bool connected=true;
 size_t write(uint8_t c){output+=(char)c;return 1;}
 size_t write(const uint8_t*p,size_t n){output.append((const char*)p,n);return n;}
 void println(){output+='\n';}
 int available(){return 0;}int read(){return -1;}
 explicit operator bool()const{return connected;}
} Serial;
#include "patch_runtime.h"
JsonDocument send(JsonDocument &c){
 Serial.output.clear();serializeJson(c,PatchRuntime::line,sizeof(PatchRuntime::line));PatchRuntime::command();JsonDocument r;assert(!deserializeJson(r,Serial.output));return r;
}
JsonDocument command(const char *op){JsonDocument c;c["type"]="patch_command";c["request_id"]="01234567890123456789012345678901";c["op"]=op;c["challenge"]=PatchRuntime::challenge;return c;}
void hello(){auto c=command("hello");c["now_ms"]=(int64_t)1700000000000+clockMS;auto r=send(c);assert(r["ok"]==true);}
void rehash(JsonDocument &c){std::string bytes;serializeJson(c["action"]["parameters"],bytes);unsigned char raw[32];char hash[65];assert(!mbedtls_sha256_ret((const unsigned char*)bytes.data(),bytes.size(),raw,0));for(int i=0;i<32;i++)snprintf(hash+i*2,3,"%02x",raw[i]);c["digest"]=hash;c["action"]["digest"]=hash;c["action"]["approval"]["digest"]=hash;}
JsonDocument arm(int n){
 auto c=command("arm");char id[33];snprintf(id,sizeof(id),"%032d",n);c["action_id"]=id;
 auto a=c["action"].to<JsonObject>();a["id"]=id;
 auto p=a["parameters"].to<JsonObject>();p["profile_id"]="test-project";p["profile_revision"]=1;p["probe_map_hash"]="test-map";p["device_id"]="device";p["boot_id"]="1";p["target_node"]="isolated-input";p["patch_pin"]=10;p["mode"]="PULSE";p["logic_level"]="HIGH";p["max_voltage"]=3.3;p["frequency_hz"]=0;p["duty_cycle"]=0;p["duration_ms"]=10;p["expires_at_ms"]=(int64_t)1700000003000;p["source"]="REAL_SERIAL";
 std::string canonical;serializeJson(p,canonical);unsigned char raw[32];char digest[65];assert(!mbedtls_sha256_ret((const unsigned char*)canonical.data(),canonical.size(),raw,0));for(int i=0;i<32;i++)snprintf(digest+i*2,3,"%02x",raw[i]);
 c["digest"]=digest;a["digest"]=digest;auto approved=a["approval"].to<JsonObject>();approved["digest"]=digest;approved["actor"]="human";approved["expires_at_ms"]=(int64_t)1700000002000;
 return c;
}
int main(){
 gpioLevels[12]=1;PatchRuntime::begin("device","test-project",1);assert(gpioLevels[11]==0&&gpioModes[10]==GPIO_MODE_INPUT);hello();
 auto a=arm(1);auto r=send(a);assert(r["ok"]==true&&r["state"]=="ARMED");
 auto execute=command("execute");execute["action_id"]=a["action_id"];execute["digest"]=a["digest"];r=send(execute);assert(r["state"]=="ACTIVE"&&gpioLevels[11]==1);
 clockMS+=11;PatchRuntime::watchdog(nullptr);assert(gpioLevels[11]==0&&gpioModes[10]==GPIO_MODE_INPUT&&PatchRuntime::machine->completed);
 r=send(execute);assert(r["ok"]==false);hello();a["challenge"]=PatchRuntime::challenge;r=send(a);assert(r["ok"]==false); // replay across hello
 for(int n=2;n<9;n++){
  hello();a=arm(n);switch(n){case 2:a["action"]["parameters"]["patch_pin"]=4;break;case 3:a["action"]["parameters"]["duration_ms"]=251;break;case 4:a["action"]["parameters"]["max_voltage"]=5;break;case 5:a["action"]["parameters"]["source"]="SIMULATED";break;case 6:a["action"]["parameters"]["boot_id"]="old";break;case 7:a["action"]["approval"]["expires_at_ms"]=1;break;case 8:a["action"]["parameters"]["profile_id"]="other";break;}
  rehash(a);r=send(a);assert(r["ok"]==false&&gpioLevels[11]==0);
 }
 hello();a=arm(9);r=send(a);assert(r["ok"]==true);clockMS+=101;PatchRuntime::watchdog(nullptr);assert(gpioLevels[11]==0&&!PatchRuntime::machine->completed);
 hello();a=arm(10);r=send(a);assert(r["ok"]==true);execute=command("execute");execute["action_id"]=a["action_id"];execute["digest"]=a["digest"];r=send(execute);assert(gpioLevels[11]);strcpy(PatchRuntime::line,"{malformed");PatchRuntime::command();assert(!gpioLevels[11]);
 hello();a=arm(11);gpioLevels[12]=0;r=send(a);assert(r["ok"]==false&&r["state"]=="LOCKED");
 gpioLevels[12]=1;hello();a=arm(12);a["action"]["parameters"]["duration_ms"]=11;r=send(a);assert(r["ok"]==false); // changed approved parameters
 hello();a=arm(13);a["action"].remove("approval");r=send(a);assert(r["ok"]==false);
 hello();a=arm(14);a["action"]["parameters"]["duration_ms"]=250;rehash(a);r=send(a);assert(r["ok"]==true);execute=command("execute");execute["action_id"]=a["action_id"];execute["digest"]=a["digest"];r=send(execute);assert(gpioLevels[11]);clockMS+=101;PatchRuntime::watchdog(nullptr);assert(!gpioLevels[11]&&!PatchRuntime::machine->completed);
 hello();a=arm(15);r=send(a);assert(r["ok"]==true);execute=command("execute");execute["action_id"]=a["action_id"];execute["digest"]=a["digest"];r=send(execute);Serial.connected=false;PatchRuntime::tick();assert(!gpioLevels[11]&&gpioModes[10]==GPIO_MODE_INPUT);
 puts("PATCH production command parser / digest / replay / watchdog tests PASS");
}
