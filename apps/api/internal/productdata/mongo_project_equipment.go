package productdata

import (
	"context"
	"fmt"

	"github.com/re-weird/reweird/apps/api/internal/domain"
	"go.mongodb.org/mongo-driver/v2/bson"
	"go.mongodb.org/mongo-driver/v2/mongo"
)

// AttachProjectEquipment returns domain.ErrConflict if this exact
// (project_id, equipment_id) pair is already attached, relying on the
// unique index EnsureIndexes creates rather than a separate read-then-write
// existence check that could race.
func (store *MongoStore) AttachProjectEquipment(ctx context.Context, link domain.ProjectEquipment) (domain.ProjectEquipment, error) {
	if link.CreatedAtMS == 0 {
		link.CreatedAtMS = nowMS()
	}
	if _, err := store.projectEquipment().InsertOne(ctx, link); err != nil {
		if mongo.IsDuplicateKeyError(err) {
			return domain.ProjectEquipment{}, domain.ErrConflict
		}
		return domain.ProjectEquipment{}, fmt.Errorf("productdata: attach project equipment: %w", err)
	}
	return link, nil
}

func (store *MongoStore) ListProjectEquipment(ctx context.Context, ownerID, projectID string) ([]domain.ProjectEquipment, error) {
	cursor, err := store.projectEquipment().Find(ctx, bson.D{{Key: "owner_id", Value: ownerID}, {Key: "project_id", Value: projectID}})
	if err != nil {
		return nil, fmt.Errorf("productdata: list project equipment: %w", err)
	}
	defer cursor.Close(ctx)
	items := make([]domain.ProjectEquipment, 0)
	if err := cursor.All(ctx, &items); err != nil {
		return nil, fmt.Errorf("productdata: decode project equipment list: %w", err)
	}
	return items, nil
}

func (store *MongoStore) DetachProjectEquipment(ctx context.Context, ownerID, projectID, equipmentID string) error {
	filter := bson.D{{Key: "owner_id", Value: ownerID}, {Key: "project_id", Value: projectID}, {Key: "equipment_id", Value: equipmentID}}
	result, err := store.projectEquipment().DeleteOne(ctx, filter)
	if err != nil {
		return fmt.Errorf("productdata: detach project equipment: %w", err)
	}
	if result.DeletedCount == 0 {
		return domain.ErrNotFound
	}
	return nil
}
