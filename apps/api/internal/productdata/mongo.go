// Package productdata is the MongoDB-backed "product data" store: users,
// equipment, and project<->equipment relationships (domain.ProductRepository).
// It is deliberately separate from apps/api/internal/store (SQLite), which
// remains authoritative for projects/diagnostics/sessions/workflows -- this
// package answers "what does this user own?", not "what happened
// electrically?" (that remains SQLite's measurement_windows table, in
// apps/api/internal/store).
package productdata

import (
	"context"
	"fmt"
	"time"

	"go.mongodb.org/mongo-driver/v2/bson"
	"go.mongodb.org/mongo-driver/v2/mongo"
	"go.mongodb.org/mongo-driver/v2/mongo/options"
)

// Config configures the MongoDB connection. A zero-value Config (empty URI)
// means MongoDB is not configured; Connect refuses to run against it so
// callers fail closed and clearly rather than silently no-op-ing.
type Config struct {
	URI      string
	Database string
}

// Configured reports whether enough configuration was supplied to attempt a
// connection. It does not verify reachability.
func (config Config) Configured() bool {
	return config.URI != "" && config.Database != ""
}

// MongoStore implements domain.ProductRepository against a real MongoDB
// Atlas (or any MongoDB-compatible) deployment.
type MongoStore struct {
	client   *mongo.Client
	database *mongo.Database
}

// Connect dials MongoDB and verifies reachability with a bounded-timeout
// ping. It does not create indexes -- call EnsureIndexes once at startup
// after Connect succeeds.
func Connect(ctx context.Context, config Config) (*MongoStore, error) {
	if !config.Configured() {
		return nil, fmt.Errorf("productdata: MONGODB_URI and MONGODB_DATABASE must both be set")
	}
	client, err := mongo.Connect(options.Client().ApplyURI(config.URI))
	if err != nil {
		return nil, fmt.Errorf("productdata: connect: %w", err)
	}
	pingCtx, cancel := context.WithTimeout(ctx, 10*time.Second)
	defer cancel()
	if err := client.Ping(pingCtx, nil); err != nil {
		_ = client.Disconnect(ctx)
		return nil, fmt.Errorf("productdata: ping: %w", err)
	}
	return &MongoStore{client: client, database: client.Database(config.Database)}, nil
}

// Close disconnects the underlying client. Safe to call once during
// shutdown.
func (store *MongoStore) Close(ctx context.Context) error {
	return store.client.Disconnect(ctx)
}

func (store *MongoStore) users() *mongo.Collection {
	return store.database.Collection("users")
}

func (store *MongoStore) equipment() *mongo.Collection {
	return store.database.Collection("equipment")
}

func (store *MongoStore) projectEquipment() *mongo.Collection {
	return store.database.Collection("project_equipment")
}

// EnsureIndexes creates every index this milestone requires. It is
// idempotent (CreateOne on an already-existing equivalent index is a
// no-op), so it is safe to call on every startup rather than only once
// ever.
func (store *MongoStore) EnsureIndexes(ctx context.Context) error {
	if _, err := store.users().Indexes().CreateOne(ctx, mongo.IndexModel{
		Keys:    bson.D{{Key: "auth_provider", Value: 1}, {Key: "auth_subject", Value: 1}},
		Options: options.Index().SetUnique(true).SetName("auth_provider_auth_subject_unique"),
	}); err != nil {
		return fmt.Errorf("productdata: users index: %w", err)
	}
	if _, err := store.equipment().Indexes().CreateOne(ctx, mongo.IndexModel{
		Keys:    bson.D{{Key: "owner_id", Value: 1}},
		Options: options.Index().SetName("equipment_owner_id"),
	}); err != nil {
		return fmt.Errorf("productdata: equipment index: %w", err)
	}
	if _, err := store.projectEquipment().Indexes().CreateOne(ctx, mongo.IndexModel{
		Keys:    bson.D{{Key: "project_id", Value: 1}, {Key: "equipment_id", Value: 1}},
		Options: options.Index().SetUnique(true).SetName("project_equipment_pair_unique"),
	}); err != nil {
		return fmt.Errorf("productdata: project_equipment pair index: %w", err)
	}
	if _, err := store.projectEquipment().Indexes().CreateOne(ctx, mongo.IndexModel{
		Keys:    bson.D{{Key: "owner_id", Value: 1}, {Key: "project_id", Value: 1}},
		Options: options.Index().SetName("project_equipment_owner_project"),
	}); err != nil {
		return fmt.Errorf("productdata: project_equipment owner/project index: %w", err)
	}
	if _, err := store.projectEquipment().Indexes().CreateOne(ctx, mongo.IndexModel{
		Keys:    bson.D{{Key: "equipment_id", Value: 1}},
		Options: options.Index().SetName("project_equipment_equipment_id"),
	}); err != nil {
		return fmt.Errorf("productdata: project_equipment equipment_id index: %w", err)
	}
	return nil
}

func nowMS() int64 {
	return time.Now().UnixMilli()
}
