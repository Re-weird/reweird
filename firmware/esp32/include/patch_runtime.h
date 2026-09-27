#pragma once
#include "patch_provision.h"
#include "patch_machine.h"
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
