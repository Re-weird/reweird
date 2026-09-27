#pragma once
#include <cstdint>
#include <cstddef>
#include <cstdio>
#include <cmath>
using std::round;
#define IRAM_ATTR
#define HIGH 1
#define LOW 0
#define INPUT 0x01
#define CHANGE 0x03
typedef int portMUX_TYPE;
#define portMUX_INITIALIZER_UNLOCKED 0
inline void portENTER_CRITICAL(portMUX_TYPE*) {}
inline void portEXIT_CRITICAL(portMUX_TYPE*) {}
inline void portENTER_CRITICAL_ISR(portMUX_TYPE*) {}
inline void portEXIT_CRITICAL_ISR(portMUX_TYPE*) {}
typedef enum { ADC_0db, ADC_2_5db, ADC_6db, ADC_11db } adc_attenuation_t;
unsigned long micros(); unsigned long millis(); void delay(uint32_t);
int digitalRead(uint8_t); void pinMode(uint8_t, uint8_t);
void analogReadResolution(uint8_t); void analogSetPinAttenuation(uint8_t, adc_attenuation_t);
uint32_t analogReadMilliVolts(uint8_t);
void attachInterruptArg(uint8_t, void (*)(void*), void*, int);
struct HWCDC { explicit operator bool() const { return true; } void begin(unsigned long); size_t write(uint8_t); size_t write(const uint8_t*, size_t); size_t println(); };
extern HWCDC Serial;
struct EspClass { uint64_t getEfuseMac(); }; extern EspClass ESP;
typedef int esp_err_t;
#define ESP_OK 0
// FreeRTOS subset used by the OLED task.
typedef void* QueueHandle_t;
typedef void (*TaskFunction_t)(void*);
#define pdTRUE 1
#define pdFALSE 0
#define portMAX_DELAY 0xFFFFFFFFu
QueueHandle_t xQueueCreate(unsigned length, unsigned itemSize);
int xQueueOverwrite(QueueHandle_t queue, const void* item);
int xQueueReceive(QueueHandle_t queue, void* item, uint32_t ticks);
int xTaskCreatePinnedToCore(TaskFunction_t fn, const char* name, uint32_t stack, void* arg, unsigned priority, void* handle, int core);
void vTaskDelete(void* task);
