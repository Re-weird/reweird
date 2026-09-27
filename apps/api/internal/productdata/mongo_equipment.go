package productdata

import (
	"context"
	"errors"
	"fmt"

	"github.com/re-weird/reweird/apps/api/internal/domain"
	"go.mongodb.org/mongo-driver/v2/bson"
	"go.mongodb.org/mongo-driver/v2/mongo"
)

func (store *MongoStore) CreateEquipment(ctx context.Context, equipment domain.Equipment) (domain.Equipment, error) {
	if _, err := store.equipment().InsertOne(ctx, equipment); err != nil {
		return domain.Equipment{}, fmt.Errorf("productdata: insert equipment: %w", err)
	}
	return equipment, nil
}

func (store *MongoStore) ListEquipment(ctx context.Context, ownerID string) ([]domain.Equipment, error) {
	cursor, err := store.equipment().Find(ctx, bson.D{{Key: "owner_id", Value: ownerID}})
	if err != nil {
		return nil, fmt.Errorf("productdata: list equipment: %w", err)
	}
	defer cursor.Close(ctx)
	items := make([]domain.Equipment, 0)
	if err := cursor.All(ctx, &items); err != nil {
		return nil, fmt.Errorf("productdata: decode equipment list: %w", err)
	}
	return items, nil
}

// GetEquipment returns (nil, nil) both when the id does not exist at all and
// when it exists but belongs to a different owner -- callers must not be
// able to distinguish "not yours" from "doesn't exist" (see domain.ErrNotFound).
func (store *MongoStore) GetEquipment(ctx context.Context, ownerID, id string) (*domain.Equipment, error) {
	var item domain.Equipment
	err := store.equipment().FindOne(ctx, bson.D{{Key: "_id", Value: id}}).Decode(&item)
	if errors.Is(err, mongo.ErrNoDocuments) {
		return nil, nil
	}
	if err != nil {
		return nil, fmt.Errorf("productdata: get equipment: %w", err)
	}
	if item.OwnerID != ownerID {
		return nil, nil
	}
	return &item, nil
}

func (store *MongoStore) UpdateEquipment(ctx context.Context, ownerID, id string, update domain.EquipmentUpdate) (*domain.Equipment, error) {
	existing, err := store.GetEquipment(ctx, ownerID, id)
	if err != nil || existing == nil {
		return existing, err
	}
	set := bson.M{}
	if update.Name != nil {
		set["name"] = *update.Name
	}
	if update.Category != nil {
		set["category"] = *update.Category
	}
	if update.Manufacturer != nil {
		set["manufacturer"] = *update.Manufacturer
	}
	if update.Model != nil {
		set["model"] = *update.Model
	}
	if update.SerialNumber != nil {
		set["serial_number"] = *update.SerialNumber
	}
	if update.ControllerType != nil {
		set["controller_type"] = *update.ControllerType
	}
	if update.LogicVoltage != nil {
		set["logic_voltage"] = *update.LogicVoltage
	}
	if update.Notes != nil {
		set["notes"] = *update.Notes
	}
	if update.CatalogIDSet {
		set["catalog_id"] = update.CatalogID
	}
	if len(set) == 0 {
		return existing, nil
	}
	set["updated_at_ms"] = nowMS()
	filter := bson.D{{Key: "_id", Value: id}, {Key: "owner_id", Value: ownerID}}
	if _, err := store.equipment().UpdateOne(ctx, filter, bson.M{"$set": set}); err != nil {
		return nil, fmt.Errorf("productdata: update equipment: %w", err)
	}
	return store.GetEquipment(ctx, ownerID, id)
}

// DeleteEquipment also cascades to remove any ProjectEquipment rows
// referencing this equipment, so deleting equipment can never leave a
// dangling attachment behind on a project.
func (store *MongoStore) DeleteEquipment(ctx context.Context, ownerID, id string) error {
	filter := bson.D{{Key: "_id", Value: id}, {Key: "owner_id", Value: ownerID}}
	result, err := store.equipment().DeleteOne(ctx, filter)
	if err != nil {
		return fmt.Errorf("productdata: delete equipment: %w", err)
	}
	if result.DeletedCount == 0 {
		return domain.ErrNotFound
	}
	if _, err := store.projectEquipment().DeleteMany(ctx, bson.D{{Key: "equipment_id", Value: id}, {Key: "owner_id", Value: ownerID}}); err != nil {
		return fmt.Errorf("productdata: cascade delete project_equipment: %w", err)
	}
	return nil
}
