package componentcatalog

import "testing"

func TestCatalogContainsRequiredEntriesWithProvenance(t *testing.T) {
	entries, err := Load()
	if err != nil {
		t.Fatal(err)
	}
	for _, id := range []string{"hc-sr04", "sg90-servo", "led", "push-button", "generic-digital-input", "generic-digital-output", "pwm-output", "i2c-device", "uart-device"} {
		entry, ok := Find(entries, id)
		if !ok || len(entry.Sources) == 0 || entry.InterfaceType == "" {
			t.Fatalf("catalog entry %q missing or incomplete: %#v", id, entry)
		}
	}
}
