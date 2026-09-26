//go:build windows

package computer

import (
	"context"
	"os"
	"testing"
)

func TestOptInWindowsCollectorNormalizesToAnalysisContract(t *testing.T) {
	if os.Getenv("REWEIRD_TEST_REAL_COLLECTOR") != "true" {
		t.Skip("set REWEIRD_TEST_REAL_COLLECTOR=true to exercise local read-only collection")
	}
	snapshot, err := (WindowsCollector{}).Collect(context.Background(), Expectations{})
	if err != nil {
		t.Fatal(err)
	}
	if snapshot.SchemaVersion != SchemaVersion || snapshot.Source != "windows-read-only" || snapshot.System.OS == "" {
		t.Fatalf("invalid normalized snapshot source/version")
	}
	analysis, err := Analyze(snapshot, Expectations{})
	if err != nil {
		t.Fatal(err)
	}
	if len(analysis.Evidence.PhysicalEvidence) != 0 {
		t.Fatal("local computer evidence crossed physical domain")
	}
}
