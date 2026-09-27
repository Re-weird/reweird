package domain

import (
	"context"
	"errors"
)

// ErrNotFound is returned by ProductRepository methods when the requested
// document does not exist, or exists but is owned by a different verified
// identity. Callers must treat both cases identically (404, never 403) so a
// private resource's existence is never leaked to another owner.
var ErrNotFound = errors.New("not found")

// ErrConflict is returned when a write would violate a uniqueness constraint
// this milestone requires (e.g. a duplicate project<->equipment attachment).
var ErrConflict = errors.New("conflict")

// User is a persistent, MongoDB-backed record of an authenticated identity.
// It is only ever created/looked up via ResolveUser, which is only ever
// called with a server-verified (auth_provider, auth_subject) pair -- never
// with anything a client claims about itself.
type User struct {
	ID           string `json:"id" bson:"_id"`
	AuthProvider string `json:"auth_provider" bson:"auth_provider"`
	AuthSubject  string `json:"auth_subject" bson:"auth_subject"`
	Email        string `json:"email,omitempty" bson:"email,omitempty"`
	DisplayName  string `json:"display_name,omitempty" bson:"display_name,omitempty"`
	AvatarURL    string `json:"avatar_url,omitempty" bson:"avatar_url,omitempty"`
	CreatedAtMS  int64  `json:"created_at_ms" bson:"created_at_ms"`
	UpdatedAtMS  int64  `json:"updated_at_ms" bson:"updated_at_ms"`
}

// UserProfileHints carries optional, non-authoritative display fields
// (email/name/avatar) observed alongside a verified token. They are only
// ever used to fill a field this store does not yet have a value for -- an
// already-stored value is never overwritten by a later hint, and no hint is
// ever treated as changing which user this is.
type UserProfileHints struct {
	Email       string
	DisplayName string
	AvatarURL   string
}

// Equipment is a physical component a user owns or is diagnosing. It is
// distinct from the trusted, read-only Component Catalog: CatalogID merely
// references a catalog entry (e.g. "hc-sr04") when the caller supplied and
// it validated against packages/component-catalog; it is never a copy of
// catalog data, and may be nil for custom/unknown equipment.
type Equipment struct {
	ID             string   `json:"id" bson:"_id"`
	OwnerID        string   `json:"owner_id" bson:"owner_id"`
	CatalogID      *string  `json:"catalog_id,omitempty" bson:"catalog_id,omitempty"`
	Name           string   `json:"name" bson:"name"`
	Category       string   `json:"category,omitempty" bson:"category,omitempty"`
	Manufacturer   string   `json:"manufacturer,omitempty" bson:"manufacturer,omitempty"`
	Model          string   `json:"model,omitempty" bson:"model,omitempty"`
	SerialNumber   string   `json:"serial_number,omitempty" bson:"serial_number,omitempty"`
	ControllerType string   `json:"controller_type,omitempty" bson:"controller_type,omitempty"`
	LogicVoltage   *float64 `json:"logic_voltage,omitempty" bson:"logic_voltage,omitempty"`
	Notes          string   `json:"notes,omitempty" bson:"notes,omitempty"`
	CreatedAtMS    int64    `json:"created_at_ms" bson:"created_at_ms"`
	UpdatedAtMS    int64    `json:"updated_at_ms" bson:"updated_at_ms"`
}

// EquipmentUpdate is a partial PATCH payload. A nil field is left
// unchanged. CatalogID uses an explicit "Set" flag rather than a
// pointer-to-pointer so "clear the catalog reference" can be expressed
// without the double-indirection that would otherwise require.
type EquipmentUpdate struct {
	Name           *string
	Category       *string
	Manufacturer   *string
	Model          *string
	SerialNumber   *string
	ControllerType *string
	LogicVoltage   *float64
	Notes          *string
	CatalogIDSet   bool
	CatalogID      *string
}

// ProjectEquipment links one piece of Equipment to one Project with a
// caller-supplied role (e.g. "distance sensing"). OwnerID is redundant with
// both Project.OwnerID and Equipment.OwnerID by construction -- it is
// stored (rather than re-derived) so isolation can be enforced with a
// single indexed query, and is only ever set from a verified identity that
// has already been checked to own both sides of the link.
type ProjectEquipment struct {
	ProjectID   string `json:"project_id" bson:"project_id"`
	EquipmentID string `json:"equipment_id" bson:"equipment_id"`
	OwnerID     string `json:"owner_id" bson:"owner_id"`
	Role        string `json:"role,omitempty" bson:"role,omitempty"`
	CreatedAtMS int64  `json:"created_at_ms" bson:"created_at_ms"`
}

// ProductRepository is the MongoDB-backed "product data" store: authenticated
// users, the equipment they own, and project<->equipment relationships. It
// is a separate abstraction from Repository (SQLite) by design -- this
// milestone is additive and does not migrate projects/diagnostics into
// MongoDB. Every method takes ownerID/authSubject explicitly rather than
// reading it from context, so ownership scoping is visible at every call
// site instead of hidden in a package global.
type ProductRepository interface {
	// ResolveUser looks up a user by verified (authProvider, authSubject),
	// creating one on first sight. It never creates a user for an empty
	// authSubject (anonymous/Demo Mode callers must never reach this
	// method at all -- see httpapi's ownerID/requireAuthenticated).
	ResolveUser(ctx context.Context, authProvider, authSubject string, hints UserProfileHints) (User, error)
	GetUser(ctx context.Context, id string) (*User, error)

	CreateEquipment(ctx context.Context, equipment Equipment) (Equipment, error)
	ListEquipment(ctx context.Context, ownerID string) ([]Equipment, error)
	// GetEquipment returns (nil, nil) -- not an error -- when the id exists
	// but is owned by a different owner, so callers can map both "missing"
	// and "not yours" to an identical 404 without a branch of their own.
	GetEquipment(ctx context.Context, ownerID, id string) (*Equipment, error)
	UpdateEquipment(ctx context.Context, ownerID, id string, update EquipmentUpdate) (*Equipment, error)
	// DeleteEquipment also removes any ProjectEquipment rows referencing
	// this equipment, so a deleted piece of equipment can never be left
	// dangling in a project's attachment list.
	DeleteEquipment(ctx context.Context, ownerID, id string) error

	// AttachProjectEquipment returns ErrConflict if this exact
	// (project_id, equipment_id) pair is already attached.
	AttachProjectEquipment(ctx context.Context, link ProjectEquipment) (ProjectEquipment, error)
	ListProjectEquipment(ctx context.Context, ownerID, projectID string) ([]ProjectEquipment, error)
	DetachProjectEquipment(ctx context.Context, ownerID, projectID, equipmentID string) error
}
