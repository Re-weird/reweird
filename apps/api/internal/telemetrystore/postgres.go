// Package telemetrystore is the Tiger Data (PostgreSQL/Timescale-compatible)
// time-series telemetry sink (domain.TelemetrySink). It answers "what
// happened electrically over time?" -- a separate concern from both the
// SQLite-backed diagnostic loop (which remains authoritative and
// unaffected by this package's availability) and productdata's MongoDB
// account/equipment store.
package telemetrystore

import (
	"context"
	"fmt"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/re-weird/reweird/apps/api/internal/domain"
)

// MaxQueryLimit bounds every telemetry read, regardless of what a caller
// requests, so a single query can never pull an unbounded result set.
const MaxQueryLimit = 1000

// DefaultQueryLimit is used when a caller does not specify a limit.
const DefaultQueryLimit = 100

type Config struct {
	DatabaseURL string
}

func (config Config) Configured() bool {
	return config.DatabaseURL != ""
}

type Store struct {
	pool *pgxpool.Pool
}

func Connect(ctx context.Context, config Config) (*Store, error) {
	if !config.Configured() {
		return nil, fmt.Errorf("telemetrystore: TIGER_DATABASE_URL must be set")
	}
	pool, err := pgxpool.New(ctx, config.DatabaseURL)
	if err != nil {
		return nil, fmt.Errorf("telemetrystore: connect: %w", err)
	}
	pingCtx, cancel := context.WithTimeout(ctx, 10*time.Second)
	defer cancel()
	if err := pool.Ping(pingCtx); err != nil {
		pool.Close()
		return nil, fmt.Errorf("telemetrystore: ping: %w", err)
	}
	return &Store{pool: pool}, nil
}

func (store *Store) Close() {
	store.pool.Close()
}

// EnsureSchema creates the telemetry table and its indexes if they do not
// already exist, and opportunistically converts the table to a TimescaleDB
// hypertable when the timescaledb extension is available. A plain
// PostgreSQL deployment (no Timescale) is fully supported: the hypertable
// conversion failing is logged by the caller and never treated as fatal --
// this is an ordinary indexed table either way.
func (store *Store) EnsureSchema(ctx context.Context) error {
	_, err := store.pool.Exec(ctx, `
		CREATE TABLE IF NOT EXISTS telemetry_measurements (
			time              TIMESTAMPTZ      NOT NULL,
			device_id         TEXT             NOT NULL,
			profile_id        TEXT             NOT NULL,
			source            TEXT             NOT NULL,
			probe             TEXT             NOT NULL,
			mode              TEXT             NOT NULL,
			sequence          BIGINT           NOT NULL,
			state             INTEGER,
			edge_count        INTEGER,
			rising_edges      INTEGER,
			falling_edges     INTEGER,
			max_gap_us        BIGINT,
			avg_analog_mv     DOUBLE PRECISION,
			avg_period_us     DOUBLE PRECISION,
			avg_pulse_width_us DOUBLE PRECISION,
			ingested_at       TIMESTAMPTZ      NOT NULL DEFAULT now()
		)
	`)
	if err != nil {
		return fmt.Errorf("telemetrystore: create table: %w", err)
	}
	statements := []string{
		`CREATE INDEX IF NOT EXISTS telemetry_measurements_probe_time ON telemetry_measurements (probe, time DESC)`,
		`CREATE INDEX IF NOT EXISTS telemetry_measurements_profile_time ON telemetry_measurements (profile_id, time DESC)`,
		`CREATE INDEX IF NOT EXISTS telemetry_measurements_device_time ON telemetry_measurements (device_id, time DESC)`,
		`CREATE INDEX IF NOT EXISTS telemetry_measurements_time ON telemetry_measurements (time DESC)`,
	}
	for _, statement := range statements {
		if _, err := store.pool.Exec(ctx, statement); err != nil {
			return fmt.Errorf("telemetrystore: create index: %w", err)
		}
	}
	// Best-effort: only succeeds against a Timescale-enabled Postgres
	// (Tiger Data). Ignored on plain PostgreSQL, and ignored if the table
	// already is a hypertable.
	_, _ = store.pool.Exec(ctx, `SELECT create_hypertable('telemetry_measurements', 'time', if_not_exists => TRUE, migrate_data => TRUE)`)
	return nil
}

// Insert is always called as a best-effort dual-write after SQLite's own
// SaveMeasurement already succeeded -- a failure here is reported to the
// caller to log, but must never be treated as though the measurement was
// lost (SQLite already has it).
func (store *Store) Insert(ctx context.Context, window domain.MeasurementWindow) error {
	records := domain.FlattenMeasurementWindow(window)
	if len(records) == 0 {
		return nil
	}
	batch := &pgx.Batch{}
	for _, record := range records {
		batch.Queue(`
			INSERT INTO telemetry_measurements
				(time, device_id, profile_id, source, probe, mode, sequence, state, edge_count, rising_edges, falling_edges, max_gap_us, avg_analog_mv, avg_period_us, avg_pulse_width_us)
			VALUES
				(to_timestamp($1 / 1000.0), $2, $3, $4, $5, $6, $7, $8, $9, $10, $11, $12, $13, $14, $15)
		`, record.TimeMS, record.DeviceID, record.ProfileID, record.Source, record.Probe, record.Mode, int64(record.Sequence),
			record.State, record.EdgeCount, record.RisingEdges, record.FallingEdges, record.MaxGapUS,
			record.AvgAnalogMV, record.AvgPeriodUS, record.AvgPulseWidthUS)
	}
	results := store.pool.SendBatch(ctx, batch)
	defer results.Close()
	for range records {
		if _, err := results.Exec(); err != nil {
			return fmt.Errorf("telemetrystore: insert: %w", err)
		}
	}
	return nil
}

func (store *Store) Query(ctx context.Context, query domain.TelemetryQuery) ([]domain.TelemetryRecord, error) {
	limit := query.Limit
	if limit <= 0 {
		limit = DefaultQueryLimit
	}
	if limit > MaxQueryLimit {
		limit = MaxQueryLimit
	}
	sql := `
		SELECT extract(epoch FROM time) * 1000, device_id, profile_id, source, probe, mode, sequence,
		       state, edge_count, rising_edges, falling_edges, max_gap_us, avg_analog_mv, avg_period_us, avg_pulse_width_us
		FROM telemetry_measurements
		WHERE ($1 = '' OR probe = $1)
		  AND ($2 = '' OR profile_id = $2)
		  AND ($3::bigint IS NULL OR time >= to_timestamp($3 / 1000.0))
		  AND ($4::bigint IS NULL OR time <= to_timestamp($4 / 1000.0))
		ORDER BY time DESC
		LIMIT $5
	`
	rows, err := store.pool.Query(ctx, sql, query.Probe, query.ProfileID, query.SinceMS, query.UntilMS, limit)
	if err != nil {
		return nil, fmt.Errorf("telemetrystore: query: %w", err)
	}
	defer rows.Close()

	records := make([]domain.TelemetryRecord, 0, limit)
	for rows.Next() {
		var record domain.TelemetryRecord
		var timeMS float64
		var sequence int64
		if err := rows.Scan(&timeMS, &record.DeviceID, &record.ProfileID, &record.Source, &record.Probe, &record.Mode, &sequence,
			&record.State, &record.EdgeCount, &record.RisingEdges, &record.FallingEdges, &record.MaxGapUS,
			&record.AvgAnalogMV, &record.AvgPeriodUS, &record.AvgPulseWidthUS); err != nil {
			return nil, fmt.Errorf("telemetrystore: scan: %w", err)
		}
		record.TimeMS = int64(timeMS)
		record.Sequence = uint64(sequence)
		records = append(records, record)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("telemetrystore: rows: %w", err)
	}
	return records, nil
}
