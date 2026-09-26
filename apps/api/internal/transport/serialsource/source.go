package serialsource

import (
	"bufio"
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"sync"
	"time"

	"github.com/re-weird/reweird/apps/api/internal/domain"
	"github.com/re-weird/reweird/apps/api/internal/telemetry"
	serial "go.bug.st/serial"
)

const maxFrameBytes = 256 * 1024

var ErrNoTelemetry = errors.New("no valid ESP32 telemetry has been received")

type Source struct {
	mu         sync.RWMutex
	port       io.ReadCloser
	latest     *domain.TelemetryEnvelope
	lastError  error
	receivedAt time.Time
	updates    chan struct{}
}

type Status struct {
	Connected  bool      `json:"connected"`
	DeviceID   string    `json:"device_id,omitempty"`
	ProfileID  string    `json:"profile_id,omitempty"`
	Sequence   uint64    `json:"sequence,omitempty"`
	ReceivedAt time.Time `json:"received_at,omitempty"`
	LastError  string    `json:"last_error,omitempty"`
}

func Open(portName string, baud int) (*Source, error) {
	if portName == "" {
		return nil, errors.New("SERIAL_PORT is required in serial telemetry mode")
	}
	if baud < 1_200 || baud > 2_000_000 {
		return nil, errors.New("SERIAL_BAUD must be between 1200 and 2000000")
	}
	port, err := serial.Open(portName, &serial.Mode{BaudRate: baud})
	if err != nil {
		return nil, fmt.Errorf("open serial port %s: %w", portName, err)
	}
	return New(port), nil
}

func New(port io.ReadCloser) *Source {
	return &Source{port: port, updates: make(chan struct{}, 1)}
}

func (source *Source) Name() string { return "serial" }

func (source *Source) Run(ctx context.Context) error {
	if source.port == nil {
		return errors.New("serial source has no port")
	}
	go func() {
		<-ctx.Done()
		_ = source.port.Close()
	}()

	scanner := bufio.NewScanner(source.port)
	scanner.Buffer(make([]byte, 4096), maxFrameBytes)
	for scanner.Scan() {
		envelope, err := DecodeLine(scanner.Bytes())
		source.mu.Lock()
		if err == nil && source.latest != nil && envelope.DeviceID != source.latest.DeviceID {
			err = fmt.Errorf("device_id changed from %s to %s on an active serial stream", source.latest.DeviceID, envelope.DeviceID)
		}
		if err == nil && source.latest != nil && envelope.ProfileID != source.latest.ProfileID {
			err = fmt.Errorf("profile_id changed from %s to %s on an active serial stream", source.latest.ProfileID, envelope.ProfileID)
		}
		if err == nil && source.latest != nil && envelope.Sequence <= source.latest.Sequence {
			err = fmt.Errorf("non-increasing telemetry sequence %d after %d", envelope.Sequence, source.latest.Sequence)
		}
		if err != nil {
			source.lastError = err
		} else {
			copyOfEnvelope := envelope
			source.latest = &copyOfEnvelope
			source.lastError = nil
			source.receivedAt = time.Now().UTC()
		}
		source.mu.Unlock()
		if err == nil {
			select {
			case source.updates <- struct{}{}:
			default:
			}
		}
	}
	if ctx.Err() != nil {
		return nil
	}
	if err := scanner.Err(); err != nil {
		return fmt.Errorf("read serial telemetry: %w", err)
	}
	return errors.New("serial telemetry stream closed")
}

func (source *Source) Latest(_ context.Context) (domain.TelemetryEnvelope, error) {
	source.mu.RLock()
	defer source.mu.RUnlock()
	if source.latest == nil {
		if source.lastError != nil {
			return domain.TelemetryEnvelope{}, fmt.Errorf("%w: %v", ErrNoTelemetry, source.lastError)
		}
		return domain.TelemetryEnvelope{}, ErrNoTelemetry
	}
	maximumAge := 5 * time.Second
	windowAge := time.Duration(source.latest.WindowMS) * time.Millisecond * 3
	if windowAge > maximumAge {
		maximumAge = windowAge
	}
	if time.Since(source.receivedAt) > maximumAge {
		return domain.TelemetryEnvelope{}, fmt.Errorf("%w: last frame is stale", ErrNoTelemetry)
	}
	return *source.latest, nil
}

func (source *Source) WaitNext(ctx context.Context, afterSequence uint64) (domain.TelemetryEnvelope, error) {
	for {
		envelope, err := source.Latest(ctx)
		if err == nil && envelope.Sequence > afterSequence {
			return envelope, nil
		}
		select {
		case <-ctx.Done():
			return domain.TelemetryEnvelope{}, fmt.Errorf("wait for fresh serial telemetry: %w", ctx.Err())
		case <-source.updates:
		}
	}
}

func (source *Source) Status() Status {
	source.mu.RLock()
	defer source.mu.RUnlock()
	status := Status{Connected: source.latest != nil, ReceivedAt: source.receivedAt}
	if source.latest != nil {
		status.DeviceID = source.latest.DeviceID
		status.ProfileID = source.latest.ProfileID
		status.Sequence = source.latest.Sequence
	}
	if source.lastError != nil {
		status.LastError = source.lastError.Error()
	}
	return status
}

func (source *Source) Close() error {
	if source.port == nil {
		return nil
	}
	return source.port.Close()
}

func DecodeLine(line []byte) (domain.TelemetryEnvelope, error) {
	if len(line) == 0 {
		return domain.TelemetryEnvelope{}, errors.New("empty telemetry frame")
	}
	if len(line) > maxFrameBytes {
		return domain.TelemetryEnvelope{}, errors.New("telemetry frame exceeds 256 KiB")
	}
	var envelope domain.TelemetryEnvelope
	decoder := json.NewDecoder(bytes.NewReader(line))
	decoder.DisallowUnknownFields()
	if err := decoder.Decode(&envelope); err != nil {
		return domain.TelemetryEnvelope{}, fmt.Errorf("decode telemetry JSON: %w", err)
	}
	var trailing any
	if err := decoder.Decode(&trailing); err != io.EOF {
		return domain.TelemetryEnvelope{}, errors.New("telemetry frame must contain exactly one JSON object")
	}
	if err := telemetry.Validate(envelope); err != nil {
		return domain.TelemetryEnvelope{}, fmt.Errorf("validate telemetry: %w", err)
	}
	return envelope, nil
}
