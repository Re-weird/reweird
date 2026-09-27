package productdata

import (
	"context"
	"testing"

	"github.com/re-weird/reweird/apps/api/internal/domain"
)

func TestResolveUserDoesNotDuplicateOnRepeatSignIn(t *testing.T) {
	store := NewMemoryStore()
	ctx := context.Background()

	first, err := store.ResolveUser(ctx, "google", "sub-1", domain.UserProfileHints{Email: "a@example.com"})
	if err != nil {
		t.Fatalf("ResolveUser() error = %v", err)
	}
	second, err := store.ResolveUser(ctx, "google", "sub-1", domain.UserProfileHints{Email: "different@example.com"})
	if err != nil {
		t.Fatalf("ResolveUser() error = %v", err)
	}
	if first.ID != second.ID {
		t.Fatalf("ResolveUser() created a second user: %q != %q", first.ID, second.ID)
	}
	// An already-populated field must never be overwritten by a later hint.
	if second.Email != "a@example.com" {
		t.Fatalf("Email = %q, want original %q preserved", second.Email, "a@example.com")
	}
}

func TestResolveUserFillsOnlyEmptyHintFields(t *testing.T) {
	store := NewMemoryStore()
	ctx := context.Background()

	_, err := store.ResolveUser(ctx, "google", "sub-2", domain.UserProfileHints{})
	if err != nil {
		t.Fatalf("ResolveUser() error = %v", err)
	}
	filled, err := store.ResolveUser(ctx, "google", "sub-2", domain.UserProfileHints{DisplayName: "Ada"})
	if err != nil {
		t.Fatalf("ResolveUser() error = %v", err)
	}
	if filled.DisplayName != "Ada" {
		t.Fatalf("DisplayName = %q, want %q", filled.DisplayName, "Ada")
	}
}

func TestEquipmentOwnershipIsolation(t *testing.T) {
	store := NewMemoryStore()
	ctx := context.Background()

	owned, err := store.CreateEquipment(ctx, domain.Equipment{ID: "eq-1", OwnerID: "user-a", Name: "Front sensor"})
	if err != nil {
		t.Fatalf("CreateEquipment() error = %v", err)
	}

	if got, err := store.GetEquipment(ctx, "user-a", owned.ID); err != nil || got == nil {
		t.Fatalf("GetEquipment(owner) = %v, %v; want the equipment", got, err)
	}
	if got, err := store.GetEquipment(ctx, "user-b", owned.ID); err != nil || got != nil {
		t.Fatalf("GetEquipment(other owner) = %v, %v; want nil, nil", got, err)
	}
	if _, err := store.UpdateEquipment(ctx, "user-b", owned.ID, domain.EquipmentUpdate{}); err != nil {
		t.Fatalf("UpdateEquipment(other owner) unexpected error = %v", err)
	}
	if updated, err := store.UpdateEquipment(ctx, "user-b", owned.ID, domain.EquipmentUpdate{}); err != nil || updated != nil {
		t.Fatalf("UpdateEquipment(other owner) = %v, %v; want nil, nil", updated, err)
	}
	if err := store.DeleteEquipment(ctx, "user-b", owned.ID); err != domain.ErrNotFound {
		t.Fatalf("DeleteEquipment(other owner) error = %v, want ErrNotFound", err)
	}
	list, err := store.ListEquipment(ctx, "user-b")
	if err != nil {
		t.Fatalf("ListEquipment() error = %v", err)
	}
	if len(list) != 0 {
		t.Fatalf("ListEquipment(other owner) = %v, want empty", list)
	}
}

func TestEquipmentUpdateAppliesOnlySuppliedFields(t *testing.T) {
	store := NewMemoryStore()
	ctx := context.Background()
	created, _ := store.CreateEquipment(ctx, domain.Equipment{ID: "eq-2", OwnerID: "user-a", Name: "Motor", Category: "actuator"})

	name := "Renamed Motor"
	updated, err := store.UpdateEquipment(ctx, "user-a", created.ID, domain.EquipmentUpdate{Name: &name})
	if err != nil {
		t.Fatalf("UpdateEquipment() error = %v", err)
	}
	if updated.Name != "Renamed Motor" || updated.Category != "actuator" {
		t.Fatalf("UpdateEquipment() = %+v, want name changed and category preserved", updated)
	}
}

func TestEquipmentCatalogIDCanBeClearedExplicitly(t *testing.T) {
	store := NewMemoryStore()
	ctx := context.Background()
	catalogID := "hc-sr04"
	created, _ := store.CreateEquipment(ctx, domain.Equipment{ID: "eq-3", OwnerID: "user-a", Name: "Sensor", CatalogID: &catalogID})

	updated, err := store.UpdateEquipment(ctx, "user-a", created.ID, domain.EquipmentUpdate{CatalogIDSet: true, CatalogID: nil})
	if err != nil {
		t.Fatalf("UpdateEquipment() error = %v", err)
	}
	if updated.CatalogID != nil {
		t.Fatalf("CatalogID = %v, want cleared to nil", updated.CatalogID)
	}
}

func TestProjectEquipmentAttachDetachAndDuplicateRejection(t *testing.T) {
	store := NewMemoryStore()
	ctx := context.Background()
	link := domain.ProjectEquipment{ProjectID: "proj-1", EquipmentID: "eq-1", OwnerID: "user-a", Role: "distance sensing"}

	if _, err := store.AttachProjectEquipment(ctx, link); err != nil {
		t.Fatalf("AttachProjectEquipment() error = %v", err)
	}
	if _, err := store.AttachProjectEquipment(ctx, link); err != domain.ErrConflict {
		t.Fatalf("AttachProjectEquipment() duplicate error = %v, want ErrConflict", err)
	}

	items, err := store.ListProjectEquipment(ctx, "user-a", "proj-1")
	if err != nil || len(items) != 1 {
		t.Fatalf("ListProjectEquipment() = %v, %v; want exactly one link", items, err)
	}
	// A different owner must never see this project's equipment, even if it
	// somehow guessed the project id.
	if items, err := store.ListProjectEquipment(ctx, "user-b", "proj-1"); err != nil || len(items) != 0 {
		t.Fatalf("ListProjectEquipment(other owner) = %v, %v; want empty", items, err)
	}

	if err := store.DetachProjectEquipment(ctx, "user-b", "proj-1", "eq-1"); err != domain.ErrNotFound {
		t.Fatalf("DetachProjectEquipment(other owner) error = %v, want ErrNotFound", err)
	}
	if err := store.DetachProjectEquipment(ctx, "user-a", "proj-1", "eq-1"); err != nil {
		t.Fatalf("DetachProjectEquipment() error = %v", err)
	}
	if err := store.DetachProjectEquipment(ctx, "user-a", "proj-1", "eq-1"); err != domain.ErrNotFound {
		t.Fatalf("DetachProjectEquipment() second call error = %v, want ErrNotFound", err)
	}
}

func TestDeleteEquipmentCascadesProjectEquipmentLinks(t *testing.T) {
	store := NewMemoryStore()
	ctx := context.Background()
	_, _ = store.CreateEquipment(ctx, domain.Equipment{ID: "eq-9", OwnerID: "user-a", Name: "Sensor"})
	_, _ = store.AttachProjectEquipment(ctx, domain.ProjectEquipment{ProjectID: "proj-9", EquipmentID: "eq-9", OwnerID: "user-a"})

	if err := store.DeleteEquipment(ctx, "user-a", "eq-9"); err != nil {
		t.Fatalf("DeleteEquipment() error = %v", err)
	}
	links, err := store.ListProjectEquipment(ctx, "user-a", "proj-9")
	if err != nil {
		t.Fatalf("ListProjectEquipment() error = %v", err)
	}
	if len(links) != 0 {
		t.Fatalf("ListProjectEquipment() after delete = %v, want no dangling links", links)
	}
}

func TestConfigConfigured(t *testing.T) {
	cases := []struct {
		config Config
		want   bool
	}{
		{Config{}, false},
		{Config{URI: "mongodb://localhost"}, false},
		{Config{Database: "reweird"}, false},
		{Config{URI: "mongodb://localhost", Database: "reweird"}, true},
	}
	for _, testCase := range cases {
		if got := testCase.config.Configured(); got != testCase.want {
			t.Errorf("Config(%+v).Configured() = %v, want %v", testCase.config, got, testCase.want)
		}
	}
}
