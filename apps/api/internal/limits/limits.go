package limits

import (
	"os"
	"strconv"
)

// Bounded allows operators to lower resource ceilings without raising the
// audited hard cap. Invalid configuration falls back to the safe default.
func Bounded(name string, fallback, minimum, maximum int) int {
	value, err := strconv.Atoi(os.Getenv(name))
	if err != nil || value < minimum || value > maximum {
		return fallback
	}
	return value
}
