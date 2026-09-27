#pragma once
#include <cstdint>
#define BIT(n) (1UL << (n))
typedef int esp_err_t;
typedef enum { MCPWM_UNIT_0, MCPWM_UNIT_1, MCPWM_UNIT_MAX } mcpwm_unit_t;
typedef enum { MCPWM0A = 0, MCPWM_CAP_0 = 84, MCPWM_CAP_1, MCPWM_CAP_2 } mcpwm_io_signals_t;
typedef enum { MCPWM_NEG_EDGE = BIT(0), MCPWM_POS_EDGE = BIT(1), MCPWM_BOTH_EDGE = BIT(1) | BIT(0) } mcpwm_capture_on_edge_t;
typedef enum { MCPWM_SELECT_CAP0, MCPWM_SELECT_CAP1, MCPWM_SELECT_CAP2 } mcpwm_capture_signal_t;
typedef mcpwm_capture_signal_t mcpwm_capture_channel_id_t;
typedef struct { mcpwm_capture_on_edge_t cap_edge; uint32_t cap_value; } cap_event_data_t;
typedef bool (*cap_isr_cb_t)(mcpwm_unit_t mcpwm, mcpwm_capture_channel_id_t cap_channel, const cap_event_data_t *edata, void *user_data);
typedef struct { mcpwm_capture_on_edge_t cap_edge; uint32_t cap_prescale; cap_isr_cb_t capture_cb; void *user_data; } mcpwm_capture_config_t;
esp_err_t mcpwm_gpio_init(mcpwm_unit_t mcpwm_num, mcpwm_io_signals_t io_signal, int gpio_num);
esp_err_t mcpwm_capture_enable_channel(mcpwm_unit_t mcpwm_num, mcpwm_capture_channel_id_t cap_channel, const mcpwm_capture_config_t *cap_conf);
