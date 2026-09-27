package domain

import "context"

// TelemetryQuery scopes a Tiger Data telemetry read. Limit is always
// clamped by the implementation to a sane maximum -- callers must never be
// able to request an unbounded result set.
type TelemetryQuery struct {
	Probe     string // optional
	ProfileID string // optional
	SinceMS   *int64 // optional, inclusive
	UntilMS   *int64 // optional, inclusive
	Limit     int
}

// TelemetryRecord is one flattened, per-probe measurement row, mirroring
// the fields firmware already sends in TelemetrySample rather than
// inventing a new measurement vocabulary. A field is nil/omitted when this
// particular sample did not carry it (e.g. AvgAnalogMV is nil for a purely
// digital probe).
type TelemetryRecord struct {
	TimeMS          int64    `json:"time_ms"`
	DeviceID        string   `json:"device_id"`
	ProfileID       string   `json:"profile_id"`
	Source          string   `json:"source"`
	Probe           string   `json:"probe"`
	Mode            string   `json:"mode"`
	Sequence        uint64   `json:"sequence"`
	State           *int     `json:"state,omitempty"`
	EdgeCount       *uint32  `json:"edge_count,omitempty"`
	RisingEdges     *uint32  `json:"rising_edges,omitempty"`
	FallingEdges    *uint32  `json:"falling_edges,omitempty"`
	MaxGapUS        *uint64  `json:"max_gap_us,omitempty"`
	AvgAnalogMV     *float64 `json:"avg_analog_mv,omitempty"`
	AvgPeriodUS     *float64 `json:"avg_period_us,omitempty"`
	AvgPulseWidthUS *float64 `json:"avg_pulse_width_us,omitempty"`
}

// TelemetrySink is the Tiger Data (Postgres/Timescale-compatible)
// time-series telemetry store. It is a separate abstraction from
// MeasurementRepository (SQLite, which remains authoritative for the
// existing diagnostic loop) -- Insert is always a best-effort dual-write
// alongside SQLite, never a replacement for it, and never something the
// core diagnostic path depends on succeeding.
type TelemetrySink interface {
	Insert(ctx context.Context, window MeasurementWindow) error
	Query(ctx context.Context, query TelemetryQuery) ([]TelemetryRecord, error)
}

// FlattenMeasurementWindow turns one MeasurementWindow's samples into
// TelemetryRecord rows -- one row per probe per window, matching the shape
// firmware already sends (TelemetrySample) rather than inventing a new
// measurement vocabulary. It is pure and shared by every TelemetrySink
// implementation so "how a window becomes rows" only has one definition.
func FlattenMeasurementWindow(window MeasurementWindow) []TelemetryRecord {
	records := make([]TelemetryRecord, 0, len(window.Raw.Samples))
	for _, sample := range window.Raw.Samples {
		record := TelemetryRecord{
			TimeMS: window.CapturedAtMS, DeviceID: window.DeviceID, ProfileID: window.ProfileID,
			Source: window.Source, Probe: sample.Probe, Mode: string(sample.Mode), Sequence: window.Sequence,
		}
		if sample.State != nil {
			record.State = sample.State
		}
		if sample.EdgeCount != 0 {
			value := sample.EdgeCount
			record.EdgeCount = &value
		}
		if sample.RisingEdges != 0 {
			value := sample.RisingEdges
			record.RisingEdges = &value
		}
		if sample.FallingEdges != 0 {
			value := sample.FallingEdges
			record.FallingEdges = &value
		}
		if sample.MaxGapUS != 0 {
			value := sample.MaxGapUS
			record.MaxGapUS = &value
		}
		if average, ok := averageOf(sample.AnalogMV); ok {
			record.AvgAnalogMV = &average
		}
		if average, ok := averageOf(sample.PeriodsUS); ok {
			record.AvgPeriodUS = &average
		}
		if average, ok := averageOf(sample.HighPulseWidthsUS); ok {
			record.AvgPulseWidthUS = &average
		}
		records = append(records, record)
	}
	return records
}

func averageOf(values []float64) (float64, bool) {
	if len(values) == 0 {
		return 0, false
	}
	sum := 0.0
	for _, value := range values {
		sum += value
	}
	return sum / float64(len(values)), true
}
