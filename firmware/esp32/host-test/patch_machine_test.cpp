#include <cassert>
#include <cstdio>
#include "patch_machine.h"
struct Output:ReWeirdPatch::Output {
 bool connected=true, driven=false,high=false;int drives=0;
 bool interlock()override{return connected;}
 void disable()override{driven=false;}
 void drive(bool level)override{driven=true;high=level;drives++;}
};
int main(){
 using M=ReWeirdPatch::Machine;
 Output o;M locked(o,false);assert(!o.driven&&locked.state==M::Locked);assert(!locked.arm(0,10,true)&&!locked.execute(0)&&o.drives==0);
 M m(o,true);assert(m.arm(0,10,true)&&!o.driven);assert(m.execute(1)&&o.driven);m.tick(10,true);assert(o.driven);m.tick(11,true);assert(!o.driven&&m.completed&&m.state==M::Disabled);
 assert(!m.execute(12)&&!o.driven); // consumed arm/replay
 assert(!m.arm(20,251,true));assert(!m.arm(20,0,true));
 assert(m.arm(30,250,false));assert(m.execute(31));m.tick(130,true);assert(!o.driven&&!m.completed); // lost heartbeat
 assert(m.arm(200,250,true));assert(m.execute(201));for(int i=220;i<450;i+=20){m.heartbeat(i);assert(o.driven);}m.tick(451,true);assert(!o.driven&&m.completed);
 assert(m.arm(500,250,true));assert(m.execute(501));o.connected=false;m.tick(502,true);assert(!o.driven&&m.state==M::Locked&&!m.completed);
 o.connected=true;m.tick(599,true);assert(m.state==M::Ready&&!o.driven);assert(m.arm(600,100,true));assert(m.execute(601));m.tick(602,false);assert(!o.driven&&!m.completed);
 assert(m.arm(700,100,true));assert(m.execute(701));m.stop();assert(!o.driven&&!m.completed); // malformed/error/cancel
 M reboot(o,true);assert(!o.driven&&!reboot.execute(702));
 assert(m.arm(800,10,true));assert(!m.execute(901)); // expired arm
 assert(m.arm(1000,10,true,1020));assert(!m.execute(1015)); // approval expires before bounded end
 assert(ReWeirdPatch::reservedPin(4)&&ReWeirdPatch::reservedPin(5));
 puts("PATCH firmware state-machine tests PASS");
}
