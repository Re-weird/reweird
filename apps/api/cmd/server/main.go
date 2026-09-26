package main

import (
	"context"
	"fmt"
	"log"
	"os"
	"strconv"
	"strings"

	"github.com/re-weird/reweird/apps/api/internal/codeanalysis"
	"github.com/re-weird/reweird/apps/api/internal/diagnostics"
	"github.com/re-weird/reweird/apps/api/internal/domain"
	"github.com/re-weird/reweird/apps/api/internal/httpapi"
	"github.com/re-weird/reweird/apps/api/internal/probe"
	"github.com/re-weird/reweird/apps/api/internal/profiles"
	"github.com/re-weird/reweird/apps/api/internal/projectunderstanding"
	"github.com/re-weird/reweird/apps/api/internal/signalanalysis"
	"github.com/re-weird/reweird/apps/api/internal/simulator"
	"github.com/re-weird/reweird/apps/api/internal/store"
	"github.com/re-weird/reweird/apps/api/internal/transport/serialsource"
	"github.com/re-weird/reweird/apps/api/internal/vision"
	componentcatalog "github.com/re-weird/reweird/packages/component-catalog"
)

func main() {
	databasePath := environment("DATABASE_PATH", "./reweird.db")
	port := environment("API_PORT", "8080")
	telemetryMode := strings.ToLower(environment("TELEMETRY_MODE", "simulator"))
	profileID := environment("PROJECT_PROFILE_ID", "ultrasonic-demo")
	uploadRoot := environment("UPLOAD_DIR", "./data/uploads")

	repository, err := store.Open(databasePath)
	if err != nil {
		log.Fatalf("open sqlite store: %v", err)
	}
	defer repository.Close()

	if err := seedDemoProfile(repository); err != nil {
		log.Fatalf("seed demo Project Profile: %v", err)
	}
	activeProfile, err := repository.GetProfile(profileID)
	if err != nil {
		log.Fatalf("load active Project Profile: %v", err)
	}
	if activeProfile == nil {
		log.Fatalf("Project Profile %q does not exist", profileID)
	}
	if !activeProfile.Confirmed {
		log.Fatalf("Project Profile %q must be user-confirmed before telemetry starts", profileID)
	}

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	source, err := telemetrySource(ctx, telemetryMode)
	if err != nil {
		log.Fatalf("configure telemetry: %v", err)
	}
	if closer, ok := source.(interface{ Close() error }); ok {
		defer closer.Close()
	}

	geminiAPIKey := os.Getenv("GEMINI_API_KEY")
	geminiModel := environment("GEMINI_MODEL", "gemini-3.5-flash-lite")
	engine := diagnostics.NewEngineWithProbe(signalanalysis.New(), probe.NewService(os.Getenv("PROBE_SERVICE_URL")))
	catalog, err := componentcatalog.Load()
	if err != nil {
		log.Fatalf("load component catalog: %v", err)
	}
	understanding := projectunderstanding.New(
		codeanalysis.New(),
		vision.NewGemini(geminiAPIKey, geminiModel),
		catalog,
	)
	app := httpapi.NewApp(engine, repository, source, profileID, httpapi.ProjectServices{Understanding: understanding, UploadRoot: uploadRoot})

	log.Printf("ReWeird API listening on http://localhost:%s (telemetry=%s, profile=%s, PATCH=locked)", port, source.Name(), profileID)
	if err := app.Listen(":" + port); err != nil {
		log.Fatal(err)
	}
}

func telemetrySource(ctx context.Context, mode string) (domain.TelemetrySource, error) {
	switch mode {
	case "simulator":
		return simulator.NewUltrasonicSource(), nil
	case "serial":
		baud, err := strconv.Atoi(environment("SERIAL_BAUD", "115200"))
		if err != nil {
			return nil, fmt.Errorf("SERIAL_BAUD must be a number: %w", err)
		}
		source, err := serialsource.Open(os.Getenv("SERIAL_PORT"), baud)
		if err != nil {
			return nil, err
		}
		go func() {
			if err := source.Run(ctx); err != nil {
				log.Printf("serial telemetry stopped: %v", err)
			}
		}()
		return source, nil
	default:
		return nil, fmt.Errorf("unsupported TELEMETRY_MODE %q (use simulator or serial)", mode)
	}
}

func seedDemoProfile(repository domain.Repository) error {
	existing, err := repository.GetProfile("ultrasonic-demo")
	if err != nil {
		return err
	}
	if existing != nil {
		return nil
	}
	profile := profiles.UltrasonicDemo()
	if err := profiles.Validate(profile); err != nil {
		return err
	}
	return repository.SaveProfile(profile)
}

func environment(key, fallback string) string {
	if value := os.Getenv(key); value != "" {
		return value
	}
	return fallback
}
