#pragma once
inline unsigned esp_random(){static unsigned n=1234567;return ++n;}
