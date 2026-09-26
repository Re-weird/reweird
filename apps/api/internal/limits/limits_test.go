package limits

import "testing"

func TestBoundedConfigurationCannotRaiseHardCap(t *testing.T) {
	t.Setenv("REWEIRD_TEST_LIMIT", "999999")
	if got := Bounded("REWEIRD_TEST_LIMIT", 256, 8, 256); got != 256 {
		t.Fatalf("raised hard cap to %d", got)
	}
	t.Setenv("REWEIRD_TEST_LIMIT", "16")
	if got := Bounded("REWEIRD_TEST_LIMIT", 256, 8, 256); got != 16 {
		t.Fatalf("lowered cap = %d", got)
	}
	t.Setenv("REWEIRD_TEST_LIMIT", "-1")
	if got := Bounded("REWEIRD_TEST_LIMIT", 256, 8, 256); got != 256 {
		t.Fatalf("invalid cap = %d", got)
	}
}
