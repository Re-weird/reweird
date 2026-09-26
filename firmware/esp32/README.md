# ESP32 firmware boundary

The MVP does not include physical firmware. A real device adapter should emit the
same normalized fields as the simulator: probe ID, timestamp, voltage or activity
facts, digital state, and bounded sample summaries.

The device must authenticate to the API. It must never accept arbitrary model
text as a GPIO command. Future PATCH commands require an allow-listed pin,
validated voltage/waveform/frequency/duration, an expiration time, and explicit
project safety policy.
