# PROBE service

This boundary will host the Gemini-backed diagnostic interpreter. Its input is a
structured evidence object; its output is ranked hypotheses, an explanation, and
a recommended test. It cannot write measurements or address hardware directly.
