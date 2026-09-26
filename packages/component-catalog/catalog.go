package componentcatalog

import (
	"embed"
	"encoding/json"
	"fmt"
	"path/filepath"
	"sort"
	"strings"
)

//go:embed *.json
var files embed.FS

type VoltageRange struct {
	Minimum *float64 `json:"minimum,omitempty"`
	Typical *float64 `json:"typical,omitempty"`
	Maximum *float64 `json:"maximum,omitempty"`
	Unit    string   `json:"unit"`
}

type PinRole struct {
	Name        string   `json:"name"`
	Direction   string   `json:"direction"`
	SignalTypes []string `json:"signal_types"`
	Description string   `json:"description"`
}

type Source struct {
	Title string `json:"title"`
	URL   string `json:"url"`
}

type Entry struct {
	ID               string        `json:"id"`
	Name             string        `json:"name"`
	Aliases          []string      `json:"aliases"`
	InterfaceType    string        `json:"interface_type"`
	OperatingVoltage *VoltageRange `json:"operating_voltage,omitempty"`
	LogicVoltage     *VoltageRange `json:"logic_voltage,omitempty"`
	PinRoles         []PinRole     `json:"pin_roles"`
	ExpectedBehavior string        `json:"expected_behavior"`
	SafeMeasurement  []string      `json:"safe_measurement_notes"`
	KnownSignalTypes []string      `json:"known_signal_types"`
	Sources          []Source      `json:"sources"`
}

func Load() ([]Entry, error) {
	names, err := files.ReadDir(".")
	if err != nil {
		return nil, err
	}
	entries := make([]Entry, 0, len(names))
	seen := make(map[string]bool)
	for _, name := range names {
		if name.IsDir() || filepath.Ext(name.Name()) != ".json" {
			continue
		}
		payload, err := files.ReadFile(name.Name())
		if err != nil {
			return nil, err
		}
		var entry Entry
		if err := json.Unmarshal(payload, &entry); err != nil {
			return nil, fmt.Errorf("decode %s: %w", name.Name(), err)
		}
		if entry.ID == "" || entry.Name == "" || len(entry.Sources) == 0 {
			return nil, fmt.Errorf("catalog entry %s is missing id, name, or provenance", name.Name())
		}
		if seen[entry.ID] {
			return nil, fmt.Errorf("duplicate catalog id %q", entry.ID)
		}
		seen[entry.ID] = true
		entries = append(entries, entry)
	}
	sort.Slice(entries, func(i, j int) bool { return entries[i].ID < entries[j].ID })
	return entries, nil
}

func Find(entries []Entry, value string) (Entry, bool) {
	wanted := normalize(value)
	for _, entry := range entries {
		if normalize(entry.ID) == wanted || normalize(entry.Name) == wanted {
			return entry, true
		}
		for _, alias := range entry.Aliases {
			if normalize(alias) == wanted {
				return entry, true
			}
		}
	}
	return Entry{}, false
}

func normalize(value string) string {
	return strings.Map(func(character rune) rune {
		if character >= 'a' && character <= 'z' || character >= '0' && character <= '9' {
			return character
		}
		if character >= 'A' && character <= 'Z' {
			return character + ('a' - 'A')
		}
		return -1
	}, value)
}
