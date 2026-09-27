#pragma once
#include <stdint.h>
#include "patch_safety.h"

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
