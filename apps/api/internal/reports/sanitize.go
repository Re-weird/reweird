package reports

import (
	"encoding/json"
	"os"
	"regexp"
	"strings"
)

var secretPatterns = []*regexp.Regexp{
	regexp.MustCompile(`(?is)-----BEGIN [A-Z ]*PRIVATE KEY-----.*?-----END [A-Z ]*PRIVATE KEY-----`),
	regexp.MustCompile(`\bsk-[A-Za-z0-9_-]{16,}\b`),
	regexp.MustCompile(`\bAIza[A-Za-z0-9_-]{20,}\b`),
	regexp.MustCompile(`\bgh[pousr]_[A-Za-z0-9]{20,}\b`),
	regexp.MustCompile(`(?i)\bBearer\s+[A-Za-z0-9._~+/=-]{8,}`),
	regexp.MustCompile(`(?i)\b(?:[A-Z][A-Z0-9_]*_)?(?:API[_-]?KEY|TOKEN|PASSWORD|SECRET)\b\s*[:=]\s*["']?[^"'\s,;]+`),
	regexp.MustCompile(`(?i)https?://[^/@\s]+:[^/@\s]+@`),
}

func ScanSecrets(value string) int {
	count := 0
	for _, pattern := range secretPatterns {
		count += len(pattern.FindAllString(value, -1))
	}
	return count
}

func SanitizeText(value string) (string, int) {
	count := 0
	for _, entry := range os.Environ() {
		parts := strings.SplitN(entry, "=", 2)
		if len(parts) != 2 || len(parts[1]) < 8 {
			continue
		}
		key := strings.ToUpper(parts[0])
		if !strings.Contains(key, "KEY") && !strings.Contains(key, "TOKEN") && !strings.Contains(key, "SECRET") && !strings.Contains(key, "PASSWORD") && !strings.Contains(key, "CREDENTIAL") {
			continue
		}
		if strings.Contains(value, parts[1]) {
			count += strings.Count(value, parts[1])
			value = strings.ReplaceAll(value, parts[1], "[REDACTED_SECRET]")
		}
	}
	for _, pattern := range secretPatterns {
		matches := pattern.FindAllString(value, -1)
		count += len(matches)
		value = pattern.ReplaceAllString(value, "[REDACTED_SECRET]")
	}
	return value, count
}

func Sanitize(report DetailedReport) (DetailedReport, int, error) {
	encoded, err := json.Marshal(report)
	if err != nil {
		return DetailedReport{}, 0, err
	}
	var payload any
	if err := json.Unmarshal(encoded, &payload); err != nil {
		return DetailedReport{}, 0, err
	}
	count := 0
	var walk func(any) any
	walk = func(value any) any {
		switch item := value.(type) {
		case string:
			clean, found := SanitizeText(item)
			count += found
			return clean
		case []any:
			for index := range item {
				item[index] = walk(item[index])
			}
			return item
		case map[string]any:
			for key, nested := range item {
				item[key] = walk(nested)
			}
			return item
		default:
			return value
		}
	}
	payload = walk(payload)
	encoded, err = json.Marshal(payload)
	if err != nil {
		return DetailedReport{}, 0, err
	}
	var sanitized DetailedReport
	if err := json.Unmarshal(encoded, &sanitized); err != nil {
		return DetailedReport{}, 0, err
	}
	return sanitized, count, nil
}
