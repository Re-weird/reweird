"""Deterministic text-safety checks shared by patch-proposal generation and
validation. These are a defense-in-depth secondary check, not the primary
guarantee: the primary guarantee is structural - PatchProposal/PatchType
have no field capable of expressing a GPIO command, firmware, or serial
data in the first place (see app/schemas.py's PatchType Literal and
PatchProposal's field set). This module additionally rejects free-text
content that reads like an executable hardware instruction or a claim that
execution already happened, so a proposal never even textually resembles
one.
"""

_HARDWARE_INSTRUCTION_TOKENS = (
    "digitalwrite(",
    "digitalread(",
    "analogwrite(",
    "pinmode(",
    "pulsein(",
    "gpio.",
    "gpio(",
    "gpiozero",
    "serial.write(",
    "serial.print(",
    "import serial",
    "import rpi",
    "subprocess.",
    "os.system(",
)

_EXECUTION_CLAIM_PHRASES = (
    "already executed",
    "was executed",
    "has been executed",
    "already applied",
    "has been applied",
    "confirmed executed",
    "patch has occurred",
)


def contains_executable_hardware_instruction(text: str) -> bool:
    lowered = text.lower()
    return any(token in lowered for token in _HARDWARE_INSTRUCTION_TOKENS)


def contains_execution_claim(text: str) -> bool:
    lowered = text.lower()
    return any(phrase in lowered for phrase in _EXECUTION_CLAIM_PHRASES)


def scan_text_fields(*texts: str) -> str | None:
    """Returns a human-readable rejection reason for the first unsafe text
    found, or None if all given texts are safe."""
    for text in texts:
        if contains_executable_hardware_instruction(text):
            return f"Text contains what appears to be an executable hardware instruction: {text!r}"
        if contains_execution_claim(text):
            return f"Text claims execution already happened, which a proposal must never assert: {text!r}"
    return None
