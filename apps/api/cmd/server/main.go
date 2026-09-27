package main

import (
	"context"
	"fmt"
	"log"
	"net"
	"os"
	"strconv"
	"strings"
	"time"

	"github.com/re-weird/reweird/apps/api/internal/codeanalysis"
	"github.com/re-weird/reweird/apps/api/internal/diagnostics"
	"github.com/re-weird/reweird/apps/api/internal/domain"
	"github.com/re-weird/reweird/apps/api/internal/httpapi"
	"github.com/re-weird/reweird/apps/api/internal/probe"
	"github.com/re-weird/reweird/apps/api/internal/productdata"
	"github.com/re-weird/reweird/apps/api/internal/profiles"
	"github.com/re-weird/reweird/apps/api/internal/projectunderstanding"
	"github.com/re-weird/reweird/apps/api/internal/signalanalysis"
	"github.com/re-weird/reweird/apps/api/internal/simulator"
	"github.com/re-weird/reweird/apps/api/internal/store"
	"github.com/re-weird/reweird/apps/api/internal/telemetrystore"
	"github.com/re-weird/reweird/apps/api/internal/transport/serialsource"
	"github.com/re-weird/reweird/apps/api/internal/vision"
	componentcatalog "github.com/re-weird/reweird/packages/component-catalog"
)

func main() {
	databasePath := environment("DATABASE_PATH", "./reweird.db")
	port := environment("API_PORT", "8080")
	host := environment("API_HOST", "127.0.0.1")
	address := net.ParseIP(host)
	if address == nil {
		log.Fatalf("API_HOST must be an IP address, got %q", host)
	}
	if !address.IsLoopback() && os.Getenv("REWEIRD_API_TOKEN") == "" && os.Getenv("API_TRUSTED_NETWORK") != "true" {
		log.Fatal("non-loopback API_HOST requires REWEIRD_API_TOKEN or explicit API_TRUSTED_NETWORK=true")
	}
	if token := os.Getenv("REWEIRD_API_TOKEN"); token != "" && len(token) < 32 {
		log.Fatal("REWEIRD_API_TOKEN must contain at least 32 characters")
	}
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

	authTokenSecret := os.Getenv("AUTH_TOKEN_SECRET")

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

	product, closeProduct := connectProductData(ctx)
	defer closeProduct()

	tigerTelemetry, closeTiger := connectTigerTelemetry(ctx)
	defer closeTiger()

	app := httpapi.NewApp(engine, repository, source, profileID, httpapi.ProjectServices{
		Understanding: understanding, UploadRoot: uploadRoot,
		Product: product, Catalog: catalog, Telemetry: tigerTelemetry,
	}, authTokenSecret != "")

	log.Printf("ReWeird API listening on %s (telemetry=%s, profile=%s, auth=%s, product_data=%s, tiger_telemetry=%s, PATCH=locked)", net.JoinHostPort(host, port), source.Name(), profileID, authStatus(authTokenSecret != ""), productDataStatus(product != nil), tigerTelemetryStatus(tigerTelemetry != nil))
	if err := app.Listen(net.JoinHostPort(host, port)); err != nil {
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

// connectProductData connects to MongoDB when MONGODB_URI/MONGODB_DATABASE
// are both configured, and returns a no-op closer plus a nil
// domain.ProductRepository otherwise. MongoDB being unconfigured or
// unreachable must never take down the rest of the API -- every equipment/
// me handler already checks for a nil product repository and fails clearly
// (503 PRODUCT_DATA_UNAVAILABLE) instead.
func connectProductData(ctx context.Context) (domain.ProductRepository, func()) {
	config := productdata.Config{URI: os.Getenv("MONGODB_URI"), Database: environment("MONGODB_DATABASE", "reweird")}
	if os.Getenv("MONGODB_URI") == "" {
		log.Printf("MONGODB_URI not set; accounts/equipment (product data) are disabled for this run")
		return nil, func() {}
	}
	mongoStore, err := productdata.Connect(ctx, config)
	if err != nil {
		log.Printf("product data unavailable: %v (accounts/equipment endpoints will return 503)", err)
		return nil, func() {}
	}
	indexCtx, cancel := context.WithTimeout(ctx, 30*time.Second)
	defer cancel()
	if err := mongoStore.EnsureIndexes(indexCtx); err != nil {
		log.Printf("product data index setup failed: %v (accounts/equipment endpoints will return 503)", err)
		_ = mongoStore.Close(ctx)
		return nil, func() {}
	}
	return mongoStore, func() {
		closeCtx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()
		if err := mongoStore.Close(closeCtx); err != nil {
			log.Printf("error closing product data store: %v", err)
		}
	}
}

// connectTigerTelemetry connects to Tiger Data (PostgreSQL/Timescale-
// compatible) when TIGER_DATABASE_URL is configured, and returns a no-op
// closer plus a nil domain.TelemetrySink otherwise. Unconfigured or
// unreachable Tiger Data must never take down the rest of the API: the
// existing SQLite measurement path (and Demo Mode/the simulator) keeps
// working unchanged either way, and every telemetry-dependent code path
// already checks for nil.
func connectTigerTelemetry(ctx context.Context) (domain.TelemetrySink, func()) {
	config := telemetrystore.Config{DatabaseURL: os.Getenv("TIGER_DATABASE_URL")}
	if !config.Configured() {
		log.Printf("TIGER_DATABASE_URL not set; Tiger Data telemetry is disabled for this run (SQLite measurements are unaffected)")
		return nil, func() {}
	}
	tigerStore, err := telemetrystore.Connect(ctx, config)
	if err != nil {
		log.Printf("tiger telemetry unavailable: %v (measurements still save to sqlite; GET /api/v1/telemetry will return 503)", err)
		return nil, func() {}
	}
	schemaCtx, cancel := context.WithTimeout(ctx, 30*time.Second)
	defer cancel()
	if err := tigerStore.EnsureSchema(schemaCtx); err != nil {
		log.Printf("tiger telemetry schema setup failed: %v (measurements still save to sqlite; GET /api/v1/telemetry will return 503)", err)
		tigerStore.Close()
		return nil, func() {}
	}
	return tigerStore, tigerStore.Close
}

func tigerTelemetryStatus(configured bool) string {
	if configured {
		return "tiger"
	}
	return "disabled"
}

func productDataStatus(configured bool) string {
	if configured {
		return "mongodb"
	}
	return "disabled"
}

func authStatus(configured bool) string {
	if configured {
		return "google"
	}
	return "anonymous-only"
}

func environment(key, fallback string) string {
	if value := os.Getenv(key); value != "" {
		return value
	}
	return fallback
}
