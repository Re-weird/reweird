package productdata

import (
	"context"
	"fmt"
	"sync"

	"github.com/re-weird/reweird/apps/api/internal/domain"
)

// MemoryStore is an in-process, non-persistent implementation of
// domain.ProductRepository with identical ownership/uniqueness semantics to
// MongoStore. It exists so httpapi tests (and anything else that needs a
// ProductRepository) can run fully offline, without a real MongoDB
// deployment -- production always uses MongoStore, constructed only when
// MONGODB_URI/MONGODB_DATABASE are configured.
type MemoryStore struct {
	mu               sync.Mutex
	usersByID        map[string]domain.User
	usersByIdentity  map[[2]string]string // (auth_provider, auth_subject) -> user id
	equipment        map[string]domain.Equipment
	projectEquipment map[[2]string]domain.ProjectEquipment // (project_id, equipment_id) -> link
	nextID           int
}

func NewMemoryStore() *MemoryStore {
	return &MemoryStore{
		usersByID:        make(map[string]domain.User),
		usersByIdentity:  make(map[[2]string]string),
		equipment:        make(map[string]domain.Equipment),
		projectEquipment: make(map[[2]string]domain.ProjectEquipment),
	}
}

func (store *MemoryStore) newID(prefix string) string {
	store.nextID++
	return fmt.Sprintf("%s-%d", prefix, store.nextID)
}

func (store *MemoryStore) ResolveUser(_ context.Context, authProvider, authSubject string, hints domain.UserProfileHints) (domain.User, error) {
	if authProvider == "" || authSubject == "" {
		return domain.User{}, fmt.Errorf("productdata: authProvider and authSubject are required")
	}
	store.mu.Lock()
	defer store.mu.Unlock()
	key := [2]string{authProvider, authSubject}
	if id, ok := store.usersByIdentity[key]; ok {
		user := store.usersByID[id]
		changed := false
		if user.Email == "" && hints.Email != "" {
			user.Email = hints.Email
			changed = true
		}
		if user.DisplayName == "" && hints.DisplayName != "" {
			user.DisplayName = hints.DisplayName
			changed = true
		}
		if user.AvatarURL == "" && hints.AvatarURL != "" {
			user.AvatarURL = hints.AvatarURL
			changed = true
		}
		if changed {
			user.UpdatedAtMS = nowMS()
			store.usersByID[id] = user
		}
		return user, nil
	}
	id := store.newID("user")
	now := nowMS()
	user := domain.User{
		ID: id, AuthProvider: authProvider, AuthSubject: authSubject,
		Email: hints.Email, DisplayName: hints.DisplayName, AvatarURL: hints.AvatarURL,
		CreatedAtMS: now, UpdatedAtMS: now,
	}
	store.usersByID[id] = user
	store.usersByIdentity[key] = id
	return user, nil
}

func (store *MemoryStore) GetUser(_ context.Context, id string) (*domain.User, error) {
	store.mu.Lock()
	defer store.mu.Unlock()
	if user, ok := store.usersByID[id]; ok {
		return &user, nil
	}
	return nil, nil
}

func (store *MemoryStore) CreateEquipment(_ context.Context, equipment domain.Equipment) (domain.Equipment, error) {
	store.mu.Lock()
	defer store.mu.Unlock()
	store.equipment[equipment.ID] = equipment
	return equipment, nil
}

func (store *MemoryStore) ListEquipment(_ context.Context, ownerID string) ([]domain.Equipment, error) {
	store.mu.Lock()
	defer store.mu.Unlock()
	items := make([]domain.Equipment, 0)
	for _, item := range store.equipment {
		if item.OwnerID == ownerID {
			items = append(items, item)
		}
	}
	return items, nil
}

func (store *MemoryStore) GetEquipment(_ context.Context, ownerID, id string) (*domain.Equipment, error) {
	store.mu.Lock()
	defer store.mu.Unlock()
	item, ok := store.equipment[id]
	if !ok || item.OwnerID != ownerID {
		return nil, nil
	}
	return &item, nil
}

func (store *MemoryStore) UpdateEquipment(_ context.Context, ownerID, id string, update domain.EquipmentUpdate) (*domain.Equipment, error) {
	store.mu.Lock()
	defer store.mu.Unlock()
	item, ok := store.equipment[id]
	if !ok || item.OwnerID != ownerID {
		return nil, nil
	}
	if update.Name != nil {
		item.Name = *update.Name
	}
	if update.Category != nil {
		item.Category = *update.Category
	}
	if update.Manufacturer != nil {
		item.Manufacturer = *update.Manufacturer
	}
	if update.Model != nil {
		item.Model = *update.Model
	}
	if update.SerialNumber != nil {
		item.SerialNumber = *update.SerialNumber
	}
	if update.ControllerType != nil {
		item.ControllerType = *update.ControllerType
	}
	if update.LogicVoltage != nil {
		item.LogicVoltage = update.LogicVoltage
	}
	if update.Notes != nil {
		item.Notes = *update.Notes
	}
	if update.CatalogIDSet {
		item.CatalogID = update.CatalogID
	}
	item.UpdatedAtMS = nowMS()
	store.equipment[id] = item
	return &item, nil
}

func (store *MemoryStore) DeleteEquipment(_ context.Context, ownerID, id string) error {
	store.mu.Lock()
	defer store.mu.Unlock()
	item, ok := store.equipment[id]
	if !ok || item.OwnerID != ownerID {
		return domain.ErrNotFound
	}
	delete(store.equipment, id)
	for key, link := range store.projectEquipment {
		if link.EquipmentID == id && link.OwnerID == ownerID {
			delete(store.projectEquipment, key)
		}
	}
	return nil
}

func (store *MemoryStore) AttachProjectEquipment(_ context.Context, link domain.ProjectEquipment) (domain.ProjectEquipment, error) {
	store.mu.Lock()
	defer store.mu.Unlock()
	key := [2]string{link.ProjectID, link.EquipmentID}
	if _, exists := store.projectEquipment[key]; exists {
		return domain.ProjectEquipment{}, domain.ErrConflict
	}
	if link.CreatedAtMS == 0 {
		link.CreatedAtMS = nowMS()
	}
	store.projectEquipment[key] = link
	return link, nil
}

func (store *MemoryStore) ListProjectEquipment(_ context.Context, ownerID, projectID string) ([]domain.ProjectEquipment, error) {
	store.mu.Lock()
	defer store.mu.Unlock()
	items := make([]domain.ProjectEquipment, 0)
	for _, link := range store.projectEquipment {
		if link.OwnerID == ownerID && link.ProjectID == projectID {
			items = append(items, link)
		}
	}
	return items, nil
}

func (store *MemoryStore) DetachProjectEquipment(_ context.Context, ownerID, projectID, equipmentID string) error {
	store.mu.Lock()
	defer store.mu.Unlock()
	key := [2]string{projectID, equipmentID}
	link, ok := store.projectEquipment[key]
	if !ok || link.OwnerID != ownerID {
		return domain.ErrNotFound
	}
	delete(store.projectEquipment, key)
	return nil
}
