package codeanalysis

import (
	"fmt"
	"path/filepath"
	"regexp"
	"sort"
	"strconv"
	"strings"

	"github.com/re-weird/reweird/apps/api/internal/domain"
)

type Analyzer interface {
	Analyze(filename, source string) domain.CodeAnalysis
}

type DeterministicAnalyzer struct{}

func New() *DeterministicAnalyzer { return &DeterministicAnalyzer{} }

var (
	definePattern      = regexp.MustCompile(`(?m)^\s*#\s*define\s+([A-Za-z_]\w*)\s+([0-9]{1,2})\b`)
	declarationPattern = regexp.MustCompile(`(?m)\b(?:const(?:expr)?\s+)?(?:unsigned\s+)?(?:int|long|short|byte|uint8_t|uint16_t|gpio_num_t)\s+([A-Za-z_]\w*)\s*=\s*([0-9]{1,2})\b`)
	includePattern     = regexp.MustCompile(`(?m)^\s*#\s*include\s*[<"]([^>"]+)[>"]`)
	callPattern        = regexp.MustCompile(`\b(pinMode|digitalWrite|digitalRead|analogRead|analogWrite|pulseIn|pulseInLong|ledcAttachPin|ledcAttach|touchRead)\s*\(\s*([A-Za-z_]\w*|[0-9]{1,2})\s*(?:,\s*([A-Za-z_]\w*|[0-9]+))?`)
	modePattern        = regexp.MustCompile(`\b(INPUT_PULLUP|INPUT_PULLDOWN|INPUT|OUTPUT|OUTPUT_OPEN_DRAIN)\b`)
	ledcSetupPattern   = regexp.MustCompile(`\bledcSetup\s*\(\s*[^,]+,\s*([0-9]+(?:\.[0-9]+)?)`)
	timingPattern      = regexp.MustCompile(`\b(delay|delayMicroseconds)\s*\(\s*([0-9]+)\s*\)`)
	wireBeginPattern   = regexp.MustCompile(`\bWire\s*\.\s*begin\s*\(\s*([A-Za-z_]\w*|[0-9]{1,2})\s*,\s*([A-Za-z_]\w*|[0-9]{1,2})`)
	pythonConstant     = regexp.MustCompile(`(?m)^\s*([A-Za-z_]\w*)\s*=\s*([0-9]{1,2})\s*(?:#.*)?$`)
	pythonPinPattern   = regexp.MustCompile(`\bPin\s*\(\s*([A-Za-z_]\w*|[0-9]{1,2})\s*,\s*Pin\.(IN|OUT)`)
	pythonPWMPattern   = regexp.MustCompile(`\bPWM\s*\(\s*Pin\s*\(\s*([A-Za-z_]\w*|[0-9]{1,2})`)
)

type mutablePin struct {
	gpio       int
	symbol     string
	direction  string
	behavior   string
	confidence float64
	evidence   []string
}

func (analyzer *DeterministicAnalyzer) Analyze(filename, source string) domain.CodeAnalysis {
	language := languageFor(filename)
	result := domain.CodeAnalysis{
		Status:    "OK",
		Language:  language,
		Parser:    "deterministic-structure-v1",
		Pins:      []domain.CodePinFinding{},
		Includes:  []string{},
		Libraries: []string{},
		Timing:    []string{},
		Warnings:  []string{},
	}
	if strings.TrimSpace(source) == "" {
		result.Status = "EMPTY"
		result.Warnings = append(result.Warnings, "No source code was provided.")
		return result
	}
	switch language {
	case "arduino-cpp", "c", "cpp", "text":
		return analyzeCpp(result, source)
	case "python":
		return analyzePython(result, source)
	default:
		result.Status = "UNSUPPORTED"
		result.Warnings = append(result.Warnings, fmt.Sprintf("The %s file type is stored but has no deterministic parser.", filepath.Ext(filename)))
		return result
	}
}

func analyzeCpp(result domain.CodeAnalysis, source string) domain.CodeAnalysis {
	constants := collectConstants(source, definePattern, declarationPattern)
	pins := make(map[int]*mutablePin)
	for _, match := range includePattern.FindAllStringSubmatch(source, -1) {
		result.Includes = appendUnique(result.Includes, match[1])
		result.Libraries = appendUnique(result.Libraries, libraryName(match[1]))
	}
	for _, match := range callPattern.FindAllStringSubmatch(source, -1) {
		gpio, symbol, ok := resolvePin(match[2], constants)
		if !ok {
			continue
		}
		pin := ensurePin(pins, gpio, symbol)
		call := match[1]
		argument := match[3]
		switch call {
		case "pinMode":
			if mode := modePattern.FindString(argument); mode != "" {
				if strings.HasPrefix(mode, "INPUT") {
					pin.direction = "input"
				} else {
					pin.direction = "output"
				}
				pin.evidence = appendUnique(pin.evidence, fmt.Sprintf("pinMode(%s, %s)", match[2], mode))
			}
		case "digitalWrite":
			pin.direction = preferDirection(pin.direction, "output")
			pin.behavior = preferBehavior(pin.behavior, "digital_output")
			pin.evidence = appendUnique(pin.evidence, "digitalWrite")
		case "digitalRead":
			pin.direction = preferDirection(pin.direction, "input")
			pin.behavior = preferBehavior(pin.behavior, "digital_input")
			pin.evidence = appendUnique(pin.evidence, "digitalRead")
		case "analogRead", "touchRead":
			pin.direction = preferDirection(pin.direction, "input")
			pin.behavior = preferBehavior(pin.behavior, "analog_input")
			pin.evidence = appendUnique(pin.evidence, call)
		case "analogWrite":
			pin.direction = preferDirection(pin.direction, "output")
			pin.behavior = "pwm_output"
			pin.evidence = appendUnique(pin.evidence, "analogWrite")
		case "pulseIn", "pulseInLong":
			pin.direction = preferDirection(pin.direction, "input")
			pin.behavior = "pulse_input"
			pin.evidence = appendUnique(pin.evidence, call)
		case "ledcAttachPin", "ledcAttach":
			pin.direction = preferDirection(pin.direction, "output")
			pin.behavior = "pwm_output"
			pin.evidence = appendUnique(pin.evidence, call)
		}
	}
	if match := ledcSetupPattern.FindStringSubmatch(source); len(match) > 1 {
		result.Timing = appendUnique(result.Timing, "LEDC frequency "+match[1]+" Hz")
	}
	for _, match := range timingPattern.FindAllStringSubmatch(source, -1) {
		unit := "ms"
		if match[1] == "delayMicroseconds" {
			unit = "us"
		}
		result.Timing = appendUnique(result.Timing, match[1]+" "+match[2]+" "+unit)
	}
	if match := wireBeginPattern.FindStringSubmatch(source); len(match) == 3 {
		for index, role := range []string{"SDA", "SCL"} {
			gpio, symbol, ok := resolvePin(match[index+1], constants)
			if !ok {
				continue
			}
			pin := ensurePin(pins, gpio, symbol)
			pin.direction = "bidirectional"
			pin.behavior = "i2c_" + strings.ToLower(role)
			pin.evidence = appendUnique(pin.evidence, "Wire.begin "+role)
		}
	}
	for symbol, gpio := range constants {
		if looksLikePin(symbol) {
			ensurePin(pins, gpio, symbol).evidence = appendUnique(ensurePin(pins, gpio, symbol).evidence, "constant assignment")
		}
	}
	result.Pins = finalizePins(pins)
	if len(result.Pins) == 0 {
		result.Status = "NO_PINS_FOUND"
		result.Warnings = append(result.Warnings, "No supported GPIO declarations or calls were found.")
	}
	return result
}

func analyzePython(result domain.CodeAnalysis, source string) domain.CodeAnalysis {
	constants := collectConstants(source, pythonConstant)
	pins := make(map[int]*mutablePin)
	for _, match := range pythonPinPattern.FindAllStringSubmatch(source, -1) {
		gpio, symbol, ok := resolvePin(match[1], constants)
		if !ok {
			continue
		}
		pin := ensurePin(pins, gpio, symbol)
		pin.direction = strings.ToLower(match[2])
		pin.behavior = "digital_" + pin.direction
		pin.evidence = appendUnique(pin.evidence, "machine.Pin "+match[2])
	}
	for _, match := range pythonPWMPattern.FindAllStringSubmatch(source, -1) {
		gpio, symbol, ok := resolvePin(match[1], constants)
		if !ok {
			continue
		}
		pin := ensurePin(pins, gpio, symbol)
		pin.direction = "output"
		pin.behavior = "pwm_output"
		pin.evidence = appendUnique(pin.evidence, "machine.PWM")
	}
	result.Pins = finalizePins(pins)
	if len(result.Pins) == 0 {
		result.Status = "NO_PINS_FOUND"
		result.Warnings = append(result.Warnings, "No supported MicroPython Pin or PWM declarations were found.")
	}
	return result
}

func collectConstants(source string, patterns ...*regexp.Regexp) map[string]int {
	values := make(map[string]int)
	for _, pattern := range patterns {
		for _, match := range pattern.FindAllStringSubmatch(source, -1) {
			value, err := strconv.Atoi(match[2])
			if err == nil {
				values[match[1]] = value
			}
		}
	}
	return values
}

func resolvePin(token string, constants map[string]int) (int, string, bool) {
	if value, err := strconv.Atoi(token); err == nil {
		return value, "GPIO_" + token, value >= 0 && value <= 99
	}
	value, ok := constants[token]
	return value, token, ok
}

func ensurePin(pins map[int]*mutablePin, gpio int, symbol string) *mutablePin {
	if pin, ok := pins[gpio]; ok {
		if strings.HasPrefix(pin.symbol, "GPIO_") && !strings.HasPrefix(symbol, "GPIO_") {
			pin.symbol = symbol
		}
		return pin
	}
	pin := &mutablePin{gpio: gpio, symbol: symbol, direction: "unknown", behavior: behaviorFromSymbol(symbol), confidence: 1}
	pins[gpio] = pin
	return pin
}

func finalizePins(pins map[int]*mutablePin) []domain.CodePinFinding {
	keys := make([]int, 0, len(pins))
	for gpio := range pins {
		keys = append(keys, gpio)
	}
	sort.Ints(keys)
	result := make([]domain.CodePinFinding, 0, len(keys))
	for _, gpio := range keys {
		pin := pins[gpio]
		if pin.behavior == "unknown" && pin.direction != "unknown" {
			pin.behavior = "digital_" + pin.direction
		}
		result = append(result, domain.CodePinFinding{GPIO: gpio, Symbol: pin.symbol, Direction: pin.direction, Behavior: pin.behavior, Confidence: pin.confidence, Source: domain.SourceCodeStaticAnalysis, Evidence: pin.evidence})
	}
	return result
}

func languageFor(filename string) string {
	switch strings.ToLower(filepath.Ext(filename)) {
	case ".ino":
		return "arduino-cpp"
	case ".c":
		return "c"
	case ".cpp", ".h", ".hpp":
		return "cpp"
	case ".py":
		return "python"
	case ".txt", "":
		return "text"
	default:
		return "unsupported"
	}
}

func libraryName(include string) string {
	base := filepath.Base(include)
	return strings.TrimSuffix(strings.TrimSuffix(base, filepath.Ext(base)), ".h")
}

func looksLikePin(symbol string) bool {
	lower := strings.ToLower(symbol)
	for _, token := range []string{"pin", "trig", "echo", "servo", "sda", "scl", "led", "button", "sensor", "motor", "tx", "rx", "pwm"} {
		if strings.Contains(lower, token) {
			return true
		}
	}
	return false
}

func behaviorFromSymbol(symbol string) string {
	lower := strings.ToLower(symbol)
	switch {
	case strings.Contains(lower, "echo"):
		return "pulse_input"
	case strings.Contains(lower, "trig"):
		return "digital_pulse"
	case strings.Contains(lower, "pwm") || strings.Contains(lower, "servo") || strings.Contains(lower, "motor"):
		return "pwm_output"
	case strings.Contains(lower, "sda"):
		return "i2c_sda"
	case strings.Contains(lower, "scl"):
		return "i2c_scl"
	case strings.Contains(lower, "button"):
		return "digital_input"
	case strings.Contains(lower, "led"):
		return "digital_output"
	default:
		return "unknown"
	}
}

func preferDirection(current, candidate string) string {
	if current == "unknown" || current == candidate {
		return candidate
	}
	return current
}

func preferBehavior(current, candidate string) string {
	if current == "unknown" || current == candidate {
		return candidate
	}
	if strings.Contains(candidate, "pulse") || strings.Contains(candidate, "pwm") {
		return candidate
	}
	return current
}

func appendUnique(values []string, value string) []string {
	for _, existing := range values {
		if existing == value {
			return values
		}
	}
	return append(values, value)
}
