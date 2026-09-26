package main

import (
	"log"
	"os"

	"github.com/re-weird/reweird/apps/api/internal/diagnostics"
	"github.com/re-weird/reweird/apps/api/internal/httpapi"
	"github.com/re-weird/reweird/apps/api/internal/simulator"
	"github.com/re-weird/reweird/apps/api/internal/store"
)

func main() {
	databasePath := environment("DATABASE_PATH", "./reweird.db")
	port := environment("API_PORT", "8080")

	repository, err := store.Open(databasePath)
	if err != nil {
		log.Fatalf("open sqlite store: %v", err)
	}
	defer repository.Close()

	telemetry := simulator.NewUltrasonicSource()
	engine := diagnostics.NewEngine(telemetry)
	app := httpapi.NewApp(engine, repository)

	log.Printf("ReWeird API listening on http://localhost:%s", port)
	if err := app.Listen(":" + port); err != nil {
		log.Fatal(err)
	}
}

func environment(key, fallback string) string {
	if value := os.Getenv(key); value != "" {
		return value
	}
	return fallback
}
