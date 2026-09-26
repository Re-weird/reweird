//go:build !windows

package computer

import (
	"context"
	"errors"
)

type WindowsCollector struct{}

func (WindowsCollector) Name() string { return "unavailable" }
func (WindowsCollector) Collect(_ context.Context, _ Expectations) (Snapshot, error) {
	return Snapshot{}, errors.New("the read-only real collector currently supports Windows only")
}
