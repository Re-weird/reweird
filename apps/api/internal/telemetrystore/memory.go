package telemetrystore

import (
	"context"
	"sort"
	"sync"

	"github.com/re-weird/reweird/apps/api/internal/domain"
)

// MemoryStore is an in-process, non-persistent domain.TelemetrySink with
// the same query/limit semantics as Store, so httpapi tests (and anything
// else needing a TelemetrySink) can run fully offline without a real
// Postgres/Tiger Data deployment.
type MemoryStore struct {
	mu      sync.Mutex
	records []domain.TelemetryRecord
}

func NewMemoryStore() *MemoryStore {
	return &MemoryStore{}
}

func (store *MemoryStore) Insert(_ context.Context, window domain.MeasurementWindow) error {
	store.mu.Lock()
	defer store.mu.Unlock()
	store.records = append(store.records, domain.FlattenMeasurementWindow(window)...)
	return nil
}

func (store *MemoryStore) Query(_ context.Context, query domain.TelemetryQuery) ([]domain.TelemetryRecord, error) {
	store.mu.Lock()
	defer store.mu.Unlock()
	limit := query.Limit
	if limit <= 0 {
		limit = DefaultQueryLimit
	}
	if limit > MaxQueryLimit {
		limit = MaxQueryLimit
	}
	matches := make([]domain.TelemetryRecord, 0, len(store.records))
	for _, record := range store.records {
		if query.Probe != "" && record.Probe != query.Probe {
			continue
		}
		if query.ProfileID != "" && record.ProfileID != query.ProfileID {
			continue
		}
		if query.SinceMS != nil && record.TimeMS < *query.SinceMS {
			continue
		}
		if query.UntilMS != nil && record.TimeMS > *query.UntilMS {
			continue
		}
		matches = append(matches, record)
	}
	sort.SliceStable(matches, func(i, j int) bool { return matches[i].TimeMS > matches[j].TimeMS })
	if len(matches) > limit {
		matches = matches[:limit]
	}
	return matches, nil
}
