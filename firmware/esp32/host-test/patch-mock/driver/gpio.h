#pragma once
using gpio_num_t=int;
constexpr int GPIO_MODE_INPUT=0,GPIO_MODE_OUTPUT=1;
inline int gpioLevels[49]={};inline int gpioModes[49]={};
inline int gpio_get_level(int p){return gpioLevels[p];}
inline void gpio_set_level(int p,int v){gpioLevels[p]=v;}
inline void gpio_set_direction(int p,int mode){gpioModes[p]=mode;}
