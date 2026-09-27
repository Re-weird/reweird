package main

import (
	"context"
	"fmt"
	"log"
	"net"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"time"

	"github.com/joho/godotenv"
	"github.com/re-weird/reweird/apps/api/internal/codeanalysis"
	"github.com/re-weird/reweird/apps/api/internal/demodata"
	"github.com/re-weird/reweird/apps/api/internal/diagnostics"
	"github.com/re-weird/reweird/apps/api/internal/domain"
	"github.com/re-weird/reweird/apps/api/internal/githubapp"
	"github.com/re-weird/reweird/apps/api/internal/httpapi"
	"github.com/re-weird/reweird/apps/api/internal/probe"
	"github.com/re-weird/reweird/apps/api/internal/productdata"
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
	loadLocalDotEnv()

	databasePath := environment("DATABASE_PATH", "./reweird.db")
	// Railway (and most PaaS hosts) assign a dynamic port via $PORT and expect
	// the app to bind it directly; API_PORT remains the override for local/
	// self-hosted runs that don't set PORT.
	port := environment("PORT", environment("API_PORT", "8080"))
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
	// The Physical Git demo is optional, judge-facing convenience, not core
	// functionality -- a failure here (e.g. this dev database's unrelated
	// measurement-retention limit) must never take down the whole API.
	if err := demodata.Seed(repository, uploadRoot); err != nil {
		log.Printf("seed Physical Git demo: %v (Physical Git demo project unavailable this run)", err)
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
	githubConfig, githubConfigured, err := githubapp.ConfigFromEnv()
	if err != nil {
		log.Fatalf("configure GitHub App: %v", err)
	}
	var githubClient *githubapp.Client
	if githubConfigured {
		githubClient = githubapp.New(githubConfig, nil)
	}
	pollSeconds, err := strconv.Atoi(environment("GITHUB_POLL_SECONDS", "60"))
	if err != nil || pollSeconds < 0 {
		log.Fatalf("GITHUB_POLL_SECONDS must be a whole number of seconds (0 disables polling)")
	}
	if pollSeconds > 0 && pollSeconds < 15 {
		pollSeconds = 15
	}

	product, closeProduct := connectProductData(ctx)
	defer closeProduct()

	app := httpapi.NewApp(engine, repository, source, profileID, httpapi.ProjectServices{
		Understanding: understanding, UploadRoot: uploadRoot,
		GitHub: githubClient, PollInterval: time.Duration(pollSeconds) * time.Second,
		Product: product, Catalog: catalog,
	}, authTokenSecret != "")
	log.Printf("GitHub App: %s", map[bool]string{true: "configured", false: "not configured"}[githubConfigured])

	log.Printf("ReWeird API listening on %s (telemetry=%s, profile=%s, auth=%s, product_data=%s, PATCH=locked)", net.JoinHostPort(host, port), source.Name(), profileID, authStatus(authTokenSecret != ""), productDataStatus(product != nil))
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

// loadLocalDotEnv is a local-development convenience only: it searches
// upward from the current working directory for the repository's canonical
// .env file (reweird/.env) and loads any keys from it into the process
// environment, so `go run ./cmd/server` works identically whether launched
// from the repository root or from apps/api. Production deployments are
// unaffected either way -- they inject real environment variables directly
// and never rely on a .env file being present.
//
// godotenv.Load only sets a key if it is not already present in the
// process environment (see joho/godotenv's documented behavior), so a real,
// already-exported OS environment variable always takes precedence over
// whatever the .env file says -- this must never let a stale local .env
// value override production configuration.
func loadLocalDotEnv() {
	dir, err := os.Getwd()
	if err != nil {
		return
	}
	for depth := 0; depth < 8; depth++ {
		candidate := filepath.Join(dir, ".env")
		if _, statErr := os.Stat(candidate); statErr == nil {
			// Only the path is logged, never the file's contents -- it
			// carries real database credentials.
			if loadErr := godotenv.Load(candidate); loadErr != nil {
				log.Printf("warning: found %s but failed to load it: %v", candidate, loadErr)
			} else {
				log.Printf("loaded local development environment from %s", candidate)
			}
			return
		}
		parent := filepath.Dir(dir)
		if parent == dir {
			return
		}
		dir = parent
	}
}

func environment(key, fallback string) string {
	if value := os.Getenv(key); value != "" {
		return value
	}
	return fallback
}
